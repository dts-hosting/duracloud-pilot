package integration

import (
	"fmt"
	"testing"
	"time"
)

const (
	bucketCreationFailureWaitTime = 5 * time.Second
	bucketCreationSuccessWaitTime = 10 * time.Second
)

func TestBucketCreationSuccess(t *testing.T) {
	t.Parallel()

	t.Run("WithStandardBuckets", func(t *testing.T) {
		t.Parallel()
		time.Sleep(0 * time.Second)

		clients, stackName, testBuckets, ctx := setupBucketTest(t, 2, "")

		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationSuccessWaitTime)
		assertBucketsExist(t, ctx, clients.S3, stackName, testBuckets)

		for _, testBucket := range testBuckets {
			primaryBucket := fmt.Sprintf("%s-%s", stackName, testBucket)
			t.Run("Config_"+testBucket, func(t *testing.T) {
				verifyBucketConfig(t, ctx, clients.S3, primaryBucket, stackName)
			})
		}
	})

	t.Run("WithPublicBucket", func(t *testing.T) {
		t.Parallel()
		time.Sleep(2 * time.Second)

		clients, stackName, testBuckets, ctx := setupBucketTest(t, 1, "-public")

		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationSuccessWaitTime)
		assertBucketsExist(t, ctx, clients.S3, stackName, testBuckets)

		for _, testBucket := range testBuckets {
			primaryBucket := fmt.Sprintf("%s-%s", stackName, testBucket)
			t.Run("Config_"+testBucket, func(t *testing.T) {
				verifyBucketConfig(t, ctx, clients.S3, primaryBucket, stackName)
			})
		}
	})
}

func TestBucketCreationFailure(t *testing.T) {
	t.Parallel()

	t.Run("WithInvalidNames", func(t *testing.T) {
		t.Parallel()
		time.Sleep(4 * time.Second)

		clients, stackName, testBuckets, ctx := setupBucketTest(t, 2, "_not@allowed")

		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationFailureWaitTime)
		assertBucketsNotExist(t, ctx, clients.S3, stackName, testBuckets)
	})

	t.Run("WithTooManyNames", func(t *testing.T) {
		t.Parallel()
		time.Sleep(6 * time.Second)

		clients, stackName, testBuckets, ctx := setupBucketTest(t, 6, "")

		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationFailureWaitTime)
		assertBucketsNotExist(t, ctx, clients.S3, stackName, testBuckets)
	})
}
