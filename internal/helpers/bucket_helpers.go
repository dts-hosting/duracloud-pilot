package helpers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BucketRequestedSuffix = "-bucket-requested"
	DuraCloudPrefix       = "duracloud-"
	LogsSuffix            = "-logs"
	ManagedSuffix         = "-managed"
	PublicSuffix          = "-public"
	ReplicationSuffix     = "-replication"
)

func AddBucketTags(ctx context.Context, s3Client *s3.Client, bucketName string, stackName string, bucketType string) error {
	_, err := s3Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(bucketName),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("Application"), Value: aws.String("Duracloud")},
				{Key: aws.String("StackName"), Value: aws.String(stackName)},
				{Key: aws.String("BucketType"), Value: aws.String(bucketType)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add  bucket tags: %v", err)
	}
	return nil
}

func AddDenyUploadPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	// apply a default deny-all policy
	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":       "DenyAllUploads",
				"Effect":    "Deny",
				"Principal": "*",
				"Action":    "s3:PutObject",
				"Resource":  fmt.Sprintf("arn:aws:s3:::%s/*", bucketName),
			},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %v", err)
	}

	_, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return fmt.Errorf("failed to put bucket policy: %v", err)
	}
	return nil
}

func AddExpiration(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	days := int32(7)
	_, err := s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("ExpireOldVersionsAfter7Days"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")}, // Applies to all objects
					NoncurrentVersionExpiration: &types.NoncurrentVersionExpiration{
						NoncurrentDays: &days,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set lifecycle rule: %v", err)
	}
	return nil
}

func AddPublicPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":       "AllowPublicRead",
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    "s3:GetObject",
				"Resource":  fmt.Sprintf("arn:aws:s3:::%s/*", bucketName),
			},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket policy: %v", err)
	}

	_, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return fmt.Errorf("failed to apply public bucket policy: %v", err)
	}
	return nil
}

func CreateNewBucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	awsCtx, ok := ctx.Value(AWSContextKey).(AWSContext)
	if !ok {
		return fmt.Errorf("error retrieving aws context")
	}

	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(awsCtx.Region),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %v", err)
	}
	return nil
}

func DeleteBucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	_, err := s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %v", err)
	}
	return nil
}

func EnableEventBridge(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	_, err := s3Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket: aws.String(bucketName),
		NotificationConfiguration: &types.NotificationConfiguration{
			EventBridgeConfiguration: &types.EventBridgeConfiguration{},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enable EventBridge notifications: %v", err)
	}
	return nil
}

