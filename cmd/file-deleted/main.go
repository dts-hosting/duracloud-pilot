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
)

var awsConfig aws.Config

func init() {
	var err error
	awsConfig, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}
}

func handler(ctx context.Context, event json.RawMessage) (events.SQSEventResponse, error) {
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err != nil {
		log.Printf("Failed to parse SQS event: %v", err)
		return events.SQSEventResponse{}, err
	}

	sqsEventWrapper := helpers.SQSEventWrapper{
		Event: &sqsEvent,
	}

	records, failedRecords := sqsEventWrapper.UnwrapS3EventBridgeEvents()

	for _, record := range records {
		if !record.IsObjectDeletedEvent() || record.IsRestrictedBucket() {
			continue
		}

		bucketName := record.BucketName()
		objectKey := record.ObjectKey()
		log.Printf("Processing event for bucket name: %s, object key: %s", bucketName, objectKey)
	}

	return events.SQSEventResponse{
		BatchItemFailures: failedRecords,
	}, nil
}

func main() {
	lambda.Start(handler)
}
