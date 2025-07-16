package main

import (
	"context"
	"duracloud/internal/accounts"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"os"
	"time"
)

var (
	accountID         string
	awsCtx            accounts.AWSContext
	bucketPrefix      string
	exportBucket      string
	managedBucketName string
	prefix            string
	region            string
	s3Client          *s3.Client
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	bucketPrefix = os.Getenv("S3_BUCKET_PREFIX")
	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")
	timestamp := time.Now().Format("2006-01-02")
	prefix = fmt.Sprintf("exports/checksum-table/%s/", timestamp)
	s3Client = s3.NewFromConfig(awsConfig)
	stackName := bucketPrefix

	awsCtx = accounts.AWSContext{
		AccountID: accountID,
		Region:    region,
		StackName: stackName,
	}
}

func handler(ctx context.Context, event json.RawMessage) error {
	ctx = context.WithValue(ctx, accounts.AWSContextKey, awsCtx)
	return nil
}

func main() {
	lambda.Start(handler)
}
