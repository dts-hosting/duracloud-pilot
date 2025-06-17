package main

import (
	"context"
	"duracloud/internal/helpers"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"os"
)

var accountID string
var awsCtx helpers.AWSContext
var region string
var s3Client *s3.Client

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	accountID, err = helpers.GetAccountID(context.Background(), awsConfig)
	if err != nil {
		log.Fatalf("Unable to get AWS account ID: %v", err)
	}

	region = awsConfig.Region
	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event json.RawMessage) error {
	awsCtx = helpers.AWSContext{
		AccountID: accountID,
		Region:    region,
	}
	ctx = context.WithValue(ctx, helpers.AWSContextKey, awsCtx)

	bucketPrefix := os.Getenv("S3_BUCKET_PREFIX")
	bucketLimit, _ := helpers.GetBucketRequestLimit(os.Getenv("S3_MAX_BUCKETS_PER_REQUEST"))
	bucketsStatus := make(map[string]string)
	managedBucketName := fmt.Sprintf("%s%s", bucketPrefix, helpers.ManagedSuffix)
	replicationRoleArn := os.Getenv("S3_REPLICATION_ROLE_ARN")

	var s3Event events.S3Event
	if err := json.Unmarshal(event, &s3Event); err != nil {
		log.Fatalf("Failed to parse event: %v", err)
		return err
	}

	e := helpers.S3EventWrapper{
		Event: &s3Event,
	}

	bucketName := e.BucketName()
	objectKey := e.ObjectKey()
	log.Printf("Received event for bucket name: %s, object key: %s", bucketName, objectKey)

	requestedBuckets, err := helpers.GetBuckets(ctx, s3Client, bucketName, objectKey, bucketLimit)
	if err != nil {
		bucketsStatus[helpers.BucketRequestedFileErrorKey] = err.Error()
		_ = helpers.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
		log.Fatalf("Error retrieving buckets list: %v", err)
	}
	log.Printf("Retrieved %d buckets list from request file", len(requestedBuckets))

	resultChan := make(chan map[string]string, len(requestedBuckets))

	for _, requestedBucketName := range requestedBuckets {
		go func(bucketName string) {
			params := helpers.ProcessBucketParams{
				RequestedBucketName: bucketName,
				BucketPrefix:        bucketPrefix,
				ManagedBucketName:   managedBucketName,
				ReplicationRoleArn:  replicationRoleArn,
				ResultChan:          resultChan,
			}
			processBucket(ctx, s3Client, params)
		}(requestedBucketName)
	}

	for i := 0; i < len(requestedBuckets); i++ {
		results := <-resultChan
		for bucket, status := range results {
			bucketsStatus[bucket] = status
		}
	}

	err = helpers.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
	if err != nil {
		log.Printf("Error writing bucket status to managed bucket: %v", err)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}

func processBucket(ctx context.Context, s3Client *s3.Client, params helpers.ProcessBucketParams) {
	localStatus := make(map[string]string)
	fullBucketName := fmt.Sprintf("%s-%s", params.BucketPrefix, params.RequestedBucketName)
	replicationBucketName := fmt.Sprintf("%s%s", fullBucketName, helpers.ReplicationSuffix)
	log.Printf("Creating buckets: %s [%s]", fullBucketName, replicationBucketName)

	err := helpers.CreateNewBucket(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		params.ResultChan <- localStatus
		return
	}

	err = helpers.AddDenyUploadPolicy(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, params.BucketPrefix, "Standard")
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.EnableVersioning(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.AddExpiration(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	if helpers.IsPublicBucket(fullBucketName) {
		err = helpers.MakePublic(ctx, s3Client, fullBucketName)
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			params.ResultChan <- localStatus
			return
		}

		err = helpers.AddBucketTags(ctx, s3Client, fullBucketName, params.BucketPrefix, "Public")
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			params.ResultChan <- localStatus
			return
		}
	} else {
		err := helpers.AddStandardLifecycle(ctx, s3Client, fullBucketName)
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			params.ResultChan <- localStatus
			return
		}
	}

	err = helpers.EnableEventBridge(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.EnableInventory(ctx, s3Client, fullBucketName, params.ManagedBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.EnableLogging(ctx, s3Client, fullBucketName, params.ManagedBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.CreateNewBucket(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.AddBucketTags(ctx, s3Client, replicationBucketName, params.BucketPrefix, "Replication")
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.EnableVersioning(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.AddExpiration(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.EnableReplication(ctx, s3Client, fullBucketName, replicationBucketName, params.ReplicationRoleArn)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.AddReplicationLifecycle(ctx, s3Client, replicationBucketName)
	if err != nil {
		localStatus[replicationBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		_ = rollback(ctx, s3Client, replicationBucketName)
		params.ResultChan <- localStatus
		return
	}

	err = helpers.RemovePolicy(ctx, s3Client, fullBucketName)
	if err != nil {
		localStatus[fullBucketName] = err.Error()
		_ = rollback(ctx, s3Client, fullBucketName)
		params.ResultChan <- localStatus
		return
	}

	// Note: we have to do this after removing the temporary DENY policy
	if helpers.IsPublicBucket(fullBucketName) {
		err = helpers.AddPublicPolicy(ctx, s3Client, fullBucketName)
		if err != nil {
			localStatus[fullBucketName] = err.Error()
			_ = rollback(ctx, s3Client, fullBucketName)
			params.ResultChan <- localStatus
			return
		}
	}

	localStatus[fullBucketName] = fmt.Sprintf("Created bucket %s", fullBucketName)
	params.ResultChan <- localStatus
}

func rollback(ctx context.Context, s3Client *s3.Client, bucket string) error {
	return helpers.DeleteBucket(ctx, s3Client, bucket)
}
