package integration

import (
	"context"
	"crypto/md5"
	"duracloud/internal/accounts"
	"duracloud/internal/buckets"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	// Default wait time for checksum generation processing
	defaultChecksumGenerationWaitTime = 120 * time.Second
	// Default wait time for verification processing
	defaultVerificationWaitTime = 180 * time.Second
)

// FixityTestHelper provides helper functions for fixity check testing
type FixityTestHelper struct {
	Clients            *TestClients
	StackName          string
	ChecksumTableName  string
	SchedulerTableName string
	Context            context.Context
}

// NewFixityTestHelper creates a new helper for fixity check testing
func NewFixityTestHelper(t *testing.T) *FixityTestHelper {
	clients, stackName := setupTestClients(t)

	// Create AWS context with account information needed by buckets package
	ctx := context.Background()
	awsConfig, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err, "Should load AWS config")

	accountID, err := accounts.GetAccountID(ctx, awsConfig)
	require.NoError(t, err, "Should get AWS account ID")

	awsContext := accounts.AWSContext{
		AccountID: accountID,
		Region:    awsConfig.Region,
		StackName: stackName,
	}

	// Create context with AWS account information
	ctxWithAWS := context.WithValue(ctx, accounts.AWSContextKey, awsContext)

	return &FixityTestHelper{
		Clients:            clients,
		StackName:          stackName,
		ChecksumTableName:  fmt.Sprintf("%s-checksum-table", stackName),
		SchedulerTableName: fmt.Sprintf("%s-checksum-scheduler-table", stackName),
		Context:            ctxWithAWS,
	}
}

// UploadTestFile uploads a test file to S3 and returns the expected checksum
func (h *FixityTestHelper) UploadTestFile(t *testing.T, bucketName, fileName, content string) string {
	err := uploadToS3(h.Context, h.Clients.S3, bucketName, fileName, content)
	require.NoError(t, err, "Should upload test file %s to bucket %s", fileName, bucketName)

	expectedChecksum := fmt.Sprintf("%x", md5.Sum([]byte(content)))
	t.Logf("Uploaded file %s/%s with expected checksum: %s", bucketName, fileName, expectedChecksum)
	return expectedChecksum
}

// WaitForThenValidateChecksum waits for the initial checksum calculation and validates the record
func (h *FixityTestHelper) WaitForThenValidateChecksum(t *testing.T, bucketName, fileName, expectedChecksum string) db.ChecksumRecord {
	obj := files.NewS3Object(bucketName, fileName)

	// Configure wait parameters for initial checksum processing
	cfg := DefaultWaitConfig()
	cfg.MaxTimeout = defaultChecksumGenerationWaitTime
	cfg.PollInterval = 3 * time.Second // Start with 3-second intervals
	cfg.InitialDelay = 2 * time.Second // Give EventBridge a moment to process

	// Validator function to check if the record has the expected checksum and is successful
	validator := func(record db.ChecksumRecord) bool {
		return record.Checksum == expectedChecksum &&
			record.LastChecksumSuccess &&
			record.LastChecksumMessage == "ok" &&
			!record.LastChecksumDate.IsZero() &&
			!record.NextChecksumDate.IsZero()
	}

	record, success := WaitForDynamoDBRecord(t, h.Clients, h.ChecksumTableName, &obj, validator, cfg)
	require.True(t, success, "Should retrieve and validate initial checksum record within timeout")

	// Use comprehensive validation function
	ValidateChecksumRecord(t, record, bucketName, fileName, expectedChecksum, true)

	t.Logf("Initial checksum record validated: %+v", record)
	return record
}

// TriggerVerification manually triggers a verification by creating an expired TTL record
func (h *FixityTestHelper) TriggerVerification(t *testing.T, record db.ChecksumRecord) {
	expiredRecord := record
	expiredRecord.NextChecksumDate = time.Now().Add(-1 * time.Hour) // Expired 1 hour ago

	err := db.ScheduleNextVerification(h.Context, h.Clients.DynamoDB, h.SchedulerTableName, expiredRecord)
	require.NoError(t, err, "Should schedule verification")

	t.Logf("Triggered verification for %s/%s", record.BucketName, record.ObjectKey)
}

