package helpers

import (
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// S3EventWrapper wraps an S3Event
type S3EventWrapper struct {
	Event *events.S3Event
}

// BucketName extracts the bucket name
func (w *S3EventWrapper) BucketName() string {
	return w.firstRecord().S3.Bucket.Name
}

// ObjectKey extracts the object key
func (w *S3EventWrapper) ObjectKey() string {
	return w.firstRecord().S3.Object.Key
}

// IsObjectCreatedEvent checks if the event is an object creation event
func (w *S3EventWrapper) IsObjectCreatedEvent() bool {
	return strings.HasPrefix(w.firstRecord().EventName, "ObjectCreated:")
}

// IsObjectDeletedEvent checks if the event is an object deletion event
func (w *S3EventWrapper) IsObjectDeletedEvent() bool {
	return strings.HasPrefix(w.firstRecord().EventName, "ObjectRemoved:")
}

// IsRestrictedBucket checks if the bucket is a restricted type
func (w *S3EventWrapper) IsRestrictedBucket() bool {
	return IsRestrictedBucket(w.BucketName())
}

// firstRecord returns the first record in the event
func (w *S3EventWrapper) firstRecord() *events.S3EventRecord {
	return &w.Event.Records[0]
}
