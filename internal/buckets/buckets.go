package buckets

import (
	"bufio"
	"bytes"
	"context"
	"duracloud/internal/accounts"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	AwsPrefix       = "aws-"
	DuraCloudPrefix = "duracloud-"

	BucketRequestedSuffix = "-bucket-requested"
	LogsSuffix            = "-logs"
	ManagedSuffix         = "-managed"
	PublicSuffix          = "-public"
	ReplicationSuffix     = "-repl"

	ApplicationTagValue              = "DuraCloud"
	BucketNameMaxChars               = 63
	BucketRequestedFileErrorKey      = "error-processing-bucket-requested-file"
	InventoryConfigId                = "inventory"
	LifeCycleTransitionToGlacierDays = 7
	NonCurrentVersionExpirationDays  = 2
	PublicTagValue                   = "Public"
	ReplicationTagValue              = "Replication"
	StandardTagValue                 = "Standard"
)

var (
	ReservedPrefixes = []string{
		"-",
		AwsPrefix,
		DuraCloudPrefix,
	}

	ReservedSuffixes = []string{
		"-",
		BucketRequestedSuffix,
		LogsSuffix,
		ManagedSuffix,
		ReplicationSuffix,
	}
)

type BucketRequest struct {
	Name               string
	Prefix             string
	ManagedBucketName  string
	ReplicationRoleArn string
	ResultChan         chan<- map[string]string
}

func (b *BucketRequest) FullName() string {
	return fmt.Sprintf("%s-%s", b.Prefix, b.Name)
}

func (b *BucketRequest) ReplicationName() string {
	return fmt.Sprintf("%s%s", b.FullName(), ReplicationSuffix)
}

func AddBucketTags(ctx context.Context, s3Client *s3.Client, bucketName string, stackName string, bucketType string) error {
	_, err := s3Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(bucketName),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("Application"), Value: aws.String(ApplicationTagValue)},
				{Key: aws.String("StackName"), Value: aws.String(stackName)},
				{Key: aws.String("BucketType"), Value: aws.String(bucketType)},
			},
		},
	})
	if err != nil {
		return ErrorApplyingBucketTags(err)
	}
	return nil
}

func AddDenyUploadPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	// apply a default deny-upload policy
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
		return ErrorMarshallingPolicy(err)
	}

	_, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return ErrorApplyingBucketPolicy(err)
	}
	return nil
}

func AddExpiration(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	daysToExpiration := int32(NonCurrentVersionExpirationDays)

	_, err := s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("ExpireOldVersions"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")},
					NoncurrentVersionExpiration: &types.NoncurrentVersionExpiration{
						NoncurrentDays: &daysToExpiration,
					},
				},
			},
		},
	})
	if err != nil {
		return ErrorApplyingExpiration(err)
	}
	return nil
}

func AddLifecycle(ctx context.Context, s3Client *s3.Client, bucketName string, storageClass types.TransitionStorageClass, transitionDays int32) error {
	_, err := s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String(fmt.Sprintf("TransitionTo%s", storageClass)),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")},
					Transitions: []types.Transition{
						{
							Days:         &transitionDays,
							StorageClass: storageClass,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return ErrorApplyingLifecycle(err)
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
		return ErrorMarshallingBucketPolicy(err)
	}

	_, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return ErrorApplyingBucketPolicy(err)
	}
	return nil
}

func AddReplicationLifecycle(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	daysToGlacier := int32(LifeCycleTransitionToGlacierDays)
	return AddLifecycle(ctx, s3Client, bucketName, types.TransitionStorageClassDeepArchive, daysToGlacier)
}

func AddStandardLifecycle(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	daysToGlacier := int32(LifeCycleTransitionToGlacierDays)
	return AddLifecycle(ctx, s3Client, bucketName, types.TransitionStorageClassGlacierIr, daysToGlacier)
}

func CreateNewBucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	awsCtx, ok := ctx.Value(accounts.AWSContextKey).(accounts.AWSContext)
	if !ok {
		return ErrorAWSContextRetrieval()
	}

	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(awsCtx.Region),
		},
	})
	if err != nil {
		return ErrorBucketCreationFailed(err)
	}
	return nil
}

func DeleteBucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	_, err := s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return ErrorBucketDeletionFailed(err)
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
		return ErrorApplyingEventBridge(err)
	}
	return nil
}

