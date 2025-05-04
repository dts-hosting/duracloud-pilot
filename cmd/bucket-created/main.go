package main

import (
	"context"
	"encoding/json"
	"log"

	"duracloud/internal/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

var awsConfig aws.Config

func init() {
	var err error
	awsConfig, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}
}

func handler(ctx context.Context, event json.RawMessage) error {
	var bucketEvent events.BucketCreatedEvent
	if err := json.Unmarshal(event, &bucketEvent); err != nil {
		log.Printf("Failed to parse event: %v", err)
		return err
	}

	bucketName := bucketEvent.BucketName()
	log.Printf("Received event for bucket name: %s", bucketName)

	// abort if restricted bucket

	// enable event bridge notifications
	if err := enableEventBridgeNotifications(ctx, bucketName); err != nil {
		log.Printf("Failed to enable EventBridge notifications for bucket %s: %v", bucketName, err)
		return err
	}

	// apply storage tier and lifecycle rules (IA -> Glacier Instant, Standard -> IA [public])

	// setup replication (Glacier)

	// setup inventory

	// setup public access (if public bucket)

	// send notification (or push report file to indicate) that bucket is ready (?)

	return nil
}

func main() {
	lambda.Start(handler)
}
