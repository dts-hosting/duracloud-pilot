package helpers

import (
	"bufio"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"encoding/json"
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

func CreateNewBucket(ctx context.Context, s3Client *s3.Client, bucketName string) {
	region := os.Getenv("AWS_REGION")
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
            LocationConstraint: types.BucketLocationConstraint(region),
        },
	})
	if err != nil {
		log.Fatalf("failed to create bucket: %v", err)
	}
}

func AddBucketTags(ctx context.Context, s3Client *s3.Client, bucketName string, stackName string, bucketType string) {
	_, err := s3Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(bucketName),
		Tagging: &types.Tagging{
            TagSet: []types.Tag{
                {Key: aws.String("Application"), Value: aws.String("Duracloud")},
                {Key: aws.String("StackName"), Value: aws.String(stackName)},
                {Key: aws.String("BucketType"), Value: aws.String("Standard")},
            },
        },
	})
	if err != nil {
		log.Fatalf("failed to add  bucket tags: %v", err)
	}
	log.Printf("Bucket Tags added")

}

func AddDenyAllPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) {
	log.Printf("Bucket Deny-all policy generating")
	// apply a default deny-all policy
	policy := map[string]interface{}{
        "Version": "2012-10-17",
        "Statement": []map[string]interface{}{
            {
				"Sid": "DenyAllUploads",
                "Effect":   "Deny",
                "Principal": "*",
                "Action":   "s3:PutObject",
                "Resource": fmt.Sprintf("arn:aws:s3:::%s/*", bucketName),
            },
        },
    }

	policyJSON, err := json.Marshal(policy)
	if err != nil {
        log.Fatalf("failed to marshal policy: %v", err)
    }

	_, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
        Bucket: aws.String(bucketName),
        Policy: aws.String(string(policyJSON)),
    })
    if err != nil {
        log.Fatalf("failed to put bucket policy: %v", err)
    }
	log.Printf("Applied Bucket Deny-all policy")
}

func EnableVersioning(ctx context.Context, s3Client *s3.Client, bucketName string) {
	log.Printf("Enable Bucket versioning")
	_, err := s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		log.Fatalf("failed to enable versioning: %v", err)
	}
	log.Println("Bucket versioning enabled.")
}

func AddExpiration(ctx context.Context, s3Client *s3.Client, bucketName string) {
	days := int32(7)
	log.Printf("Bucket Non-current expiration being set to %d days", days)
	_, err := s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID: aws.String("ExpireOldVersionsAfter7Days"),
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
		log.Fatalf("failed to set lifecycle rule: %v", err)
	}
	log.Printf("Lifecycle rule set: Non-current versions expire after %d days", days)
}

func MakePublic(ctx context.Context, s3Client *s3.Client, bucketName string) {
	log.Printf("Public Bucket detected")
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
		log.Fatalf("failed to disable public access block: %v", err)
	}
	log.Println("Public access block disabled.")
}

func AddPublicPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) {
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
		log.Fatalf("failed to marshal bucket policy: %v", err)
	}

	_, err = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		log.Fatalf("failed to apply public bucket policy: %v", err)
	}
	log.Println("Public bucket policy applied.")
}

func RemovePolicy(ctx context.Context, s3Client *s3.Client, bucketName string) {
    _, err := s3Client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
        Bucket: aws.String(bucketName),
    })

    if err != nil {
        log.Fatalf("Failed to delete bucket policy: %v", err)
    }
    log.Printf("Bucket policy removed from: %s\n", bucketName)
}

func EnableInventory(ctx context.Context, s3Client *s3.Client, bucketName string) {
	var arn = os.Getenv("S3_REPLICATION_ROLE_ARN")
	var destBucket = fmt.Sprintf("%s%s", bucketName, IsManagedSuffix)
    _, err := s3Client.PutBucketInventoryConfiguration(ctx, &s3.PutBucketInventoryConfigurationInput{
        Bucket: aws.String(bucketName),
        Id:     aws.String("InventoryReport"),
        InventoryConfiguration: &types.InventoryConfiguration{
			IsEnabled: aws.Bool(true),
			Id:     aws.String("InventoryReport"),
            IncludedObjectVersions: types.InventoryIncludedObjectVersionsAll,
            Schedule: &types.InventorySchedule{
                Frequency: types.InventoryFrequencyDaily,
            },
            Destination: &types.InventoryDestination{
                S3BucketDestination: &types.InventoryS3BucketDestination{
                    AccountId: aws.String(arn), // your AWS account ID
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
        log.Fatalf("failed to enable inventory configuration: %v", err)
    }

    log.Println("Inventory configuration enabled.")
}

func EnableLifecycle(ctx context.Context, s3Client *s3.Client, bucketName string) {
	daysToIA      := int32(30)
	daysToGlacier := int32(60)

	_, err := s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("IAThenGlacierIR"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")}, // All objects
					Transitions: []types.Transition{
						{
							Days:         &daysToIA,
							StorageClass: types.TransitionStorageClassStandardIa,
						},
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
		log.Fatalf("failed to configure lifecycle: %v", err)
	}
	log.Printf("Lifecycle rule set: Transition to  IA in %d days, then Glacier IR after %d days.", daysToIA, daysToGlacier)

}

func EnableEventBridge(ctx context.Context, s3Client *s3.Client, bucketName string) {
	_, err := s3Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
        Bucket: aws.String(bucketName),
        NotificationConfiguration: &types.NotificationConfiguration{
            EventBridgeConfiguration: &types.EventBridgeConfiguration{},
        },
    })
    if err != nil {
        log.Fatalf("failed to enable EventBridge notifications: %v", err)
    }
    log.Printf("EventBridge notifications enabled on bucket: %v", bucketName)
}

func CreateBucket(ctx context.Context, s3Client *s3.Client, bucketName string, stackName string) {

	fullBucketName := fmt.Sprintf("%s-%s", stackName, bucketName)
	log.Printf("Creating bucket  %v", fullBucketName)
	CreateNewBucket(ctx, s3Client, fullBucketName)
	AddBucketTags(ctx, s3Client, fullBucketName, stackName, "Standard")
	AddDenyAllPolicy(ctx, s3Client, fullBucketName)
	EnableVersioning(ctx, s3Client, fullBucketName)
	AddExpiration(ctx, s3Client, fullBucketName)

	if IsPublicBucket(bucketName) {
		MakePublic(ctx, s3Client, fullBucketName)
		AddPublicPolicy(ctx, s3Client, fullBucketName)
		//AddPublicTags(ctx, s3Client, fullBucketName)
		AddBucketTags(ctx, s3Client, fullBucketName, stackName, "Public")
	} else {
		EnableLifecycle(ctx, s3Client, fullBucketName)
	}
	EnableEventBridge(ctx, s3Client, fullBucketName)
	EnableInventory(ctx, s3Client, fullBucketName)
	var replicationBucketName = fmt.Sprintf("%s%s", fullBucketName, IsReplicationSuffix)
	CreateNewBucket(ctx, s3Client, replicationBucketName)
	AddBucketTags(ctx, s3Client, fullBucketName, stackName, "Replication")
	RemovePolicy(ctx, s3Client, fullBucketName)
}
