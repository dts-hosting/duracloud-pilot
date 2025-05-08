package main

import (
	"context"
	"duracloud/internal/helpers"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	s3Client := s3.NewFromConfig(awsConfig)
	log.Printf("Using S3 client: %v", s3Client)

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

	// 1. Download the file and read lines

	// 2. Create bucket & replication bucket with required configuration

	// 3. Upload log to managed bucket

	return nil
}

func main() {
	lambda.Start(handler)
}
