package main

import (
	"context"
	"duracloud/internal/checksum"
	"duracloud/internal/db"
	"duracloud/internal/queues"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"log"
	"os"
	"time"
)

var (
	checksumTable  string
	dynamodbClient *dynamodb.Client
	schedulerTable string
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(
				retry.NewStandard(), 10)
		}),
	)
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	checksumTable = os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
	schedulerTable = os.Getenv("DYNAMODB_SCHEDULER_TABLE")
}

func handler(ctx context.Context, event json.RawMessage) (events.SQSEventResponse, error) {
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err != nil {
		return events.SQSEventResponse{}, fmt.Errorf("failed to parse SQS event: %v", err)
	}

	sqsEventWrapper := queues.SQSEventWrapper{
		Event: &sqsEvent,
	}

	parsedEvents, failedEvents := sqsEventWrapper.UnwrapS3EventBridgeEvents()

	for _, parsedEvent := range parsedEvents {
		if !parsedEvent.IsObjectDeleted() || parsedEvent.IsIgnoreFilesBucket() {
			continue
		}

		bucketName := parsedEvent.BucketName()
		objectKey := parsedEvent.ObjectKey()
		obj := checksum.NewS3Object(bucketName, objectKey)
		log.Printf("Processing delete event for bucket name: %s, object key: %s", bucketName, objectKey)

		if err := processDeletedObject(ctx, dynamodbClient, obj); err != nil {
			// only use for retryable errors
			log.Printf("Failed to process deleted object %s/%s: %v", bucketName, objectKey, err)
			failedEvents = append(failedEvents, events.SQSBatchItemFailure{
				ItemIdentifier: parsedEvent.MessageId,
			})
		}
	}

	log.Printf("Finished processing delete events. Failed events: %d", len(failedEvents))

	return events.SQSEventResponse{
		BatchItemFailures: failedEvents,
	}, nil
}

func processDeletedObject(ctx context.Context, dynamodbClient *dynamodb.Client, obj checksum.S3Object) error {
	checksumRecord := db.ChecksumRecord{
		Bucket: obj.Bucket,
		Object: obj.Key,
	}

	err := db.DeleteChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
	if err != nil {
		return err
	}

	err = db.DeleteChecksumRecord(ctx, dynamodbClient, schedulerTable, checksumRecord)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond) // rate limit ourselves in case of very heavy bursts
	return nil
}

func main() {
	lambda.Start(handler)
}
