package integration

import (
	"context"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitialChecksumStorage tests the file-uploaded Lambda workflow
func TestInitialChecksumStorage(t *testing.T) {
	t.Parallel()

	helper := NewFixityTestHelper(t)
	testBucketName := helper.CreateTestBucket(t, "test-initial")

	t.Run("SuccessfulChecksumCalculationAndStorage", func(t *testing.T) {
		t.Parallel()

		testFileName := "test-checksum-storage.txt"
		testContent := "Hello, DuraCloud fixity test!"

		// Upload file and get expected checksum
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)

		// Wait for initial checksum processing and validate
		checksumRecord := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)

		t.Logf("Successfully verified checksum record: %+v", checksumRecord)
	})

	t.Run("LargeFileChecksumCalculation", func(t *testing.T) {
		t.Parallel()

		testFileName := "large-test-file.txt"
		// Create ~1MB test content using helper
		testContent := GenerateTestContent(1000000, "This is test data for large file checksum calculation.")

		// Upload file and get expected checksum
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)

		// Wait for initial checksum processing and validate
		checksumRecord := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)

		t.Logf("Large file checksum validation completed: %+v", checksumRecord)
	})

	t.Run("ZeroByteFileHandling", func(t *testing.T) {
		t.Parallel()

		testFileName := "empty-file.txt"
		testContent := ""

		// Upload file and get expected checksum
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)

		// Wait for initial checksum processing and validate
		checksumRecord := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)

		t.Logf("Empty file checksum validation completed: %+v", checksumRecord)
	})
}

// TestPeriodicFixityVerification tests the checksum-verification Lambda workflow
func TestPeriodicFixityVerification(t *testing.T) {
	t.Parallel()

	helper := NewFixityTestHelper(t)
	testBucketName := helper.CreateTestBucket(t, "test-verification")

	t.Run("SuccessfulChecksumVerification", func(t *testing.T) {
		t.Parallel()

		testFileName := "test-verification.txt"
		testContent := "Content for verification test"

		// Upload file and wait for initial processing
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)
		initialRecord := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)
		lastChecksumDate := initialRecord.LastChecksumDate

		helper.InvokeVerificationFunction(t, initialRecord)

		// Wait for verification processing and get updated record
		updatedRecord := helper.WaitForVerification(t, testBucketName, testFileName, lastChecksumDate)

		// Validate successful verification
		helper.ValidateSuccessfulVerification(t, initialRecord, updatedRecord, expectedChecksum)

		t.Logf("Successfully verified periodic checksum verification")
	})

	t.Run("ChecksumMismatchDetection", func(t *testing.T) {
		t.Parallel()

		testFileName := "test-mismatch.txt"
		testContent := "Content for corruption test"

		// Upload file and wait for initial processing
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)
		initialRecord := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)
		lastChecksumDate := initialRecord.LastChecksumDate

		// Simulate corruption by modifying the stored checksum in the database
		corruptedRecord := helper.SimulateCorruption(t, testBucketName, testFileName)

		helper.InvokeVerificationFunction(t, corruptedRecord)

		// Wait for verification processing and get updated record
		updatedRecord := helper.WaitForVerification(t, testBucketName, testFileName, lastChecksumDate)

		// Validate failed verification
		helper.ValidateFailedVerification(t, initialRecord, updatedRecord)

		t.Logf("Successfully detected checksum mismatch: %s", updatedRecord.LastChecksumMessage)
	})
}

