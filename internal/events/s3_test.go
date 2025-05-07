package events

import (
	"encoding/json"
	"testing"
)

const sampleS3Event = `{
  "Records": [
    {
      "eventName": "ObjectCreated:Put",
      "s3": {
        "bucket": {
          "name": "example-bucket"
        },
        "object": {
          "key": "folder/example-file.txt"
        }
      }
    }
  ]
}`

func TestS3EventExtractors(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		objectKey  string
		eventName  string
		isManaged  bool
		isPublic   bool
		isRequest  bool
		isCreated  bool
		isDeleted  bool
	}{
		{
			name:       "Bucket Request Object",
			bucketName: "example-bucket-requested",
			objectKey:  "folder/example-file.txt",
			eventName:  "ObjectCreated:Put",
			isManaged:  false,
			isPublic:   false,
			isRequest:  true,
			isCreated:  true,
			isDeleted:  false,
		},
		{
			name:       "Managed Bucket Object",
			bucketName: "example-managed",
			objectKey:  "folder/example-file.txt",
			eventName:  "ObjectCreated:Put",
			isManaged:  true,
			isPublic:   false,
			isRequest:  false,
			isCreated:  true,
			isDeleted:  false,
		},
		{
			name:       "Public Bucket Object",
			bucketName: "example-public",
			objectKey:  "folder/example-file.txt",
			eventName:  "ObjectRemoved:Delete",
			isManaged:  false,
			isPublic:   true,
			isRequest:  false,
			isCreated:  false,
			isDeleted:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &S3Event{
				Records: []S3EventRecord{
					{
						EventName: tt.eventName,
						S3: struct {
							Bucket struct {
								Name string `json:"name"`
							} `json:"bucket"`
							Object struct {
								Key string `json:"key"`
							} `json:"object"`
						}{
							Bucket: struct {
								Name string `json:"name"`
							}{
								Name: tt.bucketName,
							},
							Object: struct {
								Key string `json:"key"`
							}{
								Key: tt.objectKey,
							},
						},
					},
				},
			}

			if got := event.BucketName(); got != tt.bucketName {
				t.Errorf("BucketName() = %v, want %v", got, tt.bucketName)
			}

			if got := event.ObjectKey(); got != tt.objectKey {
				t.Errorf("ObjectKey() = %v, want %v", got, tt.objectKey)
			}

			if got := IsCreateRequestBucket(event); got != tt.isRequest {
				t.Errorf("IsCreateRequestBucket() = %v, want %v", got, tt.isRequest)
			}

			if got := IsManagedBucket(event); got != tt.isManaged {
				t.Errorf("IsManagedBucket() = %v, want %v", got, tt.isManaged)
			}

			if got := IsPublicBucket(event); got != tt.isPublic {
				t.Errorf("IsPublicBucket() = %v, want %v", got, tt.isPublic)
			}

			if got := event.IsObjectCreatedEvent(); got != tt.isCreated {
				t.Errorf("IsObjectCreatedEvent() = %v, want %v", got, tt.isCreated)
			}

			if got := event.IsObjectDeletedEvent(); got != tt.isDeleted {
				t.Errorf("IsObjectDeletedEvent() = %v, want %v", got, tt.isDeleted)
			}
		})
	}
}

func TestS3EventUnmarshal(t *testing.T) {
	var s3Event S3Event
	err := json.Unmarshal([]byte(sampleS3Event), &s3Event)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	bucketName := s3Event.BucketName()
	expectedBucketName := "example-bucket"
	if bucketName != expectedBucketName {
		t.Errorf("Expected bucket name %q, but got %q", expectedBucketName, bucketName)
	}

	objectKey := s3Event.ObjectKey()
	expectedObjectKey := "folder/example-file.txt"
	if objectKey != expectedObjectKey {
		t.Errorf("Expected object key %q, but got %q", expectedObjectKey, objectKey)
	}
}

func TestS3EventTypeIdentification(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		isCreated bool
		isDeleted bool
	}{
		{
			name:      "ObjectCreated:Put",
			eventName: "ObjectCreated:Put",
			isCreated: true,
			isDeleted: false,
		},
		{
			name:      "ObjectCreated:Post",
			eventName: "ObjectCreated:Post",
			isCreated: true,
			isDeleted: false,
		},
		{
			name:      "ObjectRemoved:Delete",
			eventName: "ObjectRemoved:Delete",
			isCreated: false,
			isDeleted: true,
		},
		{
			name:      "ObjectRemoved:DeleteMarkerCreated",
			eventName: "ObjectRemoved:DeleteMarkerCreated",
			isCreated: false,
			isDeleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &S3Event{
				Records: []S3EventRecord{
					{
						EventName: tt.eventName,
					},
				},
			}

			if got := event.IsObjectCreatedEvent(); got != tt.isCreated {
				t.Errorf("IsObjectCreatedEvent() = %v, want %v", got, tt.isCreated)
			}

			if got := event.IsObjectDeletedEvent(); got != tt.isDeleted {
				t.Errorf("IsObjectDeletedEvent() = %v, want %v", got, tt.isDeleted)
			}
		})
	}
}

func TestEmptyRecordsArray(t *testing.T) {
	event := &S3Event{
		Records: []S3EventRecord{},
	}

	if got := event.BucketName(); got != "" {
		t.Errorf("Expected empty bucket name, got %q", got)
	}

	if got := event.ObjectKey(); got != "" {
		t.Errorf("Expected empty object key, got %q", got)
	}

	if event.IsObjectCreatedEvent() {
		t.Errorf("Empty event should not be an ObjectCreated event")
	}

	if event.IsObjectDeletedEvent() {
		t.Errorf("Empty event should not be an ObjectDeleted event")
	}
}
