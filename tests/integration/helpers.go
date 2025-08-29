package integration

import (
	"context"
	"duracloud/internal/buckets"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type TestClients struct {
	S3         *s3.Client
	Lambda     *lambda.Client
	IAM        *iam.Client
	DynamoDB   *dynamodb.Client
	CloudWatch *cloudwatch.Client
	SNS        *sns.Client
}

func setupTestClients(t *testing.T) (*TestClients, string) {
	ctx := context.Background()

	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatalf("Unable to load AWS config: %v", err)
	}

	stackName := os.Getenv("STACK_NAME")
	if stackName == "" {
		t.Fatal("STACK_NAME environment variable must be set")
	}

	return &TestClients{
		S3:         s3.NewFromConfig(awsConfig),
		Lambda:     lambda.NewFromConfig(awsConfig),
		IAM:        iam.NewFromConfig(awsConfig),
		DynamoDB:   dynamodb.NewFromConfig(awsConfig),
		CloudWatch: cloudwatch.NewFromConfig(awsConfig),
		SNS:        sns.NewFromConfig(awsConfig),
	}, stackName
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

// Check if buckets exist (success case)
func assertBucketsExist(t *testing.T, ctx context.Context, s3Client *s3.Client, stackName string, testBuckets []string) {
	for _, testBucket := range testBuckets {
		primaryBucket := fmt.Sprintf("%s-%s", stackName, testBucket)
		assert.True(t, bucketExists(ctx, s3Client, primaryBucket),
			"Primary bucket %s should exist", primaryBucket)

		replicationBucket := fmt.Sprintf("%s-%s%s", stackName, testBucket, buckets.ReplicationSuffix)
		assert.True(t, bucketExists(ctx, s3Client, replicationBucket),
			"Replication bucket %s should exist", replicationBucket)
	}
}

// Check if buckets don't exist (failure case)
func assertBucketsNotExist(t *testing.T, ctx context.Context, s3Client *s3.Client, stackName string, testBuckets []string) {
	for _, testBucket := range testBuckets {
		primaryBucket := fmt.Sprintf("%s-%s", stackName, testBucket)
		assert.False(t, bucketExists(ctx, s3Client, primaryBucket),
			"Primary bucket %s should NOT exist", primaryBucket)

		replicationBucket := fmt.Sprintf("%s-%s%s", stackName, testBucket, buckets.ReplicationSuffix)
		assert.False(t, bucketExists(ctx, s3Client, replicationBucket),
			"Replication bucket %s should NOT exist", replicationBucket)
	}
}

func bucketExists(ctx context.Context, s3Client *s3.Client, bucketName string) bool {
	_, err := s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	return err == nil
}

func cleanupTestBuckets(ctx context.Context, s3Client *s3.Client, stackName string, testBuckets []string) {
	for _, bucketSuffix := range testBuckets {
		bucketName := fmt.Sprintf("%s-%s", stackName, bucketSuffix)

		deleteBucketCompletely(ctx, s3Client, bucketName)

		replicationBucketName := fmt.Sprintf("%s%s", bucketName, buckets.ReplicationSuffix)
		deleteBucketCompletely(ctx, s3Client, replicationBucketName)
	}
}

func deleteBucketCompletely(ctx context.Context, s3Client *s3.Client, bucketName string) {
	if !bucketExists(ctx, s3Client, bucketName) {
		return
	}

	paginator := s3.NewListObjectVersionsPaginator(s3Client, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucketName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			continue
		}

		var objectsToDelete []types.ObjectIdentifier
		for _, version := range page.Versions {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key:       version.Key,
				VersionId: version.VersionId,
			})
		}
		for _, deleteMarker := range page.DeleteMarkers {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key:       deleteMarker.Key,
				VersionId: deleteMarker.VersionId,
			})
		}

		if len(objectsToDelete) > 0 {
			_, err := s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucketName),
				Delete: &types.Delete{
					Objects: objectsToDelete,
					Quiet:   aws.Bool(true),
				},
			})
			if err != nil {
				fmt.Printf("Warning: failed to delete some objects from %s: %v\n", bucketName, err)
			}
		}
	}

	for i := 0; i < 3; i++ {
		_, err := s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err == nil {
			break
		}

		if i < 2 {
			time.Sleep(2 * time.Second)
		}
	}
}

