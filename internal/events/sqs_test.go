package events

import (
	"encoding/json"
	"testing"
)

const sampleSQSEvent = `{
    "detail-type": "Object Created",
    "source": "aws.s3",
    "detail": {
        "bucket": {
            "name": "example-bucket"
        },
        "object": {
            "key": "folder/example-file.txt",
            "size": 1024,
            "etag": "a1b2c3d4e5f6"
        },
        "reason": "PutObject"
    }
}`

func TestSQSEventExtractors(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		objectKey  string
		isManaged  bool
		isPublic   bool
	}{
		{
			name:       "Managed Bucket Object",
			bucketName: "example-managed",
			objectKey:  "folder/example-file.txt",
			isManaged:  true,
			isPublic:   false,
		},
		{
			name:       "Public Bucket Object",
			bucketName: "example-public",
			objectKey:  "folder/example-file.txt",
			isManaged:  false,
			isPublic:   true,
		},
		{
			name:       "Regular Bucket Object",
			bucketName: "example-regular",
			objectKey:  "folder/example-file.txt",
			isManaged:  false,
			isPublic:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &SQSEvent{
				DetailType: "Object Created",
				Source:     "aws.s3",
			}
			event.Detail.Bucket.Name = tt.bucketName
			event.Detail.Object.Key = tt.objectKey
			event.Detail.Object.Size = 1024
			event.Detail.Object.ETag = "a1b2c3d4e5f6"
			event.Detail.Reason = "PutObject"

			if got := event.BucketName(); got != tt.bucketName {
				t.Errorf("BucketName() = %v, want %v", got, tt.bucketName)
			}
			if got := event.ObjectKey(); got != tt.objectKey {
				t.Errorf("ObjectKey() = %v, want %v", got, tt.objectKey)
			}

			if got := IsManagedBucket(event); got != tt.isManaged {
				t.Errorf("IsManagedBucket() = %v, want %v", got, tt.isManaged)
			}
			if got := IsPublicBucket(event); got != tt.isPublic {
				t.Errorf("IsPublicBucket() = %v, want %v", got, tt.isPublic)
			}
		})
	}
}

func TestSQSEventUnmarshal(t *testing.T) {
	var sqsEvent SQSEvent
	err := json.Unmarshal([]byte(sampleSQSEvent), &sqsEvent)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	bucketName := sqsEvent.BucketName()
	expectedBucketName := "example-bucket"
	if bucketName != expectedBucketName {
		t.Errorf("Expected bucket name %q, but got %q", expectedBucketName, bucketName)
	}

	objectKey := sqsEvent.ObjectKey()
	expectedObjectKey := "folder/example-file.txt"
	if objectKey != expectedObjectKey {
		t.Errorf("Expected object key %q, but got %q", expectedObjectKey, objectKey)
	}
}

func TestSQSEventTypeIdentification(t *testing.T) {
	tests := []struct {
		name       string
		detailType string
		isCreated  bool
		isDeleted  bool
	}{
		{
			name:       "Object Created Event",
			detailType: "Object Created",
			isCreated:  true,
			isDeleted:  false,
		},
		{
			name:       "Object Deleted Event",
			detailType: "Object Deleted",
			isCreated:  false,
			isDeleted:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &SQSEvent{
				DetailType: tt.detailType,
				Source:     "aws.s3",
			}

			if IsObjectCreatedEvent := event.DetailType == "Object Created"; IsObjectCreatedEvent != tt.isCreated {
				t.Errorf("IsObjectCreatedEvent = %v, want %v", IsObjectCreatedEvent, tt.isCreated)
			}
			if IsObjectDeletedEvent := event.DetailType == "Object Deleted"; IsObjectDeletedEvent != tt.isDeleted {
				t.Errorf("IsObjectDeletedEvent = %v, want %v", IsObjectDeletedEvent, tt.isDeleted)
			}
		})
	}
}
