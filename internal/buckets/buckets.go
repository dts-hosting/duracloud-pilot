package buckets

import (
	"bufio"
	"bytes"
	"context"
	"duracloud/internal/accounts"
	"duracloud/internal/files"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	AwsPrefix       = "aws-"
	DuraCloudPrefix = "duracloud-"

	BucketRequestedSuffix = "-bucket-requested"
	LogsSuffix            = "-logs"
	ManagedSuffix         = "-managed"
	PublicSuffix          = "-public"
	ReplicationSuffix     = "-repl"

	ApplicationTagKey                = "Application"
	ApplicationTagValue              = "DuraCloud"
	BucketNameMaxChars               = 63
	BucketRequestedFileErrorKey      = "error-processing-bucket-requested-file"
	BucketTypeTagKey                 = "BucketType"
	DefaultBucketRequestLimit        = 5
	InventoryConfigId                = "inventory"
	LifeCycleTransitionToGlacierDays = 7
	NonCurrentVersionExpirationDays  = 2
	PublicTagValue                   = "Public"
	ReplicationTagValue              = "Replication"
	StackNameTagKey                  = "StackName"
	StandardTagValue                 = "Standard"

	// Status messages
	StatusBucketCreatedSuccessfully = "Bucket created successfully"
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
func GetBuckets(ctx context.Context, s3Client *s3.Client, obj files.S3Object, limit int) ([]string, error) {
	var buckets []string

	resp, err := files.DownloadObject(ctx, s3Client, obj, false)

	if err != nil {
		return nil, ErrorRetrievingObject(obj.Key, obj.Bucket, err)
	}
	defer func() { _ = resp.Close() }()

	scanner := bufio.NewScanner(resp)

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
	if bucketsRequested > limit {
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
	obj := files.NewS3Object(bucketName, key)

	err := files.UploadObject(ctx, s3Client, obj, reader, "text/plain")
	if err != nil {
		return ErrorBucketStatusUploadFailed(err)
	}

	return nil
}
