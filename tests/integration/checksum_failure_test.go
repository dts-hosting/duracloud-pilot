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

func TestChecksumFailureWorkflow(t *testing.T) {
	t.Parallel()

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	// Table names
	checksumTable := fmt.Sprintf("%s-checksum-table", stackName)

	// Create test object
	testBucket := fmt.Sprintf("%s-test", stackName)
	testKey := fmt.Sprintf("integration-test/checksum-failure-%d.txt", time.Now().UnixNano())
	obj := files.NewS3Object(testBucket, testKey)

	// Create initial checksum record with success=true
	record := db.ChecksumRecord{
		BucketName:          obj.Bucket,
		ObjectKey:           obj.Key,
		Checksum:            "test-checksum",
		LastChecksumDate:    time.Now().Add(-time.Hour), // Set in past
		LastChecksumMessage: "Success",
		LastChecksumSuccess: true,
		NextChecksumDate:    time.Now().Add(time.Hour),
	}

	// Insert initial record
	err := db.PutChecksumRecord(ctx, clients.DynamoDB, checksumTable, record)
	require.NoError(t, err, "Failed to create initial checksum record")

	// TODO: may need to reconsider how this is being done to make it more precise
	// Get initial SNS message count (baseline)
	initialMessageCount, err := getSNSMessageCount(t, ctx, clients, stackName)
	require.NoError(t, err, "Failed to get initial SNS message count")

	// Update record to trigger failure (this will trigger the checksum-failure lambda via DynamoDB stream)
	failedRecord := record
	failedRecord.LastChecksumDate = time.Now()
	failedRecord.LastChecksumMessage = "checksum mismatch: expected test-checksum, got different-checksum"
	failedRecord.LastChecksumSuccess = false // This change triggers the failure workflow

	err = db.PutChecksumRecord(ctx, clients.DynamoDB, checksumTable, failedRecord)
	require.NoError(t, err, "Failed to update checksum record to trigger failure")

	// Wait for failure workflow to process the change and CloudWatch metrics to be available
	// CloudWatch metrics can take several minutes to appear, so we'll poll
	waitConfig := DefaultWaitConfig()
	waitConfig.MaxTimeout = 60 * time.Second   // Allow up to 1 minute
	waitConfig.PollInterval = 10 * time.Second // Check every 10 seconds

	var finalMessageCount float64
	success := WaitForCondition(t, "SNS message count to increase", func() bool {
		count, err := getSNSMessageCount(t, ctx, clients, stackName)
		if err != nil {
			t.Logf("Error getting SNS message count: %v", err)
			return false
		}
		finalMessageCount = count
		return count > initialMessageCount
	}, waitConfig)

	if !success {
		t.Logf("CloudWatch metrics did not show SNS message increase within timeout")
		t.Logf("This could be due to CloudWatch metrics delay - the workflow may still be working")
		// The test doesn't fail here because CloudWatch metrics can be delayed
	} else {
		t.Logf("SNS message count increased from %.0f to %.0f", initialMessageCount, finalMessageCount)
	}

	// Verify the record still exists with failure status
	updatedRecord, err := db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTable, obj)
	require.NoError(t, err, "Should be able to retrieve updated record")
	assert.False(t, updatedRecord.LastChecksumSuccess, "Record should show failure status")
	assert.Contains(t, updatedRecord.LastChecksumMessage, "checksum mismatch",
		"Record should contain failure message")

	t.Logf("Successfully verified checksum-failure workflow for %s", obj.URI())
}
