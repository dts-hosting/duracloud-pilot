package main

import (
	"context"
	"duracloud/internal/accounts"
	"duracloud/internal/checksum"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"duracloud/internal/notifications"
	_ "embed"
	"fmt"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

var (
	//go:embed templates/failure-notification.txt
	notificationTemplate string

	accountID        string
	checksumTable    string
	dynamodbClient   *dynamodb.Client
	notificationTmpl *template.Template
	s3Client         *s3.Client
	schedulerTable   string
	snsClient        *sns.Client
	snsTopicArn      string
	stackName        string
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

	accountID, err = accounts.GetAccountID(context.Background(), awsConfig)
	if err != nil {
		panic(fmt.Sprintf("Unable to get AWS account ID: %v", err))
	}

	notificationTmpl, err = template.New("notification").Parse(notificationTemplate)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse notification template: %v", err))
	}

	checksumTable = os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	dynamodbClient = dynamodb.NewFromConfig(awsConfig)
	s3Client = s3.NewFromConfig(awsConfig)
	schedulerTable = os.Getenv("DYNAMODB_SCHEDULER_TABLE")
	snsClient = sns.NewFromConfig(awsConfig)
	snsTopicArn = os.Getenv("SNS_TOPIC_ARN")
	stackName = os.Getenv("STACK_NAME")
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
			// This isn't about whether the checksum verification succeeded or failed,
			// rather it indicates we failed to access or update the database about it or schedule the next check
			log.Printf("failure processing checksum verification: %s", err.Error())
			notification := notifications.ChecksumFailureNotification{
				Account:      accountID,
				Bucket:       obj.Bucket,
				Object:       obj.Key,
				Date:         time.Now().Format(time.RFC3339),
				ErrorMessage: err.Error(),
				Stack:        stackName,
				Title:        fmt.Sprintf("DuraCloud Checksum Processing Failure: %s/%s", obj.Bucket, obj.Key),
				Template:     notificationTmpl,
				Topic:        snsTopicArn,
			}

			if err := notifications.SendNotification(ctx, snsClient, notification); err != nil {
				log.Printf("Failed to send checksum failure notification: %v", err)
			}
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

	calc := checksum.NewS3Calculator(s3Client)
	checksumResult, err := calc.CalculateChecksum(ctx, obj)
	if err != nil {
		checksumRecord.LastChecksumMessage = err.Error()
		checksumRecord.LastChecksumSuccess = false
	} else if checksumResult != checksumRecord.Checksum {
		msg := fmt.Sprintf("Checksum mismatch: calculated=%s, stored=%s", checksumResult, checksumRecord.Checksum)
		log.Println(msg)
		checksumRecord.LastChecksumMessage = msg
		checksumRecord.LastChecksumSuccess = false
	} else {
		// Technically this is redundant but included for clarity
		checksumRecord.LastChecksumMessage = "ok"
		checksumRecord.LastChecksumSuccess = true
	}

	err = db.PutChecksumRecord(ctx, dynamodbClient, checksumTable, checksumRecord)
	if err != nil {
		log.Printf("Failed to update checksum record due to : %v", err)
		return err
	}

	if checksumRecord.LastChecksumSuccess {
		log.Printf("Checksum verification succeeded for: %s/%s", obj.Bucket, obj.Key)
		err = db.ScheduleNextVerification(ctx, dynamodbClient, schedulerTable, checksumRecord)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
