package main

import (
	"context"
	"duracloud/internal/accounts"
	"duracloud/internal/checksum"
	"duracloud/internal/db"
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
	ddb := db.NewDB(ctx, dynamodbClient, checksumTable, schedulerTable)

	for _, record := range event.Records {
		if !db.IsTTLExpiry(record) {
			continue
		}

		obj, err := db.ExtractBucketAndObject(record)
		if err != nil {
			log.Printf("Failed to extract bucket/object: %s", err.Error())
			continue
		}

		verifier := checksum.NewVerifier(ctx, ddb, s3Client, obj)
		ok, err := verifier.Verify()
		if err != nil {
			// This indicates we failed to access or update the database or schedule the next check
			log.Printf("Failure processing checksum verification: %s", err.Error())
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
		}

		if ok {
			log.Printf("Checksum verification successful: %s", obj.URI())
		} else {
			log.Printf("Checksum verification failed: %s", obj.URI())
			notification := notifications.ChecksumFailureNotification{
				Account:      accountID,
				Bucket:       obj.Bucket,
				Object:       obj.Key,
				Date:         record.Change.ApproximateCreationDateTime.String(),
				ErrorMessage: "First notice of checksum verification failure.",
				Stack:        stackName,
				Title:        fmt.Sprintf("DuraCloud Checksum Verification Failure (1): %s", obj.URI()),
				Template:     notificationTmpl,
				Topic:        snsTopicArn,
			}

			if err := notifications.SendNotification(ctx, snsClient, notification); err != nil {
				log.Printf("Failed to send checksum failure notification: %v", err)
			}
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
