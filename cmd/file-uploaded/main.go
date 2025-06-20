package main

import (
	"context"
	"duracloud/internal/queues"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

func handler(ctx context.Context, event json.RawMessage) (events.SQSEventResponse, error) {
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err != nil {
		return events.SQSEventResponse{}, fmt.Errorf("failed to parse SQS event: %v", err)
	}

	sqsEventWrapper := queues.SQSEventWrapper{
		Event: &sqsEvent,
	}

	parsedEvents, failedEvents := sqsEventWrapper.UnwrapS3EventBridgeEvents()

	for _, parsedEvent := range parsedEvents {
		if !parsedEvent.IsObjectCreated() || parsedEvent.IsRestrictedBucket() {
			continue
		}

		bucketName := parsedEvent.BucketName()
		objectKey := parsedEvent.ObjectKey()
		log.Printf("Processing upload event for bucket name: %s, object key: %s", bucketName, objectKey)

		if err := processUploadedObject(ctx, bucketName, objectKey); err != nil {
			// only use for retryable errors
			log.Printf("Failed to process uploaded object %s/%s: %v", bucketName, objectKey, err)
			failedEvents = append(failedEvents, events.SQSBatchItemFailure{
				ItemIdentifier: parsedEvent.MessageId,
			})
		}
	}

	return events.SQSEventResponse{
		BatchItemFailures: failedEvents,
	}, nil
}

func main() {
	lambda.Start(handler)
}

func processUploadedObject(ctx context.Context, bucketName string, objectKey string) error {
	// TODO: continue implementation ...
	// - Calc checksum
	// - Not ok: LastChecksumDate & LastChecksumSuccess (f) & Msg, PutChecksumRecord
	// - ok: LastChecksumDate & Msg ("ok"), PutChecksumRecord, Schedule next check
	return nil
}
