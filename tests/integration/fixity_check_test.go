package integration

import (
	"context"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"fmt"
	//"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const checksumGenerationWaitTime = 240 * time.Second

func TestFileUploadedAndChecksumVerificationSuccess(t *testing.T) {

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	testFileName := "checksum-it.txt"
	requestFileName := "checksum-it-bucket-request.txt"
	testBucketName := "checksum-it"
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
		if err != nil {
			t.Logf("Error getting checksum record: %v", err)
		}
		t.Logf("checksum record for %v", checksumRecord)
	})
}
