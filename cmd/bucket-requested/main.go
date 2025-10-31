package main

import (
	"context"
	"duracloud/internal/accounts"
	"duracloud/internal/buckets"
	"duracloud/internal/files"
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
	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")

	bucketLimit, err = buckets.GetBucketRequestLimit(os.Getenv("S3_MAX_BUCKETS_PER_REQUEST"))
	if err != nil {
		log.Printf("Invalid S3_MAX_BUCKETS_PER_REQUEST, using default: %v", err)
		bucketLimit = buckets.DefaultBucketRequestLimit
	}
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

	obj := files.NewS3Object(e.BucketName(), e.ObjectKey())
	log.Printf("Received event for bucket name: %s, object key: %s", obj.Bucket, obj.Key)

	requestedBuckets, err := buckets.GetBuckets(ctx, s3Client, obj, bucketLimit)
	if err != nil {
		bucketsStatus[buckets.BucketRequestedFileErrorKey] = err.Error()
		_ = buckets.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
		return fmt.Errorf("could not retrieve buckets list: %v", err)
	}

	log.Printf("Retrieved %d buckets list from request file", len(requestedBuckets))
	resultChan := make(chan map[string]string, len(requestedBuckets))

	for _, requestedBucketName := range requestedBuckets {
		go func(bucketName string) {
			bucket := buckets.NewBucketRequest(
				ctx,
				s3Client,
				bucketName,
				bucketPrefix,
				managedBucketName,
				replicationRoleArn,
				resultChan,
			)
			bucket.Setup()
		}(requestedBucketName)
	}

	for range len(requestedBuckets) {
		results := <-resultChan
		for bucket, status := range results {
			log.Printf("Bucket status: %s %s\n", bucket, status)
			bucketsStatus[bucket] = status
		}
	}

	err = buckets.WriteStatus(ctx, s3Client, managedBucketName, bucketsStatus)
	if err != nil {
		return fmt.Errorf("could not write bucket status to managed bucket: %v", err)
	}

	log.Printf("Successfully processed event for bucket name: %s, object key: %s", obj.Bucket, obj.Key)

	return nil
}

func main() {
	lambda.Start(handler)
}
