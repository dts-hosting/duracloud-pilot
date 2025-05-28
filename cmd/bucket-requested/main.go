package main

import (
	"bufio"
	"context"
	"duracloud/internal/helpers"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"os"
	"strconv"
)

var s3Client *s3.Client

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}
	s3Client = s3.NewFromConfig(awsConfig)
}

func getMaxBuckets() int {
	var maxBucketsEnv = os.Getenv("S3_MAX_BUCKETS_PER_REQUEST")
	var maxBuckets, err = strconv.Atoi(maxBucketsEnv)

	if err != nil {
		log.Fatalf("Unable to read max buckets per request environment variable due to : %v", err)
	}
	return maxBuckets
}

// getBuckets retrieves a list of valid bucket names from an S3 object, validates them, and enforces a maximum limit.
func getBuckets(ctx context.Context, bucket string, key string) []string {
	var buckets []string

	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		log.Fatalf("failed to get object: %s from %s due to %s", key, bucket, err)
		return nil
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Reading bucket name: %s", line)
		if helpers.ValidateBucketName(line) {
			buckets = append(buckets, line)
		} else {
			log.Fatalf("invalid bucket name requested: %s", line)
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading response: %v", err)
		return nil
	}

	// TODO: do we want to error like this or ignore extras (i.e. don't append additional buckets above)?
	var maxBuckets = getMaxBuckets()
	bucketsRequested := len(buckets)
	if bucketsRequested >= maxBuckets {
		log.Fatalf("Exceeded maximum allowed buckets per request [%s] with [%s]",
			maxBuckets, bucketsRequested)
		return nil
	}

	return buckets
}

func handler(ctx context.Context, event json.RawMessage) error {
	bucketPrefix := os.Getenv("S3_BUCKET_PREFIX")
	log.Printf("Using bucket prefix: %s", bucketPrefix)

	replicationRoleArn := os.Getenv("S3_REPLICATION_ROLE_ARN")
	log.Printf("Using replication role ARN: %s", replicationRoleArn)

	var s3Event events.S3Event
	if err := json.Unmarshal(event, &s3Event); err != nil {
		log.Printf("Failed to parse event: %v", err)
		return err
	}

	e := helpers.S3EventWrapper{
		Event: &s3Event,
	}

	bucketName := e.BucketName()
	objectKey := e.ObjectKey()
	log.Printf("Received event for bucket name: %s, object key: %s", bucketName, objectKey)

	buckets := getBuckets(ctx, bucketName, objectKey)
	log.Printf("Retrieved %d buckets list from request file", len(buckets))

	// Do all the things ...

	return nil
}

func main() {
	lambda.Start(handler)
}
