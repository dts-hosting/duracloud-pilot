package main

import (
	"context"
	"duracloud/internal/queues"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, event json.RawMessage) (events.SQSEventResponse, error) {
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err != nil {
		log.Printf("Failed to parse SQS event: %v", err)
		return events.SQSEventResponse{}, err
	}

	sqsEventWrapper := queues.SQSEventWrapper{
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
