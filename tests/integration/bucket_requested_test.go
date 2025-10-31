package integration

import (
	"context"
	"duracloud/internal/buckets"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
)

const (
	bucketCreationFailureWaitTime = 5 * time.Second
	bucketCreationSuccessWaitTime = 10 * time.Second
)

func TestBucketCreationSuccess(t *testing.T) {
	t.Parallel()

	t.Run("WithStandardBuckets", func(t *testing.T) {
		t.Parallel()

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

		clients, stackName, testBuckets, ctx := setupBucketTest(t, 2, "_not@allowed")

		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationFailureWaitTime)
		assertBucketsNotExist(t, ctx, clients.S3, stackName, testBuckets)
	})

	t.Run("WithTooManyNames", func(t *testing.T) {
		t.Parallel()

		clients, stackName, testBuckets, ctx := setupBucketTest(t, 6, "")

		uploadRequestAndWait(t, ctx, clients.S3, stackName, testBuckets, bucketCreationFailureWaitTime)
		assertBucketsNotExist(t, ctx, clients.S3, stackName, testBuckets)
	})
}

func setupBucketTest(t *testing.T, bucketCount int, suffix string) (*TestClients, string, []string, context.Context) {
	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	testBuckets := generateUniqueBucketNames("test", bucketCount, suffix)
	t.Logf("Using test buckets: %v", testBuckets)

	// Cleanup when tests finish
	t.Cleanup(func() {
		t.Logf("Cleaning up test buckets: %v", testBuckets)
		cleanupTestBuckets(ctx, clients.S3, stackName, testBuckets)
	})

	return clients, stackName, testBuckets, ctx
}

func verifyBucketConfig(t *testing.T, ctx context.Context, s3Client *s3.Client, bucketName, stackName string) {
	isPublicBucket := strings.HasSuffix(bucketName, buckets.PublicSuffix)

	t.Run("Versioning", func(t *testing.T) {
		versioning := getBucketVersioning(ctx, s3Client, bucketName)
		assert.Equal(t, "Enabled", versioning)
	})

	if isPublicBucket {
		t.Run("PublicAccessBlock", func(t *testing.T) {
			var publicAccessBlock *s3.GetPublicAccessBlockOutput
			success := WaitForCondition(t, "public access block configuration", func() bool {
				publicAccessBlock = getBucketPublicAccessBlock(ctx, s3Client, bucketName)
				return publicAccessBlock != nil && publicAccessBlock.PublicAccessBlockConfiguration != nil
			}, DefaultWaitConfig())

			if assert.True(t, success, "public access block should be configured") {
				assert.False(t, *publicAccessBlock.PublicAccessBlockConfiguration.BlockPublicPolicy)
			}
		})

		t.Run("PublicAccessPolicy", func(t *testing.T) {
			var policy *string
			success := WaitForCondition(t, "bucket policy configuration", func() bool {
				policy = getBucketPolicy(ctx, s3Client, bucketName)
				return policy != nil
			}, DefaultWaitConfig())

			if assert.True(t, success, "bucket policy should be configured") {
				var policyDoc map[string]any
				err := json.Unmarshal([]byte(*policy), &policyDoc)
				if assert.NoError(t, err) {
					statements := policyDoc["Statement"].([]any)
					if assert.NotEmpty(t, statements) {
						statement := statements[0].(map[string]any)
						assert.Equal(t, "AllowPublicRead", statement["Sid"])
					}
				}
			}
		})
	} else {
		t.Run("Lifecycle", func(t *testing.T) {
			var lifecycle *s3.GetBucketLifecycleConfigurationOutput
			success := WaitForCondition(t, "lifecycle configuration", func() bool {
				lifecycle = getBucketLifecycle(ctx, s3Client, bucketName)
				return lifecycle != nil && len(lifecycle.Rules) > 0 && len(lifecycle.Rules[0].Transitions) > 0
			}, DefaultWaitConfig())

			if assert.True(t, success, "lifecycle should be configured") {
				assert.Equal(t, types.TransitionStorageClassGlacierIr, lifecycle.Rules[0].Transitions[0].StorageClass)
			}
		})
	}

	t.Run("Notifications", func(t *testing.T) {
		var notifications *s3.GetBucketNotificationConfigurationOutput
		success := WaitForCondition(t, "notification configuration", func() bool {
			notifications = getBucketNotifications(ctx, s3Client, bucketName)
			return notifications != nil && notifications.EventBridgeConfiguration != nil
		}, DefaultWaitConfig())

		assert.True(t, success, "notifications should be configured")
	})

	t.Run("Inventory", func(t *testing.T) {
		var inventory []types.InventoryConfiguration
		success := WaitForCondition(t, "inventory configuration", func() bool {
			inventory = getBucketInventory(ctx, s3Client, bucketName)
			return len(inventory) > 0
		}, DefaultWaitConfig())

		if assert.True(t, success, "inventory should be configured") {
			assert.Equal(t, types.InventoryFrequencyDaily, inventory[0].Schedule.Frequency)
			assert.Equal(
				t,
				fmt.Sprintf("arn:aws:s3:::%s%s", stackName, buckets.ManagedSuffix),
				*inventory[0].Destination.S3BucketDestination.Bucket,
			)
			assert.Contains(t, *inventory[0].Destination.S3BucketDestination.Prefix, "inventory")
		}
	})

	t.Run("Logging", func(t *testing.T) {
		var logging *s3.GetBucketLoggingOutput
		success := WaitForCondition(t, "logging configuration", func() bool {
			logging = getBucketLogging(ctx, s3Client, bucketName)
			return logging != nil && logging.LoggingEnabled != nil
		}, DefaultWaitConfig())

		if assert.True(t, success, "logging should be configured") {
			assert.Equal(
				t,
				fmt.Sprintf("%s%s", stackName, buckets.ManagedSuffix),
				*logging.LoggingEnabled.TargetBucket,
			)
			assert.Contains(t, *logging.LoggingEnabled.TargetPrefix, "audit")
		}
	})

	t.Run("Replication", func(t *testing.T) {
		var replication *s3.GetBucketReplicationOutput
		success := WaitForCondition(t, "replication configuration", func() bool {
			replication = getBucketReplication(ctx, s3Client, bucketName)
			return replication != nil &&
				replication.ReplicationConfiguration != nil &&
				len(replication.ReplicationConfiguration.Rules) > 0
		}, DefaultWaitConfig())

		if assert.True(t, success, "replication should be configured") {
			assert.Equal(t, types.ReplicationRuleStatusEnabled, replication.ReplicationConfiguration.Rules[0].Status)
			assert.Equal(
				t,
				fmt.Sprintf("arn:aws:s3:::%s%s", bucketName, buckets.ReplicationSuffix),
				*replication.ReplicationConfiguration.Rules[0].Destination.Bucket,
			)
		}
	})

	t.Run("Tags", func(t *testing.T) {
		tags := getBucketTags(ctx, s3Client, bucketName)
		assert.NotEmpty(t, tags)

		var foundStack, foundType bool
		for _, tag := range tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == "BucketType" {
					foundType = true
				}
				if *tag.Key == "StackName" && *tag.Value == stackName {
					foundStack = true
				}
			}
		}
		assert.True(t, foundType, "Should have BucketType tag")
		assert.True(t, foundStack, "Should have StackName tag")
	})
}
