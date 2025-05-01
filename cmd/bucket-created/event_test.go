package main

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
		name          string
		bucketName    string
		isEventLogs   bool
		isManaged     bool
		isPublic      bool
		isReplication bool
		isRestricted  bool
	}{
		{
			name:          "Event Logs Bucket",
			bucketName:    "example-event-logs",
			isEventLogs:   true,
			isManaged:     false,
			isPublic:      false,
			isReplication: false,
			isRestricted:  true,
		},
		{
			name:          "Managed Bucket",
			bucketName:    "example-managed",
			isEventLogs:   false,
			isManaged:     true,
			isPublic:      false,
			isReplication: false,
			isRestricted:  true,
		},
		{
			name:          "Public Bucket",
			bucketName:    "example-public",
			isEventLogs:   false,
			isManaged:     false,
			isPublic:      true,
			isReplication: false,
			isRestricted:  false,
		},
		{
			name:          "Replication Bucket",
			bucketName:    "example-replication",
			isEventLogs:   false,
			isManaged:     false,
			isPublic:      false,
			isReplication: true,
			isRestricted:  true,
		},
		{
			name:          "Unrestricted Bucket",
			bucketName:    "example-unrestricted",
			isEventLogs:   false,
			isManaged:     false,
			isPublic:      false,
			isReplication: false,
			isRestricted:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := BucketCreatedEvent{
				Detail: Detail{
					RequestParameters: RequestParameters{
						BucketName: tt.bucketName,
					},
				},
			}

			if got := event.IsEventLogsBucket(); got != tt.isEventLogs {
				t.Errorf("IsEventLogsBucket() = %v, want %v", got, tt.isEventLogs)
			}
			if got := event.IsManagedBucket(); got != tt.isManaged {
				t.Errorf("IsManagedBucket() = %v, want %v", got, tt.isManaged)
			}
			if got := event.IsPublicBucket(); got != tt.isPublic {
				t.Errorf("IsPublicBucket() = %v, want %v", got, tt.isPublic)
			}
			if got := event.IsReplicationBucket(); got != tt.isReplication {
				t.Errorf("IsReplicationBucket() = %v, want %v", got, tt.isReplication)
			}
			if got := event.IsRestrictedBucket(); got != tt.isRestricted {
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
