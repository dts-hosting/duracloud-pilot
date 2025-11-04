package main

import (
	"context"
	"duracloud/internal/exports"
	"duracloud/internal/files"
	"duracloud/internal/inventory"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	s3Client *s3.Client
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event json.RawMessage) error {
	var s3Event exports.S3Event

	if err := json.Unmarshal(event, &s3Event); err != nil {
		return fmt.Errorf("failed to parse S3 event: %w", err)
	}

	if len(s3Event.Records) == 0 {
		return fmt.Errorf("no S3 records in event")
	}

	obj := files.NewS3Object(s3Event.BucketName(), s3Event.ObjectKey())

	if !strings.HasSuffix(obj.Key, inventory.ManifestFile) {
		return fmt.Errorf("invalid manifest file: %s", obj.Key)
	}

	log.Printf("Processing manifest: %s, Key: %s", obj.Bucket, obj.Key)

	// Process the manifest and collect the files to process
	unwrapper := inventory.NewInventoryUnwrapper(ctx, s3Client, obj)
	err := unwrapper.ProcessInventoryFiles()
	if err != nil {
		return fmt.Errorf("error generating consolidated inventory: %w", err)
	}

	log.Printf("Successfully processed manifest: %s, Key: %s", obj.Bucket, obj.Key)

	return nil
}

func main() {
	lambda.Start(handler)
}
