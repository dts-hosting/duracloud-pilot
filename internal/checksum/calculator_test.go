package checksum

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"duracloud/internal/files"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// Mock S3 client for testing
type mockS3Client struct {
	objects map[string][]byte // key format: "bucket/key"
	errors  map[string]error  // errors to return for specific operations
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{
		objects: make(map[string][]byte),
		errors:  make(map[string]error),
	}
}

func (m *mockS3Client) addObject(bucket, key string, content []byte) {
	m.objects[bucket+"/"+key] = content
}

func (m *mockS3Client) addError(bucket, key, operation string, err error) {
	m.errors[operation+":"+bucket+"/"+key] = err
}

func (m *mockS3Client) HeadObject(ctx context.Context, input *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	key := *input.Bucket + "/" + *input.Key

	if err, exists := m.errors["head:"+key]; exists {
		return nil, err
	}

	content, exists := m.objects[key]
	if !exists {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey", Message: "Key not found"}
	}

	contentLength := int64(len(content))
	return &s3.HeadObjectOutput{
		ContentLength: &contentLength,
	}, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := *input.Bucket + "/" + *input.Key

	if err, exists := m.errors["get:"+key]; exists {
		return nil, err
	}

	content, exists := m.objects[key]
	if !exists {
		return nil, &smithy.GenericAPIError{Code: "NoSuchKey", Message: "Key not found"}
	}

	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(content)),
	}, nil
}

// Helper function to calculate MD5 for test data
func calculateMD5(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

func TestS3Calculator_CalculateChecksum(t *testing.T) {
	tests := []struct {
		name           string
		bucket         string
		key            string
		content        []byte
		shouldError    bool
		errorSubstring string
	}{
		{
			name:    "empty file",
			bucket:  "test-bucket",
			key:     "empty.txt",
			content: []byte{},
		},
		{
			name:    "small text file",
			bucket:  "test-bucket",
			key:     "hello.txt",
			content: []byte("hello world"),
		},
		{
			name:    "medium file",
			bucket:  "test-bucket",
			key:     "medium.dat",
			content: bytes.Repeat([]byte("A"), 1024*1024), // 1MB of 'A's
		},
		{
			name:    "binary data",
			bucket:  "test-bucket",
			key:     "binary.bin",
			content: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedMD5 := calculateMD5(tt.content)

			mockClient := newMockS3Client()
			mockClient.addObject(tt.bucket, tt.key, tt.content)

			calc := NewS3Calculator(mockClient)
			obj := files.NewS3Object(tt.bucket, tt.key)

			result, err := calc.CalculateChecksum(context.Background(), obj)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorSubstring)
				} else if !strings.Contains(err.Error(), tt.errorSubstring) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != expectedMD5 {
				t.Errorf("MD5 mismatch: expected %s, got %s", expectedMD5, result)
			}

			calcSHA := NewS3CalculatorWithHasher(mockClient, sha256.New)
			resultSHA, err := calcSHA.CalculateChecksum(context.Background(), obj)
			if err != nil {
				t.Fatalf("unexpected error with SHA256: %v", err)
			}

			expectedSHA256 := fmt.Sprintf("%x", sha256.Sum256(tt.content))
			if resultSHA != expectedSHA256 {
				t.Errorf("SHA256 mismatch: expected %s, got %s", expectedSHA256, resultSHA)
			}
		})
	}
}

func TestS3Calculator_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		bucket         string
		key            string
		setupError     func(*mockS3Client)
		errorSubstring string
	}{
		{
			name:   "object not found - head",
			bucket: "test-bucket",
			key:    "nonexistent.txt",
			setupError: func(m *mockS3Client) {
				// Don't add the object, so it won't be found
			},
			errorSubstring: "object not found",
		},
		{
			name:   "object not found - get",
			bucket: "test-bucket",
			key:    "test.txt",
			setupError: func(m *mockS3Client) {
				// Add object for head but make get fail
				m.addObject("test-bucket", "test.txt", []byte("test"))
				m.addError("test-bucket", "test.txt", "get", &smithy.GenericAPIError{Code: "NoSuchKey"})
			},
			errorSubstring: "object not found",
		},
		{
			name:   "bucket not found",
			bucket: "nonexistent-bucket",
			key:    "test.txt",
			setupError: func(m *mockS3Client) {
				m.addError("nonexistent-bucket", "test.txt", "head", &smithy.GenericAPIError{Code: "NoSuchBucket"})
			},
			errorSubstring: "object not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockS3Client()
			tt.setupError(mockClient)

			calc := NewS3Calculator(mockClient)
			obj := files.NewS3Object(tt.bucket, tt.key)

			_, err := calc.CalculateChecksum(context.Background(), obj)

			if err == nil {
				t.Fatalf("expected error containing '%s', got nil", tt.errorSubstring)
			}

			if !strings.Contains(err.Error(), tt.errorSubstring) {
				t.Errorf("expected error containing '%s', got '%s'", tt.errorSubstring, err.Error())
			}
		})
	}
}

func TestS3Calculator_LargeFile(t *testing.T) {
	mockClient := newMockS3Client()

	// Create a file larger than buffer size to test streaming
	content := bytes.Repeat([]byte("DuraCloud test data for large file streaming. "), 50000) // ~2MB
	mockClient.addObject("test-bucket", "large.txt", content)

	calc := NewS3Calculator(mockClient)
	obj := files.NewS3Object("test-bucket", "large.txt")

	result, err := calc.CalculateChecksum(context.Background(), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := calculateMD5(content)
	if result != expected {
		t.Errorf("large file checksum mismatch: expected %s, got %s", expected, result)
	}
}

func TestS3Calculator_GetAdaptiveBufferSize(t *testing.T) {
	tests := []struct {
		name         string
		fileSize     int64
		expectedSize int
	}{
		{
			name:         "small file (500KB)",
			fileSize:     500 * 1024,
			expectedSize: 64 * 1024, // 64KB
		},
		{
			name:         "exactly 1MB",
			fileSize:     1024 * 1024,
			expectedSize: 512 * 1024, // 512KB
		},
		{
			name:         "medium file (50MB)",
			fileSize:     50 * 1024 * 1024,
			expectedSize: 512 * 1024, // 512KB
		},
		{
			name:         "exactly 100MB",
			fileSize:     100 * 1024 * 1024,
			expectedSize: 2 * 1024 * 1024, // 2MB
		},
		{
			name:         "large file (1GB)",
			fileSize:     1024 * 1024 * 1024,
			expectedSize: 2 * 1024 * 1024, // 2MB
		},
		{
			name:         "empty file",
			fileSize:     0,
			expectedSize: 64 * 1024, // 64KB
		},
	}

	mockClient := newMockS3Client()
	calc := NewS3Calculator(mockClient)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.getAdaptiveBufferSize(tt.fileSize)
			if result != tt.expectedSize {
				t.Errorf("buffer size mismatch for %d bytes: expected %d, got %d",
					tt.fileSize, tt.expectedSize, result)
			}
		})
	}
}

func TestS3Object_URI(t *testing.T) {
	obj := files.NewS3Object("my-bucket", "path/to/file.txt")
	expected := "s3://my-bucket/path/to/file.txt"

	if obj.URI() != expected {
		t.Errorf("URI mismatch: expected %s, got %s", expected, obj.URI())
	}
}
