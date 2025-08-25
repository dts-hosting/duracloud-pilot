package integration

import (
	"context"
	"duracloud/internal/buckets"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BucketCreationCoordinator manages bucket creation requests to prevent interference
// while allowing maximum parallelism for tests
type BucketCreationCoordinator struct {
	mutex sync.Mutex
}

// Global coordinator instance
var bucketCoordinator = &BucketCreationCoordinator{}

// BucketCreationRequest represents a single bucket creation request
type BucketCreationRequest struct {
	StackName       string
	RequestContent  string
	ExpectedBuckets []string
	MaxWaitTime     time.Duration
}

// SubmitBucketCreationRequest submits a bucket creation request and waits for completion
// This method serializes the upload to prevent Lambda interference, but allows
// parallel waiting for bucket creation completion
func (c *BucketCreationCoordinator) SubmitBucketCreationRequest(
	t *testing.T,
	ctx context.Context,
	s3Client *s3.Client,
	request BucketCreationRequest,
) error {
	// Phase 1: Serialize the upload to prevent Lambda interference
	uploadComplete := make(chan error, 1)

	go func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()

		t.Logf("Acquired bucket creation lock for %d buckets", len(request.ExpectedBuckets))

		// Upload request file to trigger bucket creation
		triggerBucket := fmt.Sprintf("%s%s", request.StackName, buckets.BucketRequestedSuffix)
		requestKey := fmt.Sprintf("test-request-%d.txt", time.Now().UnixNano()) // Use nanoseconds for uniqueness

		err := uploadToS3(ctx, s3Client, triggerBucket, requestKey, request.RequestContent)
		if err != nil {
			uploadComplete <- fmt.Errorf("failed to upload request file: %w", err)
			return
		}

		t.Logf("Uploaded bucket creation request: s3://%s/%s", triggerBucket, requestKey)

		// Small delay to ensure Lambda starts processing before releasing lock
		time.Sleep(2 * time.Second)

		t.Logf("Released bucket creation lock - Lambda processing started")
		uploadComplete <- nil
	}()

	// Wait for upload to complete
	if err := <-uploadComplete; err != nil {
		return err
	}

	// Phase 2: Wait for buckets to exist (can happen in parallel with other tests)
	// For bucket_requested tests, we just need basic existence - the tests will verify configurations
	t.Logf("Waiting for %d buckets to exist (parallel phase)", len(request.ExpectedBuckets))

	cfg := DefaultWaitConfig()
	cfg.MaxTimeout = request.MaxWaitTime
	cfg.PollInterval = 1 * time.Second // Faster polling for basic existence checks
	cfg.InitialDelay = 3 * time.Second // Give Lambda time to start processing
	cfg.BackoffFactor = 1.2            // Gentle backoff for bucket existence
	cfg.MaxPollInterval = 5 * time.Second

	// Check bucket existence and correct policy configuration
	condition := func() bool {
		for _, bucketName := range request.ExpectedBuckets {
			if !bucketExists(ctx, s3Client, bucketName) {
				t.Logf("Bucket %s does not exist yet", bucketName)
				return false
			}

			// For public buckets, also verify the correct policy is applied
			if buckets.IsPublicBucket(bucketName) {
				if !hasCorrectPublicPolicy(ctx, s3Client, bucketName) {
					t.Logf("Public bucket %s does not have correct policy yet", bucketName)
					return false
				}
			}
		}
		return true
	}

	description := fmt.Sprintf("%d S3 buckets to exist", len(request.ExpectedBuckets))
	success := WaitForCondition(t, description, condition, cfg)
	if !success {
		return fmt.Errorf("timeout waiting for buckets to exist: %v", request.ExpectedBuckets)
	}

	t.Logf("Successfully created %d buckets", len(request.ExpectedBuckets))
	return nil
}

// CreateBucketsWithCoordination creates buckets using the coordinated approach
// This is the main function that tests should use for bucket creation
func CreateBucketsWithCoordination(
	t *testing.T,
	ctx context.Context,
	s3Client *s3.Client,
	stackName string,
	bucketNames []string,
	maxWaitTime time.Duration,
) error {
	// Build request content
	requestContent := ""
	for _, bucketName := range bucketNames {
		requestContent += bucketName + "\n"
	}

	// Build expected full bucket names
	expectedBuckets := make([]string, len(bucketNames))
	for i, bucketName := range bucketNames {
		expectedBuckets[i] = fmt.Sprintf("%s-%s", stackName, bucketName)
	}

	request := BucketCreationRequest{
		StackName:       stackName,
		RequestContent:  requestContent,
		ExpectedBuckets: expectedBuckets,
		MaxWaitTime:     maxWaitTime,
	}

	return bucketCoordinator.SubmitBucketCreationRequest(t, ctx, s3Client, request)
}

// hasCorrectPublicPolicy checks if a public bucket has the correct "AllowPublicRead" policy
func hasCorrectPublicPolicy(ctx context.Context, s3Client *s3.Client, bucketName string) bool {
	policy := getBucketPolicy(ctx, s3Client, bucketName)
	if policy == nil {
		return false
	}

	var policyDoc map[string]interface{}
	err := json.Unmarshal([]byte(*policy), &policyDoc)
	if err != nil {
		return false
	}

	statements, ok := policyDoc["Statement"].([]interface{})
	if !ok || len(statements) == 0 {
		return false
	}

	statement, ok := statements[0].(map[string]interface{})
	if !ok {
		return false
	}

	sid, ok := statement["Sid"].(string)
	if !ok {
		return false
	}

	// Check if the policy has "AllowPublicRead" Sid (not "DenyAllUploads")
	return sid == "AllowPublicRead"
}
