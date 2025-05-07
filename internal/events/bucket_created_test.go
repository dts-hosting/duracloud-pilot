package events

// TODO: delete if using bucket-requested
import (
	"encoding/json"
	"testing"
)

const sampleEvent = `{
    "detail": {
        "requestParameters": {
            "bucketName": "example-bucket"
        }
    }
}`

func TestBucketCreatedIdentification(t *testing.T) {
	tests := []struct {
		name            string
		bucketName      string
		isCreateRequest bool
		isManaged       bool
		isPublic        bool
		isReplication   bool
		isRestricted    bool
	}{
		{
			name:            "Create Request Bucket",
			bucketName:      "example-bucket-requested",
			isCreateRequest: true,
			isManaged:       false,
			isPublic:        false,
			isReplication:   false,
			isRestricted:    false,
		},
		{
			name:            "Managed Bucket",
			bucketName:      "example-managed",
			isCreateRequest: false,
			isManaged:       true,
			isPublic:        false,
			isReplication:   false,
			isRestricted:    true,
		},
		{
			name:            "Public Bucket",
			bucketName:      "example-public",
			isCreateRequest: false,
			isManaged:       false,
			isPublic:        true,
			isReplication:   false,
			isRestricted:    false,
		},
		{
			name:            "Replication Bucket",
			bucketName:      "example-replication",
			isCreateRequest: false,
			isManaged:       false,
			isPublic:        false,
			isReplication:   true,
			isRestricted:    true,
		},
		{
			name:            "Unrestricted Bucket",
			bucketName:      "example-unrestricted",
			isCreateRequest: false,
			isManaged:       false,
			isPublic:        false,
			isReplication:   false,
			isRestricted:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &BucketCreatedEvent{}
			event.Detail.RequestParameters.BucketName = tt.bucketName

			if got := IsCreateRequestBucket(event); got != tt.isCreateRequest {
				t.Errorf("IsCreateRequestBucket() = %v, want %v", got, tt.isCreateRequest)
			}
			if got := IsManagedBucket(event); got != tt.isManaged {
				t.Errorf("IsManagedBucket() = %v, want %v", got, tt.isManaged)
			}
			if got := IsPublicBucket(event); got != tt.isPublic {
				t.Errorf("IsPublicBucket() = %v, want %v", got, tt.isPublic)
			}
			if got := IsReplicationBucket(event); got != tt.isReplication {
				t.Errorf("IsReplicationBucket() = %v, want %v", got, tt.isReplication)
			}
			if got := IsRestrictedBucket(event); got != tt.isRestricted {
				t.Errorf("IsRestrictedBucket() = %v, want %v", got, tt.isRestricted)
			}
		})
	}
}

func TestBucketName(t *testing.T) {
	var bucketEvent BucketCreatedEvent
	err := json.Unmarshal([]byte(sampleEvent), &bucketEvent)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	bucketName := bucketEvent.BucketName()

	expectedBucketName := "example-bucket"
	if bucketName != expectedBucketName {
		t.Errorf("Expected bucket name %q, but got %q", expectedBucketName, bucketName)
	}
}
