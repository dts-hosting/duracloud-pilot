package main

import (
	"context"
	"duracloud/internal/checksum"
	"duracloud/internal/queues"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"time"
)

var dynamodbClient *dynamodb.Client
var s3Client *s3.Client

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

	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
	s3Client = s3.NewFromConfig(awsConfig)
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
		if !parsedEvent.IsObjectCreated() || parsedEvent.IsRestrictedBucket() {
			continue
		}

		bucketName := parsedEvent.BucketName()
		objectKey := parsedEvent.ObjectKey()
		obj := checksum.NewS3Object(bucketName, objectKey)
		log.Printf("Processing upload event for bucket name: %s, object key: %s", bucketName, objectKey)

		if err := processUploadedObject(ctx, s3Client, dynamodbClient, obj); err != nil {
			// only use for retryable errors
			log.Printf("Failed to process uploaded object %s/%s: %v", bucketName, objectKey, err)
			failedEvents = append(failedEvents, events.SQSBatchItemFailure{
				ItemIdentifier: parsedEvent.MessageId,
			})
		}
	}

	return events.SQSEventResponse{
		BatchItemFailures: failedEvents,
	}, nil
}

func main() {
	lambda.Start(handler)
}

func processUploadedObject(
	ctx context.Context,
	s3Client *s3.Client,
	dynamodbClient *dynamodb.Client,
	obj checksum.S3Object,
) error {
	// TODO: continue implementation ...
	// - Calc checksum
	// - Not ok: LastChecksumDate & LastChecksumSuccess (f) & Msg, PutChecksumRecord
	// - ok: LastChecksumDate & Msg ("ok"), PutChecksumRecord, Schedule next check

	time.Sleep(100 * time.Millisecond) // rate limit ourselves in case of very heavy bursts
	return nil
}
