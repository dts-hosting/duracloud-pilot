package integration

import (
	"testing"
)

func TestFileUploadedSuccess(t *testing.T) {
	t.Run("WithSingleFile", func(t *testing.T) {
		clients, stackName, testBuckets, ctx := setupBucketTest(t, 1, "")
		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationSuccessWaitTime)
		t.Logf("Using test bucket: %v", testBuckets[0])
		assertFileUploadSuccess(t, ctx, clients.S3, stackName, testBuckets[0])
	})
}

