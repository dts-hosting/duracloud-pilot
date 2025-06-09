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

	bucketLimit, _ := helpers.GetBucketRequestLimit(ctx)

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

	createdBuckets := make([]string, 0, bucketLimit)
	for _, requestedBucketName := range requestedBuckets {

		fullBucketName := fmt.Sprintf("%s-%s", bucketPrefix, requestedBucketName)
		log.Printf("Creating bucket  %v", fullBucketName)
		err := helpers.CreateNewBucket(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panicf("Unable to create bucket: %s", err)
		}
		createdBuckets = append(createdBuckets, fullBucketName)
		err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Standard")
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		err = helpers.AddDenyAllPolicy(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		err = helpers.EnableVersioning(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		err = helpers.AddExpiration(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		if helpers.IsPublicBucket(bucketName) {
			err = helpers.MakePublic(ctx, s3Client, fullBucketName)
			if err != nil {
				rollback(ctx, s3Client, createdBuckets)
				log.Panic(err.Error())
			}

			err = helpers.AddPublicPolicy(ctx, s3Client, fullBucketName)
			if err != nil {
				rollback(ctx, s3Client, createdBuckets)
				log.Panic(err.Error())
			}

			err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Public")
			if err != nil {
				rollback(ctx, s3Client, createdBuckets)
				log.Panic(err.Error())
			}

		} else {
			err := helpers.EnableLifecycle(ctx, s3Client, fullBucketName)
			if err != nil {
				rollback(ctx, s3Client, createdBuckets)
				log.Panic(err.Error())
			}

		}
		err = helpers.EnableEventBridge(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		err = helpers.EnableInventory(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		var replicationBucketName = fmt.Sprintf("%s%s", fullBucketName, helpers.ReplicationSuffix)
		err = helpers.CreateNewBucket(ctx, s3Client, replicationBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, bucketPrefix, "Replication")
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		err = helpers.RemovePolicy(ctx, s3Client, fullBucketName)
		if err != nil {
			rollback(ctx, s3Client, createdBuckets)
			log.Panic(err.Error())
		}

		log.Printf("Finished Creating bucket %v", fullBucketName)
	}

	return nil
}

func rollback(ctx context.Context, s3Client *s3.Client, buckets []string) {
	for _, createdBucket := range buckets {
		err := helpers.DeleteBucket(ctx, s3Client, createdBucket)
		if err != nil {
			log.Fatalf("Error rolling back previous error. Quiting!")
		}
	}
}

func main() {
	lambda.Start(handler)
}