func EnableInventory(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	awsCtx, ok := ctx.Value(AWSContextKey).(AWSContext)
	if !ok {
		return fmt.Errorf("error retrieving aws context")
	}

	var destBucket = fmt.Sprintf("%s%s", bucketName, ManagedSuffix)
	_, err := s3Client.PutBucketInventoryConfiguration(ctx, &s3.PutBucketInventoryConfigurationInput{
		Bucket: aws.String(bucketName),
		Id:     aws.String("InventoryReport"),
		InventoryConfiguration: &types.InventoryConfiguration{
			IsEnabled:              aws.Bool(true),
			Id:                     aws.String("InventoryReport"),
			IncludedObjectVersions: types.InventoryIncludedObjectVersionsAll,
			Schedule: &types.InventorySchedule{
				Frequency: types.InventoryFrequencyDaily,
			},
			Destination: &types.InventoryDestination{
				S3BucketDestination: &types.InventoryS3BucketDestination{
					AccountId: aws.String(awsCtx.AccountID),
					Bucket:    aws.String("arn:aws:s3:::" + destBucket),
					Format:    types.InventoryFormatCsv,
					Prefix:    aws.String("inventory/"),
				},
			},
			OptionalFields: []types.InventoryOptionalField{
				types.InventoryOptionalFieldSize,
				types.InventoryOptionalFieldLastModifiedDate,
				types.InventoryOptionalFieldStorageClass,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to enable inventory configuration: %v", err)
	}
	return nil
}

func EnableLifecycle(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	daysToGlacier := int32(2)

	_, err := s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("TransitionToGlacierIRIn2Days"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")}, // All objects
					Transitions: []types.Transition{
						{
							Days:         &daysToGlacier,
							StorageClass: types.TransitionStorageClassGlacierIr,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to configure lifecycle: %v", err)
	}
	return nil
}

func EnableReplication(ctx context.Context, s3Client *s3.Client, sourceBucketName string, replicationBucketName string, replicationRoleArn string) error {
	_, err := s3Client.PutBucketReplication(ctx, &s3.PutBucketReplicationInput{
		Bucket: aws.String(sourceBucketName),
		ReplicationConfiguration: &types.ReplicationConfiguration{
			Role: aws.String(replicationRoleArn),
			Rules: []types.ReplicationRule{
				{
					ID:       aws.String("ReplicateAll"),
					Status:   types.ReplicationRuleStatusEnabled,
					Priority: aws.Int32(1),
					Filter:   &types.ReplicationRuleFilter{Prefix: aws.String("")},
					Destination: &types.Destination{
						Bucket: aws.String(fmt.Sprintf("arn:aws:s3:::%s", replicationBucketName)),
						ReplicationTime: &types.ReplicationTime{
							Status: types.ReplicationTimeStatusEnabled,
							Time: &types.ReplicationTimeValue{
								Minutes: aws.Int32(15),
							},
						},
						Metrics: &types.Metrics{
							Status: types.MetricsStatusEnabled,
							EventThreshold: &types.ReplicationTimeValue{
								Minutes: aws.Int32(15),
							},
						},
					},
					DeleteMarkerReplication: &types.DeleteMarkerReplication{
						Status: types.DeleteMarkerReplicationStatusEnabled,
					},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to enable replication configuration: %v", err)
	}
	return nil
}

func EnableVersioning(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	_, err := s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enable versioning: %v", err)
	}
	return nil
}

func GetBucketRequestLimit(bucketsPerRequest string) (int, error) {
	maxBuckets, err := strconv.Atoi(bucketsPerRequest)

	if err != nil {
		return -1, fmt.Errorf("unable to read max buckets per request variable due to: %v", err)
	}

	return maxBuckets, nil
}

// GetBuckets retrieves a list of valid bucket names from an S3 object, validates them, and enforces a maximum limit.
func GetBuckets(ctx context.Context, s3Client *s3.Client, bucket string, key string, limit int) ([]string, error) {
	var buckets []string

	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get object: %s from %s due to %s", key, bucket, err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		if ValidateBucketName(line) {
			buckets = append(buckets, line)
		} else {
			return nil, fmt.Errorf("invalid bucket name requested: %s", line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	bucketsRequested := len(buckets)
	if bucketsRequested >= limit {
		return nil, fmt.Errorf("exceeded maximum allowed buckets per request [%d] with [%d]",
			limit, bucketsRequested)
	}

	return buckets, nil
}

func IsBucketRequestBucket(name string) bool {
	return strings.HasSuffix(name, BucketRequestedSuffix)
}

// IsIgnoreFilesBucket buckets excluded from checksum processing
func IsIgnoreFilesBucket(name string) bool {
	return IsBucketRequestBucket(name) || IsRestrictedBucket(name)
}

func IsDuraCloudBucket(name string) bool {
	return strings.HasPrefix(name, DuraCloudPrefix)
}

func IsLogsBucket(name string) bool {
	return strings.HasSuffix(name, LogsSuffix)
}

func IsManagedBucket(name string) bool {
	return strings.HasSuffix(name, ManagedSuffix)
}

func IsPublicBucket(name string) bool {
	return strings.HasSuffix(name, PublicSuffix)
}

func IsReplicationBucket(name string) bool {
	return strings.HasSuffix(name, ReplicationSuffix)
}

// IsRestrictedBucket buckets with restricted access permissions for s3 users
func IsRestrictedBucket(name string) bool {
	return IsLogsBucket(name) || IsManagedBucket(name) || IsReplicationBucket(name)
}

func MakePublic(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	blockFalse := false
	_, err := s3Client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: aws.String(bucketName),
		PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       &blockFalse,
			IgnorePublicAcls:      &blockFalse,
			BlockPublicPolicy:     &blockFalse,
			RestrictPublicBuckets: &blockFalse,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to disable public access block: %v", err)
	}
	return nil
}

func RemovePolicy(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	_, err := s3Client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return fmt.Errorf("failed to delete bucket policy: %v", err)
	}
	return nil
}

func ValidateBucketName(name string) bool {
	var (
		whitelist  = "a-zA-Z0-9-"
		disallowed = regexp.MustCompile(fmt.Sprintf("[^%s]+", whitelist))
	)
	return !disallowed.MatchString(name)
}

func WriteStatus(ctx context.Context, s3Client *s3.Client, bucketName string, log map[string]string) error {
	var builder strings.Builder
	for bucket, status := range log {
		builder.WriteString(fmt.Sprintf("[%s] %s\n", bucket, status))
	}
	logContent := builder.String()
	reader := bytes.NewReader([]byte(logContent))
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	key := fmt.Sprintf("logs/bucket-request-log-%s.txt", timestamp)
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   reader,
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return fmt.Errorf("Failed to write bucket status: %v", err)
	}

	return nil
}
