package helpers

import (
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"log"
)

// S3EventBridgeEvent represents an SQS S3 event
type S3EventBridgeEvent struct {
	DetailType string `json:"detail-type"`
	Source     string `json:"source"`
	Detail     struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key string `json:"key"`
		} `json:"object"`
	} `json:"detail"`
}

// SQSEventWrapper wraps an SQSEvent
type SQSEventWrapper struct {
	Event *events.SQSEvent
}

// BucketName extracts the bucket name
func (w *S3EventBridgeEvent) BucketName() string {
	return w.Detail.Bucket.Name
}

// ObjectKey extracts the object key
func (w *S3EventBridgeEvent) ObjectKey() string {
	return w.Detail.Object.Key
}

// IsObjectCreatedEvent checks if the event is an object creation event
func (w *S3EventBridgeEvent) IsObjectCreatedEvent() bool {
	return w.DetailType == "Object Created"
}

// IsObjectDeletedEvent checks if the event is an object deletion event
func (w *S3EventBridgeEvent) IsObjectDeletedEvent() bool {
	return w.DetailType == "Object Deleted"
}

// IsRestrictedBucket checks if the bucket is a restricted type
func (w *S3EventBridgeEvent) IsRestrictedBucket() bool {
	return IsRestrictedBucket(w.BucketName())
}

// UnwrapS3EventBridgeEvents extracts all S3 events from the SQS message
func (w *SQSEventWrapper) UnwrapS3EventBridgeEvents() ([]S3EventBridgeEvent, []events.SQSBatchItemFailure) {
	var records []S3EventBridgeEvent
	var failedRecords []events.SQSBatchItemFailure

	for _, record := range w.Event.Records {
		var event S3EventBridgeEvent
		if err := json.Unmarshal([]byte(record.Body), &event); err != nil {
			log.Printf("Failed to parse EventBridge event from SQS message: %v", err)
			failedRecords = append(failedRecords, events.SQSBatchItemFailure{
				ItemIdentifier: record.MessageId,
			})
			continue
		}

		if event.Source == "aws.s3" {
			records = append(records, event)
		}
	}

	return records, failedRecords
}
