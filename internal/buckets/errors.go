package buckets

import (
	"errors"
	"fmt"
)

var (
	ErrAWSContextRetrieval            = errors.New("error retrieving aws context")
	ErrBucketCreationFailed           = errors.New("failed to create bucket")
	ErrBucketDeletionFailed           = errors.New("failed to delete bucket")
	ErrBucketPolicyApplication        = errors.New("failed to apply bucket policy")
	ErrBucketPolicyDeletion           = errors.New("failed to delete bucket policy")
	ErrBucketPolicyMarshalling        = errors.New("failed to marshal bucket policy")
	ErrBucketStatusWriteFailed        = errors.New("failed to write bucket status")
	ErrBucketTagsAddFailed            = errors.New("failed to add bucket tags")
	ErrEventBridgeEnableFailed        = errors.New("failed to enable EventBridge notifications")
	ErrExceededMaxBucketsPerRequest   = errors.New("exceeded maximum allowed buckets per request")
	ErrInventoryConfigurationFailed   = errors.New("failed to enable inventory configuration")
	ErrInvalidBucketNameRequested     = errors.New("invalid bucket name requested")
	ErrLifecycleConfigurationFailed   = errors.New("failed to configure lifecycle")
	ErrLifecycleRuleSetFailed         = errors.New("failed to set lifecycle rule")
	ErrLoggingEnableFailed            = errors.New("failed to enable access logging")
	ErrMaxBucketsPerRequestRead       = errors.New("unable to read max buckets per request variable")
	ErrObjectGetFailed                = errors.New("failed to get object")
	ErrPolicyMarshalling              = errors.New("failed to marshal policy")
	ErrPublicAccessBlockDisable       = errors.New("failed to disable public access block")
	ErrReplicationConfigurationFailed = errors.New("failed to enable replication configuration")
	ErrResponseReading                = errors.New("error reading response")
	ErrVersioningEnableFailed         = errors.New("failed to enable versioning")
)

func AWSContextRetrievalError() error {
	return ErrAWSContextRetrieval
}

func BucketCreationFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketCreationFailed, cause)
}

func BucketDeletionFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketDeletionFailed, cause)
}

func BucketPolicyApplicationError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketPolicyApplication, cause)
}

func BucketPolicyDeletionError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketPolicyDeletion, cause)
}

func BucketPolicyMarshallingError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketPolicyMarshalling, cause)
}

func BucketStatusWriteFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketStatusWriteFailed, cause)
}

func BucketTagsAddFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrBucketTagsAddFailed, cause)
}

func EventBridgeEnableFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrEventBridgeEnableFailed, cause)
}

func ExceededMaxBucketsPerRequestError(limit, requested int) error {
	return fmt.Errorf("%w: limit=%d requested=%d", ErrExceededMaxBucketsPerRequest, limit, requested)
}

func InventoryConfigurationFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrInventoryConfigurationFailed, cause)
}

func InvalidBucketNameRequestedError(bucketName string) error {
	return fmt.Errorf("%w: bucket=%s", ErrInvalidBucketNameRequested, bucketName)
}

func LifecycleConfigurationFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrLifecycleConfigurationFailed, cause)
}

func LifecycleRuleSetFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrLifecycleRuleSetFailed, cause)
}

func LoggingEnableFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrLoggingEnableFailed, cause)
}

func MaxBucketsPerRequestReadError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrMaxBucketsPerRequestRead, cause)
}

func ObjectGetFailedError(key, bucket string, cause error) error {
	return fmt.Errorf("%w: key=%s bucket=%s cause=%v", ErrObjectGetFailed, key, bucket, cause)
}

func PolicyMarshallingError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrPolicyMarshalling, cause)
}

func PublicAccessBlockDisableError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrPublicAccessBlockDisable, cause)
}

func ReplicationConfigurationFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrReplicationConfigurationFailed, cause)
}

func ResponseReadingError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrResponseReading, cause)
}

func VersioningEnableFailedError(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrVersioningEnableFailed, cause)
}
