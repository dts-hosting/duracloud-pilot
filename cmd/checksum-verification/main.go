package main

import (
	"context"
	"duracloud/internal/checksum"
	"duracloud/internal/db"
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
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		if !isTTLExpiry(record) {
			continue
		}

		bucket, object, err := extractBucketAndObject(record)
		if err != nil {
			return fmt.Errorf("failed to extract bucket/object: %w", err)
		}

		log.Printf("Starting checksum verification for: %s %s", bucket, object)

		//currentTime := time.Now()
		//nextScheduledTime := db.GetNextScheduledTime()

		checksumRecord := db.ChecksumRecord{
			Bucket: bucket,
			Object: object,
		}

		checksumRecord, err = db.GetChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
		if err != nil {
			// TODO update checksumRecord for failure and PutChecksumRecord (continue)
			return fmt.Errorf("failed to get checksum record: %w", err)
		}

		obj := checksum.NewS3Object(bucket, object)
		calc := checksum.NewS3Calculator(s3Client)
		checksumResult, err := calc.CalculateChecksum(ctx, obj)
		if err != nil {
			// TODO update checksumRecord for failure and PutChecksumRecord (continue)
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}

		log.Printf("Calculated checksum: %s", checksumResult)
		log.Printf("Checksum record: %v", checksumRecord)

		// TODO: continue implementation ...
		// - Compare with checksumResult with checkSumRecord.Checksum
		// - Not ok: update LastChecksumDate & LastChecksumSuccess (f) & Msg, PutChecksumRecord
		// - ok: update LastChecksumDate & Msg ("ok"), PutChecksumRecord, Schedule next check
	}

	return nil
}

func extractBucketAndObject(record events.DynamoDBEventRecord) (string, string, error) {
	bucketAttr, exists := record.Change.OldImage["Bucket"]
	if !exists {
		return "", "", fmt.Errorf("missing Bucket attribute")
	}

	objectAttr, exists := record.Change.OldImage["Object"]
	if !exists {
		return "", "", fmt.Errorf("missing Object attribute")
	}

	return bucketAttr.String(), objectAttr.String(), nil
}

func isTTLExpiry(record events.DynamoDBEventRecord) bool {
	return record.EventName == "REMOVE" &&
		record.UserIdentity != nil &&
		record.UserIdentity.Type == "Service" &&
		record.UserIdentity.PrincipalID == "dynamodb.amazonaws.com"
}

func main() {
	lambda.Start(handler)
}