func EnableInventory(ctx context.Context, s3Client *s3.Client, srcBucketName string, destBucketName string) error {
	awsCtx, ok := ctx.Value(accounts.AWSContextKey).(accounts.AWSContext)
	if !ok {
		return ErrorAWSContextRetrieval()
	}

	_, err := s3Client.PutBucketInventoryConfiguration(ctx, &s3.PutBucketInventoryConfigurationInput{
		Bucket: aws.String(srcBucketName),
		Id:     aws.String(InventoryConfigId),
		InventoryConfiguration: &types.InventoryConfiguration{
			IsEnabled:              aws.Bool(true),
			Id:                     aws.String(InventoryConfigId),
			IncludedObjectVersions: types.InventoryIncludedObjectVersionsAll,
			Schedule: &types.InventorySchedule{
				Frequency: types.InventoryFrequencyDaily,
			},
			Destination: &types.InventoryDestination{
				S3BucketDestination: &types.InventoryS3BucketDestination{
					AccountId: aws.String(awsCtx.AccountID),
					Bucket:    aws.String(fmt.Sprintf("arn:aws:s3:::%s", destBucketName)),
					Format:    types.InventoryFormatCsv,
					Prefix:    aws.String(InventoryConfigId),
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
		return ErrorApplyingInventory(err)
	}
	return nil
}

func EnableLogging(ctx context.Context, s3Client *s3.Client, srcBucketName string, destBucketName string) error {
	_, err := s3Client.PutBucketLogging(ctx, &s3.PutBucketLoggingInput{
		Bucket: aws.String(srcBucketName),
		BucketLoggingStatus: &types.BucketLoggingStatus{
			LoggingEnabled: &types.LoggingEnabled{
				TargetBucket: aws.String(destBucketName),
				TargetPrefix: aws.String(fmt.Sprintf("audit/%s/", srcBucketName)),
			},
		},
	})
	if err != nil {
		return ErrorApplyingLogging(err)
	}
	return nil
}

func EnableReplication(ctx context.Context, s3Client *s3.Client, srcBucketName string, replBucketName string, replRoleArn string) error {
	_, err := s3Client.PutBucketReplication(ctx, &s3.PutBucketReplicationInput{
		Bucket: aws.String(srcBucketName),
		ReplicationConfiguration: &types.ReplicationConfiguration{
			Role: aws.String(replRoleArn),
			Rules: []types.ReplicationRule{
				{
					ID:       aws.String("ReplicateAll"),
					Status:   types.ReplicationRuleStatusEnabled,
					Priority: aws.Int32(1),
					Filter:   &types.ReplicationRuleFilter{Prefix: aws.String("")},
					Destination: &types.Destination{
						Bucket: aws.String(fmt.Sprintf("arn:aws:s3:::%s", replBucketName)),
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
		return ErrorApplyingReplication(err)
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
		return ErrorApplyingVersioning(err)
	}
	return nil
}

func GetBucketPrefix(bucketName string) string {
	parts := strings.Split(bucketName, "-")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:2], "-")
}

func GetBucketRequestLimit(bucketsPerRequest string) (int, error) {
	maxBuckets, err := strconv.Atoi(bucketsPerRequest)

	if err != nil {
		return -1, ErrorReadingMaxBucketsPerRequest(err)
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
		return nil, ErrorRetrievingObject(key, bucket, err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if ValidateBucketName(ctx, line) {
			buckets = append(buckets, line)
		} else {
			return nil, ErrorInvalidBucketName(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, ErrorReadingResponse(err)
	}

	bucketsRequested := len(buckets)
	if bucketsRequested >= limit {
		return nil, ErrorExceededMaxBucketsPerRequest(limit, bucketsRequested)
	}

	return buckets, nil
}

func HasReservedPrefix(name string) bool {
	for _, prefix := range ReservedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

func HasReservedSuffix(name string) bool {
	for _, suffix := range ReservedSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return false
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
		return ErrorApplyingPublicAccessBlock(err)
	}
	return nil
}

func RemovePolicy(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	_, err := s3Client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return ErrorDeletingBucketPolicy(err)
	}
	return nil
}

func ValidateBucketName(ctx context.Context, name string) bool {
	awsCtx, ok := ctx.Value(accounts.AWSContextKey).(accounts.AWSContext)
	if !ok {
		return false
	}

	if len(name) < 1 || len(name) > BucketNameMaxChars-(len(awsCtx.StackName)+len(ReplicationSuffix)) {
		return false
	}

	var (
		whitelist  = "a-z0-9-"
		disallowed = regexp.MustCompile(fmt.Sprintf("[^%s]+", whitelist))
	)

	if disallowed.MatchString(name) || HasReservedPrefix(name) || HasReservedSuffix(name) {
		return false
	}

	return true
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
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return ErrorBucketStatusUploadFailed(err)
	}

	return nil
}