// TestDatabaseOperations tests CRUD operations on checksum and scheduler tables
func TestDatabaseOperations(t *testing.T) {
	t.Parallel()

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	checksumTableName := fmt.Sprintf("%s-checksum-table", stackName)
	schedulerTableName := fmt.Sprintf("%s-checksum-scheduler-table", stackName)

	t.Run("ChecksumRecordCRUD", func(t *testing.T) {
		obj := files.NewS3Object("test-bucket", "test-object.txt")

		// Test record creation
		record := db.ChecksumRecord{
			BucketName:          obj.Bucket,
			ObjectKey:           obj.Key,
			Checksum:            "d41d8cd98f00b204e9800998ecf8427e",
			LastChecksumDate:    time.Now(),
			LastChecksumMessage: "test message",
			LastChecksumSuccess: true,
			NextChecksumDate:    time.Now().Add(24 * time.Hour),
		}

		err := db.PutChecksumRecord(ctx, clients.DynamoDB, checksumTableName, record)
		require.NoError(t, err, "Should create checksum record")

		// Test record retrieval
		retrievedRecord, err := db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTableName, obj)
		require.NoError(t, err, "Should retrieve checksum record")

		assert.Equal(t, record.BucketName, retrievedRecord.BucketName)
		assert.Equal(t, record.ObjectKey, retrievedRecord.ObjectKey)
		assert.Equal(t, record.Checksum, retrievedRecord.Checksum)
		assert.Equal(t, record.LastChecksumMessage, retrievedRecord.LastChecksumMessage)
		assert.Equal(t, record.LastChecksumSuccess, retrievedRecord.LastChecksumSuccess)

		// Test record update
		updatedRecord := retrievedRecord
		updatedRecord.LastChecksumMessage = "updated message"
		updatedRecord.LastChecksumSuccess = false

		err = db.PutChecksumRecord(ctx, clients.DynamoDB, checksumTableName, updatedRecord)
		require.NoError(t, err, "Should update checksum record")

		finalRecord, err := db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTableName, obj)
		require.NoError(t, err, "Should retrieve updated record")

		assert.Equal(t, "updated message", finalRecord.LastChecksumMessage)
		assert.False(t, finalRecord.LastChecksumSuccess)

		// Test record deletion
		err = db.DeleteChecksumRecord(ctx, clients.DynamoDB, checksumTableName, obj)
		require.NoError(t, err, "Should delete checksum record")

		_, err = db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTableName, obj)
		assert.Error(t, err, "Should not find deleted record")
	})

	t.Run("SchedulerOperations", func(t *testing.T) {
		record := db.ChecksumRecord{
			BucketName:       "test-scheduler-bucket",
			ObjectKey:        "test-scheduler-object.txt",
			NextChecksumDate: time.Now().Add(1 * time.Hour),
		}

		err := db.ScheduleNextVerification(ctx, clients.DynamoDB, schedulerTableName, record)
		require.NoError(t, err, "Should schedule verification")

		t.Logf("Successfully scheduled verification for %s/%s", record.BucketName, record.ObjectKey)
	})
}

// TestBucketCreationWithFixityWorkflow tests the integration between bucket creation and fixity checking
// This test validates the complete workflow from direct bucket creation through fixity check processing
func TestBucketCreationWithFixityWorkflow(t *testing.T) {
	t.Parallel()

	// Create helper for fixity testing (uses direct bucket creation)
	helper := NewFixityTestHelper(t)

	t.Run("BucketCreationAndFileUpload", func(t *testing.T) {
		// Create test bucket directly for fixity testing
		testBucketName := helper.CreateTestBucket(t, "checksum-it")
		testFileName := "checksum-it.txt"
		testContent := "test-file"

		// Upload test file and validate fixity checking works
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)
		checksumRecord := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)

		t.Logf("Successfully validated fixity workflow on directly created bucket: %+v", checksumRecord)
	})
}

// TestEndToEndWorkflow tests complete lifecycle from upload through multiple verification cycles
func TestEndToEndWorkflow(t *testing.T) {
	t.Parallel()

	helper := NewFixityTestHelper(t)
	testBucketName := helper.CreateTestBucket(t, "test-e2e")

	t.Run("CompleteLifecycle", func(t *testing.T) {
		testFileName := "lifecycle-test.txt"
		testContent := "End-to-end lifecycle test content"

		// Phase 1: Initial upload and checksum storage
		t.Log("Phase 1: Initial upload and checksum storage")
		expectedChecksum := helper.UploadTestFile(t, testBucketName, testFileName, testContent)
		phase1Record := helper.WaitForThenValidateChecksum(t, testBucketName, testFileName, expectedChecksum)
		lastChecksumDate := phase1Record.LastChecksumDate

		// Phase 2: First verification cycle
		t.Log("Phase 2: First verification cycle")
		helper.InvokeVerificationFunction(t, phase1Record)
		phase2Record := helper.WaitForVerification(t, testBucketName, testFileName, lastChecksumDate)
		lastChecksumDate = phase2Record.LastChecksumDate
		helper.ValidateSuccessfulVerification(t, phase1Record, phase2Record, expectedChecksum)

		// Phase 3: Second verification cycle (simulating long-term operation)
		t.Log("Phase 3: Second verification cycle")
		helper.InvokeVerificationFunction(t, phase2Record)
		phase3Record := helper.WaitForVerification(t, testBucketName, testFileName, lastChecksumDate)
		helper.ValidateSuccessfulVerification(t, phase2Record, phase3Record, expectedChecksum)

		t.Logf("Successfully completed end-to-end lifecycle test with %d verification cycles", 2)
	})
}
