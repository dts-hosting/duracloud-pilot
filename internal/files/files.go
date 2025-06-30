package files

import "fmt"

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
