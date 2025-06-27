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

		log.Printf("Starting checksum verification for: %s/%s", obj.Bucket, obj.Key)

		//currentTime := time.Now()
		//nextScheduledTime := db.GetNextScheduledTime()

		checksumRecord, err := db.GetChecksumRecord(ctx, dynamodbClient, checksumTable, obj)
		if err != nil {
			// TODO update checksumRecord for failure and PutChecksumRecord (continue)
			log.Printf("failed to get checksum record: %s", err.Error())
			continue
		}

		calc := checksum.NewS3Calculator(s3Client)
		checksumResult, err := calc.CalculateChecksum(ctx, obj)
		if err != nil {
			// TODO update checksumRecord for failure and PutChecksumRecord (continue)
			log.Printf("failed to calculate checksum: %s", err.Error())
			continue
		}

		// TODO: remove these temporary logging statements
		log.Printf("Calculated checksum: %s", checksumResult)
		log.Printf("Checksum record: %v", checksumRecord)

		// TODO: continue implementation ...
		// - Compare with checksumResult with checkSumRecord.Checksum
		// - Not ok: update LastChecksumDate & LastChecksumSuccess (f) & Msg, PutChecksumRecord
		// - ok: update LastChecksumDate & Msg ("ok"), PutChecksumRecord, Schedule next check
	}

	return nil
}

func extractBucketAndObject(record events.DynamoDBEventRecord) (checksum.S3Object, error) {
	bucket, exists := record.Change.OldImage[string(db.ChecksumTableBucketNameId)]
	if !exists {
		return checksum.S3Object{}, fmt.Errorf("missing bucket name attribute")
	}

	object, exists := record.Change.OldImage[string(db.ChecksumTableObjectKeyId)]
	if !exists {
		return checksum.S3Object{}, fmt.Errorf("missing object key attribute")
	}

	return checksum.S3Object{Bucket: bucket.String(), Key: object.String()}, nil
}

func isTTLExpiry(record events.DynamoDBEventRecord) bool {
	return record.EventName == "REMOVE" &&
		record.UserIdentity != nil &&
		record.UserIdentity.Type == "Service" &&
		record.UserIdentity.PrincipalID == "dynamodb.amazonaws.com"
}

//func processChecksumVerification(ctx context.Context, s3Client *s3.Client, dynamodbClient *dynamodb.Client, obj checksum.S3Object) error {
//	// TODO
//	return nil
//}

func main() {
	lambda.Start(handler)
}
