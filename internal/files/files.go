package files

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Object represents an S3 object
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

// DownloadObject returns a streaming reader for S3 object with optional gzip decompression
// TODO: consolidate usage and use S3Object
func DownloadObject(ctx context.Context, s3Client *s3.Client, bucket, key string, decompress bool) (io.ReadCloser, error) {
	obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	if !decompress {
		return obj.Body, nil
	}

	gzr, err := gzip.NewReader(obj.Body)
	if err != nil {
		_ = obj.Body.Close()
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	return &gzipReadCloser{Reader: gzr, underlying: obj.Body}, nil
}

// TryObject checks if an S3 object exists and can be accessed by performing a HeadObject operation.
func TryObject(ctx context.Context, s3Client *s3.Client, obj S3Object) bool {
	_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
	})
	return err == nil
}

// UploadObject with given reader for content
// TODO: consolidate usage and use S3Object
func UploadObject(ctx context.Context, s3Client *s3.Client, bucketName, key string, content io.Reader) error {
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   content,
	})
	return err
}

type gzipReadCloser struct {
	*gzip.Reader
	underlying io.ReadCloser
}

func (g *gzipReadCloser) Close() error {
	gzipErr := g.Reader.Close()
	underlyingErr := g.underlying.Close()
	if gzipErr != nil {
		return gzipErr
	}
	return underlyingErr
}
