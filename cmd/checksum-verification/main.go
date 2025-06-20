package main

import (
	"context"
	"duracloud/internal/db"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"log"
	"os"
)

var dynamodbClient *dynamodb.Client

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	checksumTable := os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	//schedulerTable := os.Getenv("DYNAMODB_SCHEDULER_TABLE")

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

		checkSumRecord := db.ChecksumRecord{
			Bucket: bucket,
			Object: object,
		}
		checkSumRecord, err = db.GetChecksumRecord(ctx, dynamodbClient, checksumTable, checkSumRecord)
		if err != nil {
			return fmt.Errorf("failed to get checksum record: %w", err)
		}

		log.Printf("Checksum record: %v", checkSumRecord)

		// TODO: implementation ... (discuss error handling)
		// - Download object from s3
		// - If object not found delete from checksum table and continue
		// - Get checksumRecord (see above)
		// - If not found: create checksumRecord with LastChecksumSuccess (f), PutChecksumRecord
		// - Schedule next check now in case of transitory errors
		// - Calculate checksum of object
		// - Compare with checkSumRecord.Checksum
		// - If ok: update LastChecksumDate, PutChecksumRecord
		// - Not ok: update LastChecksumDate & LastChecksumSuccess (f), PutChecksumRecord
	}

	return nil
}

func main() {
	lambda.Start(handler)
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
