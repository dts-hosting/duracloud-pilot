package main

import (
	"context"
	"duracloud/internal/helpers"
	"encoding/json"
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
	//awsConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
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
		log.Printf("Failed to parse event: %v", err)
		return err
	}

	e := helpers.S3EventWrapper{
		Event: &s3Event,
	}

	bucketName := e.BucketName()
	objectKey := e.ObjectKey()
	log.Printf("Received event for bucket name: %s, object key: %s", bucketName, objectKey)

	buckets := getBuckets(ctx, bucketName, objectKey)
	log.Printf("Retrieved %d buckets list from request file", len(buckets))

	// Do all the things ...

	return nil
}

func main() {
	lambda.Start(handler)
}
