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
				retry.NewStandard(), 10)
		}),
	)
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	checksumTable = os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
	s3Client = s3.NewFromConfig(awsConfig)
	schedulerTable = os.Getenv("DYNAMODB_SCHEDULER_TABLE")

	// tmp
	fmt.Println(checksumTable)
	fmt.Println(schedulerTable)
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

func processUploadedObject(
	ctx context.Context,
	s3Client *s3.Client,
	dynamodbClient *dynamodb.Client,
	obj checksum.S3Object,
) error {
	calc := checksum.NewS3Calculator(s3Client)
	hash, err := calc.CalculateChecksum(ctx, obj)
	nextScheduledTime, err := db.GetNextScheduledTime()
	nowTime := time.Now()
	if err != nil {
		log.Printf("Failed to get a scheduled time %v, err")
	}

	if err != nil {
		log.Printf("Failed to get scheduled time %v, err")
	}

	checksumRecord := db.ChecksumRecord{}
	if err != nil {
		log.Printf("Failed to calculate checksum: %v", err)
		checksumRecord = db.ChecksumRecord{
			obj.Bucket,
			obj.Key,
			hash,
			nowTime,
			"calc fail",
			false,
			nextScheduledTime,
		}
	} else {
		checksumRecord = db.ChecksumRecord{
			obj.Bucket,
			obj.Key,
			hash,
			nowTime,
			"ok",
			true,
			nextScheduledTime,
		}
	}
		err = db.PutChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
	if err != nil {
		log.Printf("Failed to store checksum: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // rate limit ourselves in case of very heavy bursts
	return nil
}

func main() {
	lambda.Start(handler)
}
