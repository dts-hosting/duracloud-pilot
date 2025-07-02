package main

import (
	"context"
	"duracloud/internal/checksum"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"errors"
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

	// tmp
	fmt.Println(schedulerTable)
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		if !isTTLExpiry(record) {
			continue
		}

		obj, err := extractBucketAndObject(record)
		if err != nil {
			log.Printf("failed to extract bucket/object: %s", err.Error())
			continue
		}

		err = processChecksumVerification(ctx, s3Client, dynamodbClient, obj, checksumTable, schedulerTable)
		if err != nil {
			log.Printf("failed to process checksum verification: %s", err.Error())
			// TODO: put checksum record failure
			continue
		}
	}

	return nil
}

func extractBucketAndObject(record events.DynamoDBEventRecord) (files.S3Object, error) {
	bucket, exists := record.Change.OldImage[string(db.ChecksumTableBucketNameId)]
	if !exists {
		return files.S3Object{}, fmt.Errorf("missing bucket name attribute")
	}

	object, exists := record.Change.OldImage[string(db.ChecksumTableObjectKeyId)]
	if !exists {
		return files.S3Object{}, fmt.Errorf("missing object key attribute")
	}

	return files.S3Object{Bucket: bucket.String(), Key: object.String()}, nil
}

func isTTLExpiry(record events.DynamoDBEventRecord) bool {
	return record.EventName == "REMOVE" &&
		record.UserIdentity != nil &&
		record.UserIdentity.Type == "Service" &&
		record.UserIdentity.PrincipalID == "dynamodb.amazonaws.com"
}

func processChecksumVerification(
	ctx context.Context,
	s3Client *s3.Client,
	dynamodbClient *dynamodb.Client,
	obj files.S3Object,
	checksumTable string,
	schedulerTable string,
) error {
	log.Printf("Starting checksum verification for: %s/%s", obj.Bucket, obj.Key)

	currentTime := time.Now()
	nextScheduledTime, err := db.GetNextScheduledTime()
	if err != nil {
		return err
	}

	checksumRecord, err := db.GetChecksumRecord(ctx, dynamodbClient, checksumTable, obj)
	if err != nil {
		return err
	}
	checksumRecord.LastChecksumDate = currentTime
	checksumRecord.NextChecksumDate = nextScheduledTime
	checksumRecord.LastChecksumSuccess = false

	calc := checksum.NewS3Calculator(s3Client)
	checksumResult, err := calc.CalculateChecksum(ctx, obj)
	if err != nil {
		checksumRecord.LastChecksumMessage = err.Error()
		// TODO: do we want to use checksumResult.Checksum to record it?
		checksumRecord.Checksum = ""

		err = db.PutChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
		if err != nil {
			log.Printf("Failed to record failed checksum in db due to : %v", err)
			return err
		}

		return err
	}

	// TODO: remove these temporary logging statements
	log.Printf("Calculated checksum: %s", checksumResult)
	log.Printf("Checksum record: %v", checksumRecord)

	if checksumResult != checksumRecord.Checksum {
		log.Printf("Checksum mismatch. Have %s, expected %s", checksumResult, checksumRecord.Checksum)
		checksumRecord.LastChecksumMessage = "checksum mismatch"
		err = db.PutChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
		if err != nil {
			log.Printf("Failed to record failed checksum in db due to : %v", err)
			return err
		}
		return errors.New("checksum does not match")
	} else {
		checksumRecord.LastChecksumSuccess = true
		checksumRecord.LastChecksumMessage = "ok"
	}

	err = db.PutChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
	if err != nil {
		log.Printf("Failed to record successful checksum in db due to : %v", err)
		return err
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
