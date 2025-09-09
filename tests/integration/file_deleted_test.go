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

func TestFileDeletedWorkflow(t *testing.T) {
	t.Parallel()

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	// Table names
	checksumTable := fmt.Sprintf("%s-checksum-table", stackName)
	schedulerTable := fmt.Sprintf("%s-checksum-scheduler-table", stackName)

	// Create test object
	testBucket := fmt.Sprintf("%s-test", stackName)
	testKey := fmt.Sprintf("integration-test/file-deleted-%d.txt", time.Now().UnixNano())
	obj := files.NewS3Object(testBucket, testKey)

	// Create initial checksum records in both tables
	record := db.ChecksumRecord{
		BucketName:          obj.Bucket,
		ObjectKey:           obj.Key,
		Checksum:            "test-checksum",
		LastChecksumDate:    time.Now(),
		LastChecksumMessage: "Success",
		LastChecksumSuccess: true,
		NextChecksumDate:    time.Now().Add(time.Hour),
	}

	// Insert record into checksum table
	err := db.PutChecksumRecord(ctx, clients.DynamoDB, checksumTable, record)
	require.NoError(t, err, "Failed to create test checksum record")

	// Insert record into scheduler table
	err = db.ScheduleNextVerification(ctx, clients.DynamoDB, schedulerTable, record)
	require.NoError(t, err, "Failed to create test scheduler record")

	// Verify records exist before deletion
	_, err = db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTable, obj)
	assert.NoError(t, err, "Checksum record should exist before deletion")

	_, err = db.GetChecksumRecord(ctx, clients.DynamoDB, schedulerTable, obj)
	assert.NoError(t, err, "Scheduler record should exist before deletion")

	// Simulate file deletion by invoking the file-deleted lambda
	functionName := fmt.Sprintf("%s-file-deleted", stackName)

	// Use the actual event structure from the file-deleted event template (re: event.json)
	eventPayload := fmt.Sprintf(`{
		"Records": [
			{
				"messageId": "test-message-id",
				"body": "{\"version\":\"0\",\"id\":\"test-event-id\",\"detail-type\":\"Object Deleted\",\"source\":\"aws.s3\",\"account\":\"123456789012\",\"time\":\"%s\",\"region\":\"us-east-1\",\"resources\":[\"arn:aws:s3:::%s\"],\"detail\":{\"version\":\"0\",\"bucket\":{\"name\":\"%s\"},\"object\":{\"key\":\"%s\",\"etag\":\"test-etag\",\"sequencer\":\"test-sequencer\"},\"request-id\":\"test-request-id\",\"requester\":\"123456789012\",\"source-ip-address\":\"192.0.2.1\",\"reason\":\"DeleteObject\"}}"
			}
		]
	}`, time.Now().Format(time.RFC3339), obj.Bucket, obj.Bucket, obj.Key)

	_, err = lambdaFunctionInvoke(ctx, clients.Lambda, functionName, []byte(eventPayload))
	require.NoError(t, err, "Failed to invoke file-deleted lambda")

	// Wait for records to be deleted
	waitConfig := DefaultWaitConfig()
	waitConfig.MaxTimeout = 30 * time.Second

	// Verify checksum record is deleted
	success := WaitForCondition(t, "checksum record deletion", func() bool {
		_, err := db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTable, obj)
		return err != nil // Record should not exist (error expected)
	}, waitConfig)
	assert.True(t, success, "Checksum record should be deleted")

	// Verify scheduler record is deleted
	success = WaitForCondition(t, "scheduler record deletion", func() bool {
		_, err := db.GetChecksumRecord(ctx, clients.DynamoDB, schedulerTable, obj)
		return err != nil // Record should not exist (error expected)
	}, waitConfig)
	assert.True(t, success, "Scheduler record should be deleted")

	t.Logf("Successfully verified file-deleted workflow for %s", obj.URI())
}