func generateUniqueBucketNames(baseName string, count int, suffix string) []string {
	var bucketsNames []string

	for i := 0; i < count; i++ {
		uid := uuid.New().String()[:12]
		bucketName := fmt.Sprintf("%s-%s%s", baseName, uid, suffix)
		bucketsNames = append(bucketsNames, bucketName)
	}

	return bucketsNames
}

func getBucketInventory(ctx context.Context, s3Client *s3.Client, bucketName string) []types.InventoryConfiguration {
	result, err := s3Client.ListBucketInventoryConfigurations(ctx, &s3.ListBucketInventoryConfigurationsInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result.InventoryConfigurationList
}

func getBucketLifecycle(ctx context.Context, s3Client *s3.Client, bucketName string) *s3.GetBucketLifecycleConfigurationOutput {
	result, err := s3Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result
}

func getBucketLogging(ctx context.Context, s3Client *s3.Client, bucketName string) *s3.GetBucketLoggingOutput {
	result, err := s3Client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result
}

func getBucketNotifications(ctx context.Context, s3Client *s3.Client, bucketName string) *s3.GetBucketNotificationConfigurationOutput {
	result, err := s3Client.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result
}

func getBucketPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) *string {
	result, err := s3Client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result.Policy
}

func getBucketPublicAccessBlock(ctx context.Context, s3Client *s3.Client, bucketName string) *s3.GetPublicAccessBlockOutput {
	result, err := s3Client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result
}

func getBucketReplication(ctx context.Context, s3Client *s3.Client, bucketName string) *s3.GetBucketReplicationOutput {
	result, err := s3Client.GetBucketReplication(ctx, &s3.GetBucketReplicationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result
}

func getBucketTags(ctx context.Context, s3Client *s3.Client, bucketName string) []types.Tag {
	result, err := s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}
	return result.TagSet
}

func getBucketVersioning(ctx context.Context, s3Client *s3.Client, bucketName string) string {
	result, err := s3Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return ""
	}
	return string(result.Status)
}

func iamRoleExists(ctx context.Context, iamClient *iam.Client, roleName string) bool {
	_, err := iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	return err == nil
}

func lambdaFunctionExists(ctx context.Context, lambdaClient *lambda.Client, functionName string) bool {
	_, err := lambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(functionName),
	})
	return err == nil
}

func lambdaFunctionInvoke(ctx context.Context, lambdaClient *lambda.Client, functionName string, payload []byte) (*lambda.InvokeOutput, error) {
	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payload,
	}

	result, err := lambdaClient.Invoke(ctx, input)
	return result, err
}

func uploadRequestAndWait(t *testing.T, ctx context.Context, s3Client *s3.Client, stackName string, bucketNames []string, waitTime time.Duration) {
	// Use the new coordination system directly for better performance
	err := CreateBucketsWithCoordination(t, ctx, s3Client, stackName, bucketNames, waitTime)

	// For bucket_requested tests, we don't require.NoError because failure cases are expected
	// The tests will verify the actual bucket existence/non-existence using assert functions
	if err != nil {
		t.Logf("Bucket coordination completed with result: %v", err)
	} else {
		t.Logf("Bucket coordination completed successfully")
	}
}

func uploadToS3(ctx context.Context, s3Client *s3.Client, bucketName, key, content string) error {
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
	})
	return err
}

