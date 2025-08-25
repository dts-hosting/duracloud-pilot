package main

import (
	"context"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"duracloud/internal/queues"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var (
	bucketPrefix   string
	checksumTable  string
	dynamodbClient *dynamodb.Client
	schedulerTable string
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(
				retry.NewStandard(), 5)
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	bucketPrefix = os.Getenv("S3_BUCKET_PREFIX")
	checksumTable = os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
	schedulerTable = os.Getenv("DYNAMODB_SCHEDULER_TABLE")
}

func handler(ctx context.Context, event json.RawMessage) (events.SQSEventResponse, error) {
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err != nil {
		log.Printf("Failed to parse SQS event: %v", err)
		return events.SQSEventResponse{}, nil
	}

	sqsEventWrapper := queues.SQSEventWrapper{
		Event: &sqsEvent,
	}

	parsedEvents, failedEvents := sqsEventWrapper.UnwrapS3EventBridgeEvents()

	for _, parsedEvent := range parsedEvents {
		if parsedEvent.BucketPrefix() != bucketPrefix {
			continue
		}

		if !parsedEvent.IsObjectDeleted() || parsedEvent.IsIgnoreFilesBucket() {
			continue
		}

		obj := files.NewS3Object(parsedEvent.BucketName(), parsedEvent.ObjectKey())
		log.Printf("Processing delete event for bucket name: %s, object key: %s", obj.Bucket, obj.Key)

		if err := processDeletedObject(ctx, dynamodbClient, obj); err != nil {
			_, checkErr := db.GetChecksumRecord(ctx, dynamodbClient, checksumTable, obj)
			if checkErr == nil {
				// Record exists, so we should retry processing
				failedEvents = append(failedEvents, events.SQSBatchItemFailure{
					ItemIdentifier: parsedEvent.MessageId,
				})
			}
		}
	}

	log.Printf("Finished processing delete events. Failed events: %d", len(failedEvents))

	return events.SQSEventResponse{
		BatchItemFailures: failedEvents,
	}, nil
}

func processDeletedObject(ctx context.Context, dynamodbClient *dynamodb.Client, obj files.S3Object) error {
	err := db.DeleteChecksumRecord(ctx, dynamodbClient, checksumTable, obj)
	if err != nil {
		return err
	}

	err = db.DeleteChecksumRecord(ctx, dynamodbClient, schedulerTable, obj)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond) // rate limit ourselves in case of very heavy bursts
	return nil
}

func main() {
	lambda.Start(handler)
}
