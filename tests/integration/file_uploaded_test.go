package integration

import (
	"duracloud/internal/db"
	"duracloud/internal/files"
	"testing"
	"time"
)

const checksumGenerationWaitTime = 120 * time.Second

func TestFileUploadedAndChecksumVerificationSuccess(t *testing.T) {
	
	clients, stackName, testBuckets, ctx := setupBucketTest(t, 1, "")
	testFileName := getTestFileName(stackName)
	testBucketName := getTestBucketName(stackName, testBuckets[0])

	t.Run("WithSingleFile", func(t *testing.T) {
		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationSuccessWaitTime)
		t.Logf("Using test bucket: %v", testBuckets[0])
		assertFileUploadSuccess(t, ctx, clients.S3, testBucketName, testFileName, "test file")
		t.Logf("Waiting for %s", checksumGenerationWaitTime.String())
	})

	t.Run("WithChecksumValidation", func(t *testing.T) {
		t.Logf("Waiting for %s", checksumGenerationWaitTime.String())
		waitForEventBridgeProcessing(checksumGenerationWaitTime)
		checksumTable := getChecksumTable(t)
		uploadToS3(ctx, clients.S3, testBucketName, testBucketName, testFileName)
		obj := files.NewS3Object(testBucketName, testFileName)
		checksumRecord, err := db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTable, obj)
		if err != nil {
			t.Logf("Error getting checksum record: %v", err)
		}
		t.Logf("checksum record for %v", checksumRecord)
	})
}
