package helpers

import (
	"bufio"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	//TODO "encoding/json"
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
func GetBuckets(ctx context.Context, s3Client *s3.Client, bucket string, key string) []string {
	var buckets []string

	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
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

// getBuckets retrieves a list of valid bucket names from an S3 object, validates them, and enforces a maximum limit.
func CreateBucket(ctx context.Context, s3Client *s3.Client, bucketName string, bucketPrefix string) {

	fullBucketName := fmt.Sprintf("%s-%s", bucketPrefix, bucketName)
	log.Printf("Creating bucket  %v", fullBucketName)
	region := os.Getenv("AWS_REGION") 
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(fullBucketName),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
            LocationConstraint: types.BucketLocationConstraint(region),
        },
	})
	if err != nil {
		log.Fatalf("failed to create bucket: %v", err)
	}

	_, err = s3Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(fullBucketName),
		Tagging: &types.Tagging{
            TagSet: []types.Tag{
                {Key: aws.String("Application"), Value: aws.String("Duracloud")},
                {Key: aws.String("StackName"), Value: aws.String(bucketPrefix)},
                {Key: aws.String("BucketType"), Value: aws.String("Standard")},
            },
        },
	})
	if err != nil {
		log.Fatalf("failed to add  bucket tags: %v", err)
	}
	log.Printf("Bucket Tags added")

	log.Printf("Bucket Deny-all policy generating")
	// apply a default deny-all policy
	/*
	policy := map[string]interface{}{
        "Version": "2012-10-17",
        "Statement": []map[string]interface{}{
            {
                "Effect":   "Deny",
                "Principal": "*",
                "Action":   "*",
                "Resource": fmt.Sprintf("arn:aws:s3:::%s/*", fullBucketName),
            },
            {
                "Effect":   "Deny",
                "Principal": "*",
                "Action":   "*",
                "Resource": fmt.Sprintf("arn:aws:s3:::%s", fullBucketName),
            },
        },
    }

	policyJSON, err := json.Marshal(policy)
	if err != nil {
        log.Fatalf("failed to marshal policy: %v", err)
    }

	// TODO uncomment. Proved working but makes testing impossible
    _, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
        Bucket: aws.String(fullBucketName),
        Policy: aws.String(string(policyJSON)),
    })
    if err != nil {
        log.Fatalf("failed to put bucket policy: %v", err)
    }
	*/
	log.Printf("TODO INOP: Applied Bucket Deny-all policy")

	log.Printf("Enable Bucket versioning")
	_, err = s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(fullBucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		log.Fatalf("failed to enable versioning: %v", err)
	}
	fmt.Println("Bucket versioning enabled.")

}
