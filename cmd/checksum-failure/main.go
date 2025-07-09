package main

import (
	"context"
	"duracloud/internal/accounts"
	"duracloud/internal/db"
	"duracloud/internal/notifications"
	_ "embed"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"log"
	"os"
	"text/template"
)

var (
	//go:embed templates/failure-notification.txt
	notificationTemplate string

	accountID        string
	notificationTmpl *template.Template
	snsClient        *sns.Client
	snsTopicArn      string
	stackName        string
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	accountID, err = accounts.GetAccountID(context.Background(), awsConfig)
	if err != nil {
		log.Fatalf("Unable to get AWS account ID: %v", err)
	}

	notificationTmpl, err = template.New("notification").Parse(notificationTemplate)
	if err != nil {
		log.Fatalf("Failed to parse notification template: %v", err)
	}

	snsClient = sns.NewFromConfig(awsConfig)
	snsTopicArn = os.Getenv("SNS_TOPIC_ARN")
	stackName = os.Getenv("STACK_NAME")
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		if record.Change.NewImage == nil {
			log.Printf("Skipping checksum record failure with unexpected no NewImage: %v", record)
			continue
		}

		bucket := record.Change.NewImage[string(db.ChecksumTableBucketNameId)].String()
		object := record.Change.NewImage[string(db.ChecksumTableObjectKeyId)].String()

		checksumSuccess, exists := record.Change.NewImage[string(db.ChecksumTableStatusId)]
		if !exists {
			log.Printf("Checksum status field not found for %s/%s", bucket, object)
			continue
		}

		success := checksumSuccess.Boolean()
		if success {
			log.Printf("Checksum status is ok for %s/%s", bucket, object)
			continue
		}

		log.Printf("Checksum failure detected: %s/%s", bucket, object)

		errorMessage := "Unknown error"
		if lastError, exists := record.Change.NewImage[string(db.ChecksumTableMessageId)]; exists {
			errorMessage = lastError.String()
		}

		// TODO: handle failure
		// - retry in case something unexpected happened?
		// - Upload report to s3 managed bucket?
		// - Send email?
		// - etc.

		notification := notifications.ChecksumFailureNotification{
			Account:      accountID,
			Bucket:       bucket,
			Object:       object,
			Date:         record.Change.ApproximateCreationDateTime.String(),
			ErrorMessage: errorMessage,
			Stack:        stackName,
			Title:        fmt.Sprintf("DuraCloud Checksum Verification Failure: %s/%s", bucket, object),
			Template:     notificationTmpl,
			Topic:        snsTopicArn,
		}

		if err := notifications.SendNotification(ctx, snsClient, notification); err != nil {
			log.Printf("Failed to send checksum failure notification: %v", err)
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
