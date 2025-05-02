package main

import (
	"context"
	"encoding/json"
	"log"

	events "duracloud/internal/events"

	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	// TODO: aws client setup etc.
}

func handler(ctx context.Context, event json.RawMessage) error {
	var bucketEvent events.BucketCreatedEvent
	if err := json.Unmarshal(event, &bucketEvent); err != nil {
		log.Printf("Failed to parse event: %v", err)
		return err
	}

	bucketName := bucketEvent.BucketName()
	log.Printf("Bucket name: %s", bucketName)

	return nil
}

func main() {
	lambda.Start(handler)
}
