package helpers

import (
	"bufio"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"os"
	"regexp"
	"strings"
	"strconv"
)

const (
	IsBucketRequestedSuffix = "-bucket-requested"
	IsDuraCloudPrefix       = "duracloud-"
	IsLogsSuffix            = "-logs"
	IsManagedSuffix         = "-managed"
	IsPublicSuffix          = "-public"
	IsReplicationSuffix     = "-replication"
)

func IsBucketRequestBucket(name string) bool {
	return strings.HasSuffix(name, IsBucketRequestedSuffix)
}

// IsIgnoreFilesBucket buckets excluded from checksum processing
func IsIgnoreFilesBucket(name string) bool {
	return IsBucketRequestBucket(name) || IsRestrictedBucket(name)
}

func IsDuraCloudBucket(name string) bool {
	return strings.HasPrefix(name, IsDuraCloudPrefix)
}

func IsLogsBucket(name string) bool {
	return strings.HasSuffix(name, IsLogsSuffix)
}

func IsManagedBucket(name string) bool {
	return strings.HasSuffix(name, IsManagedSuffix)
}

func IsPublicBucket(name string) bool {
	return strings.HasSuffix(name, IsPublicSuffix)
}

func IsReplicationBucket(name string) bool {
	return strings.HasSuffix(name, IsReplicationSuffix)
}

// IsRestrictedBucket buckets with restricted access permissions for s3 users
func IsRestrictedBucket(name string) bool {
	return IsLogsBucket(name) || IsManagedBucket(name) || IsReplicationBucket(name)
}

func ValidateBucketName(name string) bool {
	var (
		whitelist  = "a-zA-Z0-9-"
		disallowed = regexp.MustCompile(fmt.Sprintf("[^%s]+", whitelist))
	)
	return !disallowed.MatchString(name)
}

func GetBucketRequestLimit(ctx context.Context) int {
	var maxBucketsEnv = os.Getenv("S3_MAX_BUCKETS_PER_REQUEST")
	var maxBuckets, err = strconv.Atoi(maxBucketsEnv)

	if err != nil {
		log.Fatalf("Unable to read max buckets per request environment variable due to : %v", err)
	}
	return maxBuckets
}

// getBuckets retrieves a list of valid bucket names from an S3 object, validates them, and enforces a maximum limit.
func GetBuckets(ctx context.Context, client *s3.Client, bucket string, key string) []string {
	var buckets []string

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		log.Fatalf("failed to get object: %s from %s due to %s", key, bucket, err)
		return nil
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Reading bucket name: %s", line)
		if ValidateBucketName(line) {
			buckets = append(buckets, line)
		} else {
			log.Fatalf("invalid bucket name requested: %s", line)
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading response: %v", err)
		return nil
	}

	// TODO: do we want to error like this or ignore extras (i.e. don't append additional buckets above)?
	var maxBuckets = GetBucketRequestLimit(ctx)
	bucketsRequested := len(buckets)
	if bucketsRequested >= maxBuckets {
		log.Fatalf("Exceeded maximum allowed buckets per request [%s] with [%s]",
			maxBuckets, bucketsRequested)
		return nil
	}

	return buckets
}


