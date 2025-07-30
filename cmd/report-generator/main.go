package main

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"os"
)

var (
	managedBucketName string
)

func init() {
	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")
}

func handler(ctx context.Context) error {
	log.Printf("Managed bucket name: %s", managedBucketName)
	return nil
}

func main() {
	lambda.Start(handler)
}
