package main

import (
	"context"
	"duracloud/internal/helpers"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"os"
)

var accountID string
var awsCtx helpers.AWSContext
var region string
var s3Client *s3.Client

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	accountID, err = helpers.GetAccountID(context.Background(), awsConfig)
	if err != nil {
		log.Fatalf("Unable to get AWS account ID: %v", err)
	}

	region = awsConfig.Region
	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event json.RawMessage) error {
	awsCtx = helpers.AWSContext{
		AccountID: accountID,
		Region:    region,
	}
	ctx = context.WithValue(ctx, helpers.AWSContextKey, awsCtx)

	bucketPrefix := os.Getenv("S3_BUCKET_PREFIX")
	bucketLimit, _ := helpers.GetBucketRequestLimit(os.Getenv("S3_MAX_BUCKETS_PER_REQUEST"))
	replicationRoleArn := os.Getenv("S3_REPLICATION_ROLE_ARN")

	var s3Event events.S3Event
	if err := json.Unmarshal(event, &s3Event); err != nil {
		log.Fatalf("Failed to parse event: %v", err)
		return err
	}

	e := helpers.S3EventWrapper{
		Event: &s3Event,
	}

	bucketName := e.BucketName()
	objectKey := e.ObjectKey()
	log.Printf("Received event for bucket name: %s, object key: %s", bucketName, objectKey)

	requestedBuckets, err := helpers.GetBuckets(ctx, s3Client, bucketName, objectKey, bucketLimit)
	if err != nil {
		log.Fatalf("Error retrieving buckets list: %v", err)
	}
	log.Printf("Retrieved %d buckets list from request file", len(requestedBuckets))

	bucketsStatus := make(map[string]string)
	managedBucketName := fmt.Sprintf("%s%s", bucketPrefix, helpers.ManagedSuffix)

	for _, requestedBucketName := range requestedBuckets {
		fullBucketName := fmt.Sprintf("%s-%s", bucketPrefix, requestedBucketName)
		replicationBucketName := fmt.Sprintf("%s%s", fullBucketName, helpers.ReplicationSuffix)
		message := ""
		log.Printf("Creating buckets: %s [%s]", fullBucketName, replicationBucketName)

		err := helpers.CreateNewBucket(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			continue
		}

		err = helpers.AddDenyUploadPolicy(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Standard")
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		err = helpers.EnableVersioning(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		err = helpers.AddExpiration(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		if helpers.IsPublicBucket(fullBucketName) {
			err = helpers.MakePublic(ctx, s3Client, fullBucketName)
			if err != nil {
				updateStatus(bucketsStatus, fullBucketName, err.Error())
				_ = rollback(ctx, s3Client, fullBucketName)
				continue
			}

			err = helpers.AddPublicPolicy(ctx, s3Client, fullBucketName)
			if err != nil {
				updateStatus(bucketsStatus, fullBucketName, err.Error())
				_ = rollback(ctx, s3Client, fullBucketName)
				continue
			}

			err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Public")
			if err != nil {
				updateStatus(bucketsStatus, fullBucketName, err.Error())
				_ = rollback(ctx, s3Client, fullBucketName)
				continue
			}

		} else {
			err := helpers.AddStandardLifecycle(ctx, s3Client, fullBucketName)
			if err != nil {
				updateStatus(bucketsStatus, fullBucketName, err.Error())
				_ = rollback(ctx, s3Client, fullBucketName)
				continue
			}
		}

		err = helpers.EnableEventBridge(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		err = helpers.EnableInventory(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		err = helpers.CreateNewBucket(ctx, s3Client, replicationBucketName)
		if err != nil {
			updateStatus(bucketsStatus, replicationBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		err = helpers.AddBucketTags(ctx, s3Client, replicationBucketName, bucketPrefix, "Replication")
		if err != nil {
			updateStatus(bucketsStatus, replicationBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			_ = rollback(ctx, s3Client, replicationBucketName)
			continue
		}

		err = helpers.EnableVersioning(ctx, s3Client, replicationBucketName)
		if err != nil {
			updateStatus(bucketsStatus, replicationBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			_ = rollback(ctx, s3Client, replicationBucketName)
			continue
		}

		err = helpers.AddExpiration(ctx, s3Client, replicationBucketName)
		if err != nil {
			updateStatus(bucketsStatus, replicationBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			_ = rollback(ctx, s3Client, replicationBucketName)
			continue
		}

		err = helpers.EnableReplication(ctx, s3Client, fullBucketName, replicationBucketName, replicationRoleArn)
		if err != nil {
			updateStatus(bucketsStatus, replicationBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			_ = rollback(ctx, s3Client, replicationBucketName)
			continue
		}

		err = helpers.AddReplicationLifecycle(ctx, s3Client, replicationBucketName)
		if err != nil {
			updateStatus(bucketsStatus, replicationBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			_ = rollback(ctx, s3Client, replicationBucketName)
			continue
		}

		err = helpers.RemovePolicy(ctx, s3Client, fullBucketName)
		if err != nil {
			updateStatus(bucketsStatus, fullBucketName, err.Error())
			_ = rollback(ctx, s3Client, fullBucketName)
			continue
		}

		message = fmt.Sprintf("Created bucket %s", fullBucketName)
		updateStatus(bucketsStatus, fullBucketName, message)
	}

	err = helpers.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
	if err != nil {
		log.Printf("Error writing bucket status to managed bucket: %v", err)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}

func rollback(ctx context.Context, s3Client *s3.Client, bucket string) error {
	return helpers.DeleteBucket(ctx, s3Client, bucket)
}

func updateStatus(buckets map[string]string, bucket string, message string) {
	buckets[bucket] = message
	log.Printf("[%s] %s", bucket, message)
}
