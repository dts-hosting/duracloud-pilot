package integration

import (
	"bytes"
	"context"
	"crypto/md5"
	"duracloud/internal/checksum"
	"duracloud/internal/files"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestChecksumVerification(t *testing.T) {
	t.Parallel()

	_, stackName := setupTestClients(t)
	testBucket := fmt.Sprintf("%s-managed", stackName)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	calc := checksum.NewS3Calculator(s3Client)

	tests := []struct {
		name    string
		key     string
		content []byte
	}{
		{
			name:    "small test file",
			key:     fmt.Sprintf("integration-test/small-%d.txt", time.Now().UnixNano()),
			content: []byte("Hello, DuraCloud integration test!"),
		},
		{
			name:    "medium test file",
			key:     fmt.Sprintf("integration-test/medium-%d.txt", time.Now().UnixNano()),
			content: bytes.Repeat([]byte("Integration test data. "), 50000), // ~1MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Upload test content to S3
			err = files.UploadObject(
				ctx, s3Client, files.NewS3Object(testBucket, tt.key), bytes.NewReader(tt.content),
			)
			if err != nil {
				t.Fatalf("failed to upload test object: %v", err)
			}

			// Calculate checksum
			obj := files.NewS3Object(testBucket, tt.key)
			result, err := calc.CalculateChecksum(ctx, obj)
			if err != nil {
				t.Fatalf("failed to calculate checksum: %v", err)
			}

			// Verify against local calculation
			expected := fmt.Sprintf("%x", md5.Sum(tt.content))
			if result != expected {
				t.Errorf("checksum mismatch: expected %s, got %s", expected, result)
			}

			t.Logf("Successfully verified checksum for %s: %s", obj.URI(), result)
		})
	}
}
