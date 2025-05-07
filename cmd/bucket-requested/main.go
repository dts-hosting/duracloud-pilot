package main

import (
	"context"
	"encoding/json"
	"log"

	"duracloud/internal/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

var awsConfig aws.Config

func init() {
	var err error
	awsConfig, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}
}

func handler(ctx context.Context, event json.RawMessage) error {
	var s3Event events.S3Event
	if err := json.Unmarshal(event, &s3Event); err != nil {
		log.Printf("Failed to parse event: %v", err)
		return err
	}

	if s3Event.IsObjectDeletedEvent() || events.IsRestrictedBucket(&s3Event) {
		return nil
	}

	bucketName := s3Event.BucketName()
	objectKey := s3Event.ObjectKey()
	log.Printf("Received event for bucket name: %s, object key: %s", bucketName, objectKey)

	// download file
	// read lines (for each line)
	// create bucket (using stack prefix?) with configuration we need
	// may need to wait for it for some parts?
	// upload log to managed bucket

	return nil
}

func main() {
	lambda.Start(handler)
}
