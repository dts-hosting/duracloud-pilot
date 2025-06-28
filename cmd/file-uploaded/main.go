package main

import (
	"context"
	"duracloud/internal/checksum"
	"duracloud/internal/db"
	"duracloud/internal/queues"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"os"
	"time"
)

var (
	checksumTable  string
	dynamodbClient *dynamodb.Client
	s3Client       *s3.Client
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
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	checksumTable = os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
	s3Client = s3.NewFromConfig(awsConfig)
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
		if !parsedEvent.IsObjectCreated() || parsedEvent.IsIgnoreFilesBucket() {
			continue
		}

		obj := checksum.NewS3Object(parsedEvent.BucketName(), parsedEvent.ObjectKey())
		log.Printf("Processing upload event for bucket name: %s, object key: %s", obj.Bucket, obj.Key)

		if err := processUploadedObject(ctx, s3Client, dynamodbClient, obj); err != nil {
			// Only retry if the uploaded file exists
			log.Printf("Failed to process uploaded object %s/%s: %v", obj.Bucket, obj.Key, err)
			// TODO: check if the object actually exists, if it does add to failed and notify (expected but failed)
			failedEvents = append(failedEvents, events.SQSBatchItemFailure{
				ItemIdentifier: parsedEvent.MessageId,
			})
		}
	}

	log.Printf("Finished processing upload events. Failed events: %d", len(failedEvents))

	return events.SQSEventResponse{
		BatchItemFailures: failedEvents,
	}, nil
}

func processUploadedObject(
	ctx context.Context,
	s3Client *s3.Client,
	dynamodbClient *dynamodb.Client,
	obj checksum.S3Object,
) error {
	nextScheduledTime, err := db.GetNextScheduledTime()
	if err != nil {
		return err
	}

	calc := checksum.NewS3Calculator(s3Client)
	hash, err := calc.CalculateChecksum(ctx, obj)

	// Optimistic outlook for our adventurer checksum record
	checksumRecord := db.ChecksumRecord{
		BucketName:          obj.Bucket,
		ObjectKey:           obj.Key,
		Checksum:            hash, // "" if failed
		LastChecksumDate:    time.Now(),
		LastChecksumMessage: "ok",
		LastChecksumSuccess: true,
		NextChecksumDate:    nextScheduledTime,
	}

	if err != nil {
		log.Printf("Failed to calculate checksum: %v", err)
		checksumRecord.LastChecksumMessage = err.Error()
		checksumRecord.LastChecksumSuccess = false
	} else {
		err = db.ScheduleNextVerification(ctx, dynamodbClient, schedulerTable, checksumRecord)
		if err != nil {
			return err
		}
	}

	err = db.PutChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond) // rate limit ourselves in case of very heavy bursts
	return nil
}

func main() {
	lambda.Start(handler)
}
