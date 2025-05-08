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
	if len(w.Event.Records) > 0 {
		return w.Event.Records[0].S3.Bucket.Name
	}
	return ""
}

// ObjectKey extracts the object key
func (w *S3EventWrapper) ObjectKey() string {
	if len(w.Event.Records) > 0 {
		return w.Event.Records[0].S3.Object.Key
	}
	return ""
}

// IsObjectCreatedEvent checks if the event is an object creation event
func (w *S3EventWrapper) IsObjectCreatedEvent() bool {
	if len(w.Event.Records) > 0 {
		return strings.HasPrefix(w.Event.Records[0].EventName, "ObjectCreated:")
	}
	return false
}

// IsObjectDeletedEvent checks if the event is an object deletion event
func (w *S3EventWrapper) IsObjectDeletedEvent() bool {
	if len(w.Event.Records) > 0 {
		return strings.HasPrefix(w.Event.Records[0].EventName, "ObjectRemoved:")
	}
	return false
}

// IsRestrictedBucket checks if the bucket is a restricted type
func (w *S3EventWrapper) IsRestrictedBucket() bool {
	return IsRestrictedBucket(w.BucketName())
}
