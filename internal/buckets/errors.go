package buckets

import (
	"errors"
	"fmt"
)

var (
	ErrApplyingBucketPolicy         = errors.New("failed to apply bucket policy")
	ErrApplyingBucketTags           = errors.New("failed to add bucket tags")
	ErrApplyingEventBridge          = errors.New("failed to enable EventBridge notifications")
	ErrApplyingExpiration           = errors.New("failed to set lifecycle rule")
	ErrApplyingInventory            = errors.New("failed to enable inventory configuration")
	ErrApplyingLifecycle            = errors.New("failed to configure lifecycle")
	ErrApplyingLogging              = errors.New("failed to enable access logging")
	ErrApplyingPublicAccessBlock    = errors.New("failed to disable public access block")
	ErrApplyingReplication          = errors.New("failed to enable replication configuration")
	ErrApplyingVersioning           = errors.New("failed to enable versioning")
	ErrAWSContextRetrieval          = errors.New("error retrieving aws context")
	ErrBucketCreationFailed         = errors.New("failed to create bucket")
	ErrBucketDeletionFailed         = errors.New("failed to delete bucket")
	ErrBucketStatusUploadFailed     = errors.New("failed to write bucket status")
	ErrDeletingBucketPolicy         = errors.New("failed to delete bucket policy")
	ErrExceededMaxBucketsPerRequest = errors.New("exceeded maximum allowed buckets per request")
	ErrInvalidBucketName            = errors.New("invalid bucket name requested")
	ErrMarshallingBucketPolicy      = errors.New("failed to marshal bucket policy")
	ErrMarshallingPolicy            = errors.New("failed to marshal policy")
	ErrReadingMaxBucketsPerRequest  = errors.New("unable to read max buckets per request variable")
	ErrReadingResponse              = errors.New("error reading response")
	ErrRetrievingObject             = errors.New("failed to get object")
)

func ErrorApplyingBucketPolicy(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingBucketPolicy, cause)
}

func ErrorApplyingBucketTags(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingBucketTags, cause)
}

func ErrorApplyingEventBridge(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingEventBridge, cause)
}

func ErrorApplyingExpiration(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingExpiration, cause)
}

func ErrorApplyingInventory(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingInventory, cause)
}

func ErrorApplyingLifecycle(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingLifecycle, cause)
}

func ErrorApplyingLogging(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingLogging, cause)
}

func ErrorApplyingPublicAccessBlock(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingPublicAccessBlock, cause)
}

func ErrorApplyingReplication(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingReplication, cause)
}

func ErrorApplyingVersioning(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrApplyingVersioning, cause)
}

func ErrorAWSContextRetrieval() error {
	return ErrAWSContextRetrieval
}

func ErrorBucketCreationFailed(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketCreationFailed, cause)
}

func ErrorBucketDeletionFailed(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketDeletionFailed, cause)
}

func ErrorBucketStatusUploadFailed(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketStatusUploadFailed, cause)
}

func ErrorDeletingBucketPolicy(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrDeletingBucketPolicy, cause)
}

func ErrorExceededMaxBucketsPerRequest(limit, requested int) error {
	return fmt.Errorf("%w: limit=%d requested=%d", ErrExceededMaxBucketsPerRequest, limit, requested)
}

func ErrorInvalidBucketName(bucketName string) error {
	return fmt.Errorf("%w: bucket=%s", ErrInvalidBucketName, bucketName)
}

func ErrorMarshallingBucketPolicy(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrMarshallingBucketPolicy, cause)
}

func ErrorMarshallingPolicy(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrMarshallingPolicy, cause)
}

func ErrorReadingMaxBucketsPerRequest(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrReadingMaxBucketsPerRequest, cause)
}

func ErrorReadingResponse(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrReadingResponse, cause)
}

func ErrorRetrievingObject(key, bucket string, cause error) error {
	return fmt.Errorf("%w: key=%s bucket=%s cause=%v", ErrRetrievingObject, key, bucket, cause)
}
