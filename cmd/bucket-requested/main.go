package main

import (
	"context"
	"duracloud/internal/accounts"
	"duracloud/internal/buckets"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	accountID          string
	awsCtx             accounts.AWSContext
	bucketLimit        int
	bucketPrefix       string
	managedBucketName  string
	region             string
	replicationRoleArn string
	s3Client           *s3.Client
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	accountID, err = accounts.GetAccountID(context.Background(), awsConfig)
	if err != nil {
		panic(fmt.Sprintf("Unable to get AWS account ID: %v", err))
	}

	bucketPrefix = os.Getenv("S3_BUCKET_PREFIX")
	bucketLimit, _ = buckets.GetBucketRequestLimit(os.Getenv("S3_MAX_BUCKETS_PER_REQUEST"))
	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")
	region = awsConfig.Region
	replicationRoleArn = os.Getenv("S3_REPLICATION_ROLE_ARN")
	s3Client = s3.NewFromConfig(awsConfig)
	stackName := bucketPrefix

	awsCtx = accounts.AWSContext{
		AccountID: accountID,
		Region:    region,
		StackName: stackName,
	}
}

func handler(ctx context.Context, event json.RawMessage) error {
	bucketsStatus := make(map[string]string)
	ctx = context.WithValue(ctx, accounts.AWSContextKey, awsCtx)
	var s3Event events.S3Event
	if err := json.Unmarshal(event, &s3Event); err != nil {
		return fmt.Errorf("failed to parse event: %v", err)
	}

	e := buckets.S3EventWrapper{
		Event: &s3Event,
	}

	bucketName := e.BucketName()
	objectKey := e.ObjectKey()
	log.Printf("Received event for bucket name: %s, object key: %s", bucketName, objectKey)

	requestedBuckets, err := buckets.GetBuckets(ctx, s3Client, bucketName, objectKey, bucketLimit)
	if err != nil {
		bucketsStatus[buckets.BucketRequestedFileErrorKey] = err.Error()
		_ = buckets.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
		return fmt.Errorf("could not retrieve buckets list: %v", err)
	}
	log.Printf("Retrieved %d buckets list from request file", len(requestedBuckets))

	resultChan := make(chan map[string]string, len(requestedBuckets))

	for _, requestedBucketName := range requestedBuckets {
		go func(bucketName string) {
			bucket := buckets.BucketRequest{
				Name:               bucketName,
				Prefix:             bucketPrefix,
				ManagedBucketName:  managedBucketName,
				ReplicationRoleArn: replicationRoleArn,
				ResultChan:         resultChan,
			}
			processBucket(ctx, s3Client, bucket)
		}(requestedBucketName)
	}

	for i := 0; i < len(requestedBuckets); i++ {
		results := <-resultChan
		for bucket, status := range results {
			bucketsStatus[bucket] = status
		}
	}

	err = buckets.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
	if err != nil {
		return fmt.Errorf("could not write bucket status to managed bucket: %v", err)
	}

	return nil
}

func processBucket(ctx context.Context, s3Client *s3.Client, bucket buckets.BucketRequest) {
	localStatus := make(map[string]string)
	fullBucketName := bucket.FullName()
	replicationBucketName := bucket.ReplicationName()
	log.Printf("Creating buckets: %s [%s]", fullBucketName, replicationBucketName)

	err := buckets.CreateNewBucket(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.AddDenyUploadPolicy(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.AddBucketTags(ctx, s3Client, fullBucketName, bucket.Prefix, "Standard")
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.EnableVersioning(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.AddExpiration(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	if buckets.IsPublicBucket(fullBucketName) {
		err = buckets.MakePublic(ctx, s3Client, fullBucketName)
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			bucket.ResultChan <- localStatus
			return
		}

		err = buckets.AddBucketTags(ctx, s3Client, fullBucketName, bucket.Prefix, "Public")
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			bucket.ResultChan <- localStatus
			return
		}
	} else {
		err := buckets.AddStandardLifecycle(ctx, s3Client, fullBucketName)
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			bucket.ResultChan <- localStatus
			return
		}
	}

	err = buckets.EnableEventBridge(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.EnableInventory(ctx, s3Client, fullBucketName, bucket.ManagedBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.EnableLogging(ctx, s3Client, fullBucketName, bucket.ManagedBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.CreateNewBucket(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.AddBucketTags(ctx, s3Client, replicationBucketName, bucket.Prefix, "Replication")
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.EnableVersioning(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.AddExpiration(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.EnableReplication(ctx, s3Client, fullBucketName, replicationBucketName, bucket.ReplicationRoleArn)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.AddReplicationLifecycle(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	err = buckets.RemovePolicy(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		bucket.ResultChan <- localStatus
		return
	}

	// Note: we have to do this after removing the temporary DENY policy
	if buckets.IsPublicBucket(fullBucketName) {
		err = buckets.AddPublicPolicy(ctx, s3Client, fullBucketName)
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			_ = rollback(ctx, s3Client, replicationBucketName)
			bucket.ResultChan <- localStatus
			return
		}
	}

	localStatus[fullBucketName] = "Bucket created successfully"
	bucket.ResultChan <- localStatus
}

func rollback(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	return buckets.DeleteBucket(ctx, s3Client, bucketName)
}

func main() {
	lambda.Start(handler)
}
