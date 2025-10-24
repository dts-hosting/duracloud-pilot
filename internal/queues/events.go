package queues

import (
	"duracloud/internal/buckets"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
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
			Etag string `json:"etag"`
			Key  string `json:"key"`
		} `json:"object"`
	} `json:"detail"`
}

type S3EventBridgeEventWithMessageId struct {
	MessageId string
	S3EventBridgeEvent
}

// SQSEventWrapper wraps an SQSEvent
type SQSEventWrapper struct {
	Event *events.SQSEvent
}

// BucketName extracts the bucket name
func (e *S3EventBridgeEvent) BucketName() string {
	return e.Detail.Bucket.Name
}

// BucketPrefix returns the first two segments of the bucket name, rejoined
func (w *S3EventBridgeEvent) BucketPrefix() string {
	return buckets.GetBucketPrefix(w.BucketName())
}

// Etag extracts the object etag
func (e *S3EventBridgeEvent) Etag() string {
	return e.Detail.Object.Etag
}

// ObjectKey extracts the object key
func (e *S3EventBridgeEvent) ObjectKey() string {
	return e.Detail.Object.Key
}

// IsIgnoreFilesBucket checks if the bucket contains ignorable files
func (e *S3EventBridgeEvent) IsIgnoreFilesBucket() bool {
	return buckets.IsIgnoreFilesBucket(e.BucketName())
}

// IsObjectCreated checks if the event is an object creation event
func (e *S3EventBridgeEvent) IsObjectCreated() bool {
	return e.DetailType == "Object Created"
}

// IsObjectDeleted checks if the event is an object deletion event
func (e *S3EventBridgeEvent) IsObjectDeleted() bool {
	return e.DetailType == "Object Deleted"
}

// IsRestrictedBucket checks if the bucket is a restricted type
func (e *S3EventBridgeEvent) IsRestrictedBucket() bool {
	return buckets.IsRestrictedBucket(e.BucketName())
}

// IsPrefix checks if the object key represents a prefix (directory)
func (e *S3EventBridgeEvent) IsPrefix() bool {
	key := e.ObjectKey()
	return len(key) > 0 && key[len(key)-1] == '/'
}

// UnwrapS3EventBridgeEvents extracts all S3 events from the SQS message
func (w *SQSEventWrapper) UnwrapS3EventBridgeEvents() ([]S3EventBridgeEventWithMessageId, []events.SQSBatchItemFailure) {
	var parsedEvents []S3EventBridgeEventWithMessageId
	var failedEvents []events.SQSBatchItemFailure

	for _, record := range w.Event.Records {
		var event S3EventBridgeEvent
		if err := json.Unmarshal([]byte(record.Body), &event); err != nil {
			log.Printf("Failed to parse EventBridge event from SQS message: %v", err)
			failedEvents = append(failedEvents, events.SQSBatchItemFailure{
				ItemIdentifier: record.MessageId,
			})
			continue
		}

		if event.Source == "aws.s3" && (event.IsObjectCreated() || event.IsObjectDeleted()) {
			parsedEvents = append(parsedEvents, S3EventBridgeEventWithMessageId{record.MessageId, event})
		}
	}

	return parsedEvents, failedEvents
}