// WaitForVerification waits for verification processing and returns the updated record
func (h *FixityTestHelper) WaitForVerification(t *testing.T, bucketName, fileName string) db.ChecksumRecord {
	obj := files.NewS3Object(bucketName, fileName)

	// Get the current record to compare timestamps later
	initialRecord, err := db.GetChecksumRecord(h.Context, h.Clients.DynamoDB, h.ChecksumTableName, obj)
	require.NoError(t, err, "Should retrieve initial record for comparison")

	// Configure wait parameters for verification processing (longer timeout for TTL processing)
	cfg := DefaultWaitConfig()
	cfg.MaxTimeout = defaultVerificationWaitTime
	cfg.PollInterval = 5 * time.Second // Start with 5-second intervals for verification
	cfg.InitialDelay = 3 * time.Second // Give TTL processing more time to start

	// Validator function to check if verification has been processed
	// We check if LastChecksumDate has been updated (indicating verification occurred)
	validator := func(record db.ChecksumRecord) bool {
		return record.LastChecksumDate.After(initialRecord.LastChecksumDate)
	}

	record, success := WaitForDynamoDBRecord(t, h.Clients, h.ChecksumTableName, &obj, validator, cfg)
	require.True(t, success, "Should retrieve updated checksum record after verification within timeout")

	t.Logf("Verification completed for %s/%s", bucketName, fileName)
	return record
}

// ValidateSuccessfulVerification validates that a verification was successful
func (h *FixityTestHelper) ValidateSuccessfulVerification(t *testing.T, beforeRecord, afterRecord db.ChecksumRecord, expectedChecksum string) {
	ValidateChecksumRecord(t, afterRecord, afterRecord.BucketName, afterRecord.ObjectKey, expectedChecksum, true)

	// Additional validation specific to verification (comparing before/after)
	require.True(t, afterRecord.LastChecksumDate.After(beforeRecord.LastChecksumDate),
		"Last checksum date should be updated")

	// Debug logging for NextChecksumDate comparison
	t.Logf("NextChecksumDate comparison for %s/%s:", afterRecord.BucketName, afterRecord.ObjectKey)
	t.Logf("  Before: %v", beforeRecord.NextChecksumDate)
	t.Logf("  After:  %v", afterRecord.NextChecksumDate)
	t.Logf("  After > Before: %v", afterRecord.NextChecksumDate.After(beforeRecord.NextChecksumDate))

	// The NextChecksumDate should be rescheduled to a future date
	// Note: We need to account for the fact that TriggerVerification sets an expired date
	// so we should check that the new NextChecksumDate is in the future from now
	now := time.Now()
	require.True(t, afterRecord.NextChecksumDate.After(now),
		"Next checksum date should be rescheduled to future (after %v, got %v)", now, afterRecord.NextChecksumDate)

	t.Logf("Successfully validated verification for %s/%s", afterRecord.BucketName, afterRecord.ObjectKey)
}

// ValidateFailedVerification validates that a verification failed due to checksum mismatch
func (h *FixityTestHelper) ValidateFailedVerification(t *testing.T, beforeRecord, afterRecord db.ChecksumRecord) {
	require.False(t, afterRecord.LastChecksumSuccess, "Verification should fail due to mismatch")
	require.Contains(t, afterRecord.LastChecksumMessage, "Checksum mismatch",
		"Message should indicate checksum mismatch")
	require.True(t, afterRecord.LastChecksumDate.After(beforeRecord.LastChecksumDate),
		"Last checksum date should be updated even on failure")

	t.Logf("Successfully validated failed verification: %s", afterRecord.LastChecksumMessage)
}

// CreateTestBucket creates a fully configured S3 bucket directly for fixity testing
// This bypasses the bucket-requested workflow and creates buckets with essential configurations
func (h *FixityTestHelper) CreateTestBucket(t *testing.T, suffix string) string {
	// Add random UUID to make bucket name unique and avoid conflicts during cleanup/recreation
	uid := uuid.New().String()[:12]
	bucketName := fmt.Sprintf("%s-%s-%s", h.StackName, suffix, uid)

	// Create bucket directly with essential configurations for fixity testing
	err := h.createBucketDirectly(t, bucketName)
	require.NoError(t, err, "Should create bucket directly for fixity testing")

	t.Logf("Created test bucket directly: %s", bucketName)
	return bucketName
}

