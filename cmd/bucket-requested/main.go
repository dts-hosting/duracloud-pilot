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

var s3Client *s3.Client

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}
	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event json.RawMessage) error {

	bucketPrefix := os.Getenv("S3_BUCKET_PREFIX")
	log.Printf("Using bucket prefix: %s", bucketPrefix)

	replicationRoleArn := os.Getenv("S3_REPLICATION_ROLE_ARN")
	log.Printf("Using replication role ARN: %s", replicationRoleArn)

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

	requestedBuckets, err := helpers.GetBuckets(ctx, s3Client, bucketName, objectKey)
	if err != nil {
		log.Panicln(err)
	}
	log.Printf("Retrieved %d buckets list from request file", len(requestedBuckets))

	// Create new buckets
	for _, requestedBucketName := range requestedBuckets {

		fullBucketName := fmt.Sprintf("%s-%s", bucketPrefix, requestedBucketName)
		log.Printf("Creating bucket  %v", fullBucketName)
		helpers.CreateNewBucket(ctx, s3Client, fullBucketName)
		helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Standard")
		helpers.AddDenyAllPolicy(ctx, s3Client, fullBucketName)
		helpers.EnableVersioning(ctx, s3Client, fullBucketName)
		helpers.AddExpiration(ctx, s3Client, fullBucketName)

		if helpers.IsPublicBucket(bucketName) {
			helpers.MakePublic(ctx, s3Client, fullBucketName)
			helpers.AddPublicPolicy(ctx, s3Client, fullBucketName)
			//AddPublicTags(ctx, s3Client, fullBucketName)
			helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Public")
		} else {
			helpers.EnableLifecycle(ctx, s3Client, fullBucketName)
		}
		helpers.EnableEventBridge(ctx, s3Client, fullBucketName)
		helpers.EnableInventory(ctx, s3Client, fullBucketName)
		var replicationBucketName = fmt.Sprintf("%s%s", fullBucketName, helpers.IsReplicationSuffix)
		helpers.CreateNewBucket(ctx, s3Client, replicationBucketName)
		helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Replication")
		helpers.RemovePolicy(ctx, s3Client, fullBucketName)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
