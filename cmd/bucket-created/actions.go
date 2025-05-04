package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func enableEventBridgeNotifications(ctx context.Context, bucketName string) error {
	s3Client := s3.NewFromConfig(awsConfig)

	_, err := s3Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket: aws.String(bucketName),
		NotificationConfiguration: &types.NotificationConfiguration{
			EventBridgeConfiguration: &types.EventBridgeConfiguration{},
		},
	})

	if err != nil {
		return err
	}

	log.Printf("Successfully enabled EventBridge notifications for bucket %s", bucketName)
	return nil
}
