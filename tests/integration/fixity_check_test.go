package integration

import (
	"context"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const checksumGenerationWaitTime = 120 * time.Second

func TestFileUploadedAndChecksumVerificationSuccess(t *testing.T) {

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	testFileName := "checksum-it.txt"
	requestFileName := "checksum-it-bucket-request.txt"
	testBucketName := "checksum-it"
	testChecksum := "e9309ee2f26c56636574c93cb74c951c"
	fullTestBucketName := fmt.Sprintf("%s-%s", stackName, testBucketName)
	requestBucketName := fmt.Sprintf("%s-bucket-requested", stackName)
	//testBuckets := []string{testBucketName}

	t.Run("WithUploadSingleFile", func(t *testing.T) {
		err := uploadToS3(ctx, clients.S3, requestBucketName, requestFileName, testBucketName)
		require.NoError(t, err, "Should upload request file")
		t.Logf("Waiting %v for Lambda processing...", bucketCreationSuccessWaitTime)
		waitForLambdaProcessing(bucketCreationSuccessWaitTime)
		err = uploadToS3(ctx, clients.S3, fullTestBucketName, testFileName, "test-file")
		require.NoError(t, err, "Should upload test file")
	})

	t.Run("WithChecksumValidation", func(t *testing.T) {
		t.Logf("Waiting for %s", checksumGenerationWaitTime.String())
		waitForEventBridgeProcessing(checksumGenerationWaitTime)
		uploadToS3(ctx, clients.S3, fullTestBucketName, testFileName, "test-file")
		obj := files.NewS3Object(fullTestBucketName, testFileName)
		checksumTableName := fmt.Sprintf("%s-checksum-table", stackName)
		checksumRecord, err := db.GetChecksumRecord(ctx, clients.DynamoDB, checksumTableName, obj)
		require.NoError(t, err, "Should retrieve checksum for test file")
		assert.Equal(t, testChecksum, checksumRecord.Checksum, "Checksum should match")
		t.Logf("checksum record for %v", checksumRecord)
	})
}
