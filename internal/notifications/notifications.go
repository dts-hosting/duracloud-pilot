package notifications

import (
	"bytes"
	"context"
	"log"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSNotification represents an abstraction for a notification to be published via AWS SNS.
type SNSNotification interface {
	Message() (string, error)
	Subject() string
	TopicArn() string
}

type ChecksumFailureNotification struct {
	Account      string
	Bucket       string
	Object       string
	Date         string
	ErrorMessage string
	Stack        string
	Title        string
	Template     *template.Template
	Topic        string
}

func (n ChecksumFailureNotification) Message() (string, error) {
	var buf bytes.Buffer
	if err := n.Template.Execute(&buf, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (n ChecksumFailureNotification) Subject() string {
	return n.Title
}

func (n ChecksumFailureNotification) TopicArn() string {
	return n.Topic
}

func SendNotification(ctx context.Context, client *sns.Client, notification SNSNotification) error {
	message, err := notification.Message()
	if err != nil {
		return err
	}

	result, err := client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(notification.TopicArn()),
		Subject:  aws.String(notification.Subject()),
		Message:  aws.String(message),
	})
	if err != nil {
		return err
	}

	log.Printf("Notification sent successfully: %s", *result.MessageId)
	return nil

}
