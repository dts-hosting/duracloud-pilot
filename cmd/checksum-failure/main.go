package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		if record.Change.NewImage != nil {
			bucket := record.Change.NewImage["Bucket"].String()
			object := record.Change.NewImage["Object"].String()

			if checksumSuccess, exists := record.Change.NewImage["LastChecksumSuccess"]; exists {
				success := checksumSuccess.Boolean()
				if !success {
					log.Printf("CHECKSUM FAILURE DETECTED: %s/%s", bucket, object)

					if lastError, exists := record.Change.NewImage["LastChecksumMessage"]; exists {
						log.Printf("Error Details: %s", lastError.String())
					}

					// TODO: handle failure
					// - Upload report to s3 managed bucket?
					// - Send SES email?
					// - etc.
				}
			}
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
