package checksum

import (
	"context"
	"crypto/md5"
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
	BufferSize  = 2 * 1024 * 1024
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
	bufferSize int
}

// NewS3Calculator creates a new S3 streaming calculator
func NewS3Calculator(s3Client S3ClientInterface) *S3Calculator {
	return &S3Calculator{
		s3Client:   s3Client,
		hasherFunc: md5.New,
		bufferSize: BufferSize,
	}
}

// NewS3CalculatorWithHasher creates a calculator with custom hash function
func NewS3CalculatorWithHasher(s3Client S3ClientInterface, hasherFunc func() hash.Hash) *S3Calculator {
	return &S3Calculator{
		s3Client:   s3Client,
		hasherFunc: hasherFunc,
		bufferSize: BufferSize,
	}
}

// S3Object represents an S3 object for checksum calculation
type S3Object struct {
	Bucket string
	Key    string
}

// NewS3Object creates a new S3Object
func NewS3Object(bucket, key string) S3Object {
	return S3Object{Bucket: bucket, Key: key}
}

// URI returns a human-readable URI for the S3 object
func (obj S3Object) URI() string {
	return fmt.Sprintf("s3://%s/%s", obj.Bucket, obj.Key)
}

// CalculateChecksum streams an object from S3 and calculates its checksum
func (c *S3Calculator) CalculateChecksum(ctx context.Context, obj S3Object) (string, error) {
	headResp, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return "", fmt.Errorf("object not found: %s", obj.URI())
		}
		return "", fmt.Errorf("failed to get object metadata for %s: %w", obj.URI(), err)
	}

	var fileSize int64
	if headResp.ContentLength != nil {
		fileSize = *headResp.ContentLength
	}

	if fileSize > MaxFileSize {
		return "", fmt.Errorf("file too large: %s is %d bytes (%.2f GB), maximum allowed is %d bytes (%.2f GB)",
			obj.URI(), fileSize, float64(fileSize)/(1024*1024*1024),
			MaxFileSize, float64(MaxFileSize)/(1024*1024*1024))
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
			return "", fmt.Errorf("object not found: %s", obj.URI())
		}
		return "", fmt.Errorf("failed to get object content for %s: %w", obj.URI(), err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Failed to close body for %s: %v", obj.URI(), err)
		}
	}(getResp.Body)

	return c.streamAndHash(getResp.Body, obj.URI(), fileSize)
}

// streamAndHash streams content and calculates hash
func (c *S3Calculator) streamAndHash(reader io.Reader, uri string, expectedSize int64) (string, error) {
	hashWriter := c.hasherFunc()

	totalBytes, err := io.Copy(hashWriter, reader)
	if err != nil {
		return "", fmt.Errorf("failed to read from stream for %s: %w", uri, err)
	}

	// Verify we read the expected amount
	if totalBytes != expectedSize {
		return "", fmt.Errorf("size mismatch for %s: expected %d bytes, read %d bytes",
			uri, expectedSize, totalBytes)
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
