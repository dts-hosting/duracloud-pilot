package checksum

import (
	"context"
	"crypto/md5"
	"duracloud/internal/files"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"hash"
	"io"
	"log"
)

const (
	MaxFileSize = 20 * 1024 * 1024 * 1024 // 20GB maximum file size (Lambda consideration)
)

// S3ClientInterface defines the S3 operations required for checksum verification
type S3ClientInterface interface {
	HeadObject(ctx context.Context, input *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// S3Calculator handles checksum calculation by streaming directly from S3
type S3Calculator struct {
	s3Client   S3ClientInterface
	hasherFunc func() hash.Hash
}

// NewS3Calculator creates a new S3 streaming calculator
func NewS3Calculator(s3Client S3ClientInterface) *S3Calculator {
	return &S3Calculator{
		s3Client:   s3Client,
		hasherFunc: md5.New,
	}
}

// NewS3CalculatorWithHasher creates a calculator with custom hash function
func NewS3CalculatorWithHasher(s3Client S3ClientInterface, hasherFunc func() hash.Hash) *S3Calculator {
	return &S3Calculator{
		s3Client:   s3Client,
		hasherFunc: hasherFunc,
	}
}

// CalculateChecksum streams an object from S3 and calculates its checksum
func (c *S3Calculator) CalculateChecksum(ctx context.Context, obj files.S3Object) (string, error) {
	headResp, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return "", ErrorObjectNotFound(obj.URI())
		}
		return "", ErrorMetadataNotRetrieved(obj.URI(), err)
	}

	var fileSize int64
	if headResp.ContentLength != nil {
		fileSize = *headResp.ContentLength
	}

	if fileSize > MaxFileSize {
		return "", ErrorMaxFileSizeExceeded(obj.URI(), fileSize)
	}

	log.Printf("Starting checksum calculation for %s - Size: %d bytes (%.2f MB)",
		obj.URI(), fileSize, float64(fileSize)/(1024*1024))

	// Get the object content
	getResp, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return "", ErrorObjectNotFound(obj.URI())
		}
		return "", ErrorObjectNotRetrieved(obj.URI(), err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Failed to close body for %s: %v", obj.URI(), err)
		}
	}(getResp.Body)

	return c.streamAndHash(getResp.Body, obj.URI(), fileSize)
}

// getAdaptiveBufferSize returns a buffer size based on file size
func (c *S3Calculator) getAdaptiveBufferSize(fileSize int64) int {
	switch {
	case fileSize < 1024*1024: // < 1MB
		return 64 * 1024 // 64KB
	case fileSize < 100*1024*1024: // < 100MB
		return 512 * 1024 // 512KB
	default: // >= 100MB
		return 2 * 1024 * 1024 // 2MB
	}
}

// streamAndHash streams content and calculates hash using adaptive buffer size
func (c *S3Calculator) streamAndHash(reader io.Reader, uri string, expectedSize int64) (string, error) {
	hashWriter := c.hasherFunc()

	bufferSize := c.getAdaptiveBufferSize(expectedSize)
	buffer := make([]byte, bufferSize)

	log.Printf("Using %d KB buffer for %s (%.2f MB file)",
		bufferSize/1024, uri, float64(expectedSize)/(1024*1024))

	totalBytes, err := io.CopyBuffer(hashWriter, reader, buffer)
	if err != nil {
		return "", ErrorReadingFromStream(uri, err)
	}

	// Verify we read the expected amount
	if totalBytes != expectedSize {
		return "", ErrorBytesCountDoesNotMatch(uri, expectedSize, totalBytes)
	}

	checksum := fmt.Sprintf("%x", hashWriter.Sum(nil))
	log.Printf("Successfully calculated checksum for %s: %d bytes, checksum: %s",
		uri, totalBytes, checksum)

	return checksum, nil
}

// isS3NotFound checks if an error is a "not found" error from S3
func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NoSuchBucket"
	}
	return false
}
