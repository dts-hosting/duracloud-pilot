package files

import (
	"context"
	"fmt"

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

// TryObject checks if an S3 object exists and can be accessed by performing a HeadObject operation.
func TryObject(ctx context.Context, s3Client *s3.Client, obj S3Object) bool {
	_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
	})
	return err == nil
}