// createBucketDirectly creates a bucket with essential configurations needed for fixity testing
// This replicates the key parts of the bucket-requested workflow without going through Lambda
func (h *FixityTestHelper) createBucketDirectly(t *testing.T, bucketName string) error {
	ctx := h.Context
	s3Client := h.Clients.S3

	// Import the buckets package functions we need
	// We'll call the same functions the Lambda uses, but directly

	// Step 1: Create the main bucket
	if err := h.createBasicBucket(ctx, s3Client, bucketName); err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	// Step 2: Enable versioning (required for fixity checking)
	if err := h.enableBucketVersioning(ctx, s3Client, bucketName); err != nil {
		return fmt.Errorf("failed to enable versioning for %s: %w", bucketName, err)
	}

	// Step 3: Enable EventBridge notifications (essential for file upload events)
	if err := h.enableEventBridgeNotifications(ctx, s3Client, bucketName); err != nil {
		return fmt.Errorf("failed to enable EventBridge for %s: %w", bucketName, err)
	}

	// Step 4: Add basic tags for identification
	if err := h.addBasicBucketTags(ctx, s3Client, bucketName); err != nil {
		return fmt.Errorf("failed to add tags to %s: %w", bucketName, err)
	}

	t.Logf("Successfully configured bucket %s for fixity testing", bucketName)
	return nil
}

// Helper methods for direct bucket creation

// createBasicBucket creates a basic S3 bucket
func (h *FixityTestHelper) createBasicBucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	return buckets.CreateNewBucket(ctx, s3Client, bucketName)
}

// enableBucketVersioning enables versioning on the bucket (required for fixity checking)
func (h *FixityTestHelper) enableBucketVersioning(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	return buckets.EnableVersioning(ctx, s3Client, bucketName)
}

// enableEventBridgeNotifications enables EventBridge notifications (essential for file upload events)
func (h *FixityTestHelper) enableEventBridgeNotifications(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	return buckets.EnableEventBridge(ctx, s3Client, bucketName)
}

// addBasicBucketTags adds basic tags to the bucket for identification
func (h *FixityTestHelper) addBasicBucketTags(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	return buckets.AddBucketTags(ctx, s3Client, bucketName, h.StackName, "Test")
}

// SimulateCorruption simulates file corruption by modifying the stored checksum in the database
// This creates a scenario where the file content doesn't match the stored checksum,
// which is what would happen in real corruption scenarios
func (h *FixityTestHelper) SimulateCorruption(t *testing.T, bucketName, fileName string) db.ChecksumRecord {
	obj := files.NewS3Object(bucketName, fileName)

	// Get the current record
	record, err := db.GetChecksumRecord(h.Context, h.Clients.DynamoDB, h.ChecksumTableName, obj)
	require.NoError(t, err, "Should retrieve existing checksum record")

	// Modify the checksum to simulate corruption
	// We'll change the last character to create a different but valid-looking checksum
	originalChecksum := record.Checksum
	corruptedChecksum := originalChecksum[:len(originalChecksum)-1] + "0"
	if originalChecksum[len(originalChecksum)-1:] == "0" {
		corruptedChecksum = originalChecksum[:len(originalChecksum)-1] + "1"
	}

	record.Checksum = corruptedChecksum

	// Update the record with the corrupted checksum
	err = db.PutChecksumRecord(h.Context, h.Clients.DynamoDB, h.ChecksumTableName, record)
	require.NoError(t, err, "Should update checksum record with corrupted checksum")

	t.Logf("Simulated corruption for %s/%s: original=%s, corrupted=%s",
		bucketName, fileName, originalChecksum, corruptedChecksum)

	return record
}

// GenerateTestContent generates test content of specified size
func GenerateTestContent(size int, prefix string) string {
	if size <= 0 {
		return ""
	}

	content := ""
	baseContent := fmt.Sprintf("%s test data. ", prefix)

	for len(content) < size {
		content += baseContent
	}

	// Trim to exact size if needed
	if len(content) > size {
		content = content[:size]
	}

	return content
}

// ValidateChecksumRecord validates all fields of a checksum record
func ValidateChecksumRecord(t *testing.T, record db.ChecksumRecord, expectedBucket, expectedKey, expectedChecksum string, expectedSuccess bool) {
	require.Equal(t, expectedBucket, record.BucketName, "Bucket name should match")
	require.Equal(t, expectedKey, record.ObjectKey, "Object key should match")
	require.Equal(t, expectedChecksum, record.Checksum, "Checksum should match expected value")
	require.Equal(t, expectedSuccess, record.LastChecksumSuccess, "Success status should match expected")
	require.False(t, record.LastChecksumDate.IsZero(), "Last checksum date should be set")
	require.False(t, record.NextChecksumDate.IsZero(), "Next checksum date should be set")

	if expectedSuccess {
		require.Equal(t, "ok", record.LastChecksumMessage, "Success message should be 'ok'")
	} else {
		require.NotEqual(t, "ok", record.LastChecksumMessage, "Failure message should not be 'ok'")
	}
}
