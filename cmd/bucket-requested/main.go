package main

import (
	"context"
	"duracloud/internal/helpers"
	"encoding/json"
	"strings"
	"bufio"
	"log"
	"fmt"
	"os"
	"io"
	"strconv"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var awsConfig aws.Config

func init() {
	var err error
	awsConfig, err = config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}
}

func getS3Client() *s3.Client {
	s3Client := s3.NewFromConfig(awsConfig)
	log.Printf("Using S3 client: %v", s3Client)
	return s3Client
}

func getMaxBuckets() int {
	var maxBucketsEnv = os.Getenv("S3_MAX_BUCKETS_PER_REQUEST")
	var maxBuckets, err = strconv.Atoi(maxBucketsEnv)

	if err != nil {
		log.Fatalf("Unable to read max buckets per request environment variable due to : %v", err)
	}
	return maxBuckets
}

func validateBucketName(bucketName string) bool {
	var (
		whitelist = "a-zA-Z0-9-"
		disallowed = regexp.MustCompile(fmt.Sprintf("[^%s]+", whitelist))
	)
	return !disallowed.MatchString(bucketName)
}

func getBuckets(ctx context.Context, bucket string, key string) ([]string) {
	var buckets []string

	client := getS3Client()

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		log.Fatalf("failed to get object: %s from %s due to %s", key, bucket, err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read body: %s", err)
		return nil
	}

	reader := strings.NewReader(string(body))
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Reading bucket name: ", line)
		if validateBucketName(line) {
			buckets = append(buckets, line)
		} else {
			log.Fatalf("invalid bucket name requested: %s", line)
			return nil
		}
	}

	var maxBuckets = getMaxBuckets()
	bucketsRequested := len(buckets)
	if bucketsRequested >= maxBuckets {
		log.Fatalf("Exceeded maximum allowed buckets per request [%s] with [%s]",
										maxBuckets, bucketsRequested)
		return nil
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading string:", err)
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
	log.Printf("Retrieved %s buckets list from request file", len(buckets))

	// 2. Create bucket & replication bucket with required configuration

	// 3. Upload log to managed bucket

	return nil
}

func main() {
	lambda.Start(handler)
}