// Verify bucket configurations for a single bucket
func verifyBucketConfig(t *testing.T, ctx context.Context, s3Client *s3.Client, bucketName, stackName string) {
	isPublicBucket := strings.HasSuffix(bucketName, buckets.PublicSuffix)

	t.Run("Versioning", func(t *testing.T) {
		versioning := getBucketVersioning(ctx, s3Client, bucketName)
		assert.Equal(t, "Enabled", versioning)
	})

	if isPublicBucket {
		t.Run("PublicAccessBlock", func(t *testing.T) {
			publicAccessBlock := getBucketPublicAccessBlock(ctx, s3Client, bucketName)
			assert.NotNil(t, publicAccessBlock)
			assert.NotEmpty(t, publicAccessBlock.PublicAccessBlockConfiguration)
			assert.False(t, *publicAccessBlock.PublicAccessBlockConfiguration.BlockPublicPolicy)
		})

		t.Run("PublicAccessPolicy", func(t *testing.T) {
			policy := getBucketPolicy(ctx, s3Client, bucketName)
			assert.NotNil(t, policy)

			var policyDoc map[string]interface{}
			err := json.Unmarshal([]byte(*policy), &policyDoc)
			assert.NoError(t, err)

			statements := policyDoc["Statement"].([]interface{})
			statement := statements[0].(map[string]interface{})
			assert.Equal(t, "AllowPublicRead", statement["Sid"])
		})
	} else {
		t.Run("Lifecycle", func(t *testing.T) {
			lifecycle := getBucketLifecycle(ctx, s3Client, bucketName)
			assert.NotNil(t, lifecycle)
			assert.NotEmpty(t, lifecycle.Rules)
			assert.Equal(t, types.TransitionStorageClassGlacierIr, lifecycle.Rules[0].Transitions[0].StorageClass)
		})
	}

	t.Run("Notifications", func(t *testing.T) {
		notifications := getBucketNotifications(ctx, s3Client, bucketName)
		assert.NotNil(t, notifications)
		assert.NotNil(t, notifications.EventBridgeConfiguration)
	})

	t.Run("Inventory", func(t *testing.T) {
		inventory := getBucketInventory(ctx, s3Client, bucketName)
		assert.NotEmpty(t, inventory)
		assert.Equal(t, types.InventoryFrequencyDaily, inventory[0].Schedule.Frequency)
		assert.Equal(
			t,
			fmt.Sprintf("arn:aws:s3:::%s%s", stackName, buckets.ManagedSuffix),
			*inventory[0].Destination.S3BucketDestination.Bucket,
		)
		assert.Contains(t, *inventory[0].Destination.S3BucketDestination.Prefix, "inventory")
	})

	t.Run("Logging", func(t *testing.T) {
		logging := getBucketLogging(ctx, s3Client, bucketName)
		assert.NotNil(t, logging)
		assert.NotEmpty(t, logging.LoggingEnabled)
		assert.Equal(
			t,
			fmt.Sprintf("%s%s", stackName, buckets.ManagedSuffix),
			*logging.LoggingEnabled.TargetBucket,
		)
		assert.Contains(t, *logging.LoggingEnabled.TargetPrefix, "audit")
	})

	t.Run("Replication", func(t *testing.T) {
		replication := getBucketReplication(ctx, s3Client, bucketName)
		assert.NotNil(t, replication)
		if replication != nil && replication.ReplicationConfiguration != nil {
			assert.NotEmpty(t, replication.ReplicationConfiguration.Rules)
			if len(replication.ReplicationConfiguration.Rules) > 0 {
				assert.Equal(t, types.ReplicationRuleStatusEnabled, replication.ReplicationConfiguration.Rules[0].Status)
				assert.Equal(
					t,
					fmt.Sprintf("arn:aws:s3:::%s%s", bucketName, buckets.ReplicationSuffix),
					*replication.ReplicationConfiguration.Rules[0].Destination.Bucket,
				)
			}
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

// WaitConfig defines configuration for polling-based waiting
type WaitConfig struct {
	MaxTimeout      time.Duration
	PollInterval    time.Duration
	InitialDelay    time.Duration
	BackoffFactor   float64
	MaxPollInterval time.Duration
}

// DefaultWaitConfig returns a sensible default configuration
func DefaultWaitConfig() WaitConfig {
	return WaitConfig{
		MaxTimeout:      120 * time.Second,
		PollInterval:    2 * time.Second,
		InitialDelay:    1 * time.Second,
		BackoffFactor:   1.5,
		MaxPollInterval: 10 * time.Second,
	}
}

// WaitForCondition polls a condition function until it returns true or timeout is reached
func WaitForCondition(t *testing.T, description string, condition func() bool, config WaitConfig) bool {
	t.Logf("Waiting for %s (max timeout: %v)", description, config.MaxTimeout)

	// Initial delay before starting polling
	if config.InitialDelay > 0 {
		time.Sleep(config.InitialDelay)
	}

	start := time.Now()
	pollInterval := config.PollInterval
	attempt := 1

	for time.Since(start) < config.MaxTimeout {
		t.Logf("Checking condition: %s (attempt %d)", description, attempt)

		if condition() {
			elapsed := time.Since(start)
			t.Logf("Condition met for %s after %v (attempt %d)", description, elapsed, attempt)
			return true
		}

		// Sleep for current poll interval
		time.Sleep(pollInterval)

		// Apply exponential backoff
		if config.BackoffFactor > 1.0 {
			pollInterval = time.Duration(float64(pollInterval) * config.BackoffFactor)
			if pollInterval > config.MaxPollInterval {
				pollInterval = config.MaxPollInterval
			}
		}

		attempt++
	}

	elapsed := time.Since(start)
	t.Logf("Timeout waiting for %s after %v (%d attempts)", description, elapsed, attempt-1)
	return false
}

// WaitForDynamoDBRecord waits for a DynamoDB record to exist and optionally match conditions
func WaitForDynamoDBRecord(t *testing.T, clients *TestClients, tableName string, obj *files.S3Object,
	validator func(record db.ChecksumRecord) bool, config WaitConfig) (db.ChecksumRecord, bool) {

	var lastRecord db.ChecksumRecord
	var lastErr error

	condition := func() bool {
		record, err := db.GetChecksumRecord(context.Background(), clients.DynamoDB, tableName, *obj)
		lastErr = err
		if err != nil {
			t.Logf("Record not found yet for %s/%s: %v", obj.Bucket, obj.Key, err)
			return false
		}

		lastRecord = record
		if validator != nil {
			return validator(record)
		}
		return true
	}

	description := fmt.Sprintf("DynamoDB record for %s/%s", obj.Bucket, obj.Key)
	success := WaitForCondition(t, description, condition, config)

	if !success && lastErr != nil {
		t.Logf("Final error: %v", lastErr)
	}

	return lastRecord, success
}

// getSNSMessageCount gets the NumberOfMessagesPublished metric for an SNS topic from CloudWatch
func getSNSMessageCount(t *testing.T, ctx context.Context, clients *TestClients, stackName string) (float64, error) {
	topicName := fmt.Sprintf("%s-email-alert-notifications", stackName)

	// Get metrics for the last 5 minutes to ensure we capture recent activity
	endTime := time.Now()
	startTime := endTime.Add(-5 * time.Minute)

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/SNS"),
		MetricName: aws.String("NumberOfMessagesPublished"),
		Dimensions: []cwTypes.Dimension{
			{
				Name:  aws.String("TopicName"),
				Value: aws.String(topicName),
			},
		},
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(60), // 1 minute periods
		Statistics: []cwTypes.Statistic{cwTypes.StatisticSum},
	}

	result, err := clients.CloudWatch.GetMetricStatistics(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get CloudWatch metrics: %w", err)
	}

	// Sum all the datapoints
	var total float64
	for _, datapoint := range result.Datapoints {
		if datapoint.Sum != nil {
			total += *datapoint.Sum
		}
	}

	t.Logf("Found %d CloudWatch datapoints for SNS topic %s, total messages: %.0f",
		len(result.Datapoints), topicName, total)

	return total, nil
}
