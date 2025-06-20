package db

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"math/big"
	"time"
)

type ChecksumRecord struct {
	Bucket              string `dynamodbav:"Bucket"`
	Object              string `dynamodbav:"Object"`
	Checksum            string `dynamodbav:"Checksum"`
	LastChecksumDate    string `dynamodbav:"LastChecksumDate"`
	LastChecksumMessage string `dynamodbav:"LastChecksumMessage"`
	LastChecksumSuccess bool   `dynamodbav:"LastChecksumSuccess"`
	NextChecksumDate    string `dynamodbav:"NextChecksumDate"`
}

func DeleteChecksumRecord(
	ctx context.Context,
	client *dynamodb.Client,
	table string,
	record ChecksumRecord,
) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"Bucket": &types.AttributeValueMemberS{Value: record.Bucket},
			"Object": &types.AttributeValueMemberS{Value: record.Object},
		},
	})
	return err
}

func GetChecksumRecord(
	ctx context.Context,
	client *dynamodb.Client,
	checksumTable string,
	record ChecksumRecord,
) (ChecksumRecord, error) {
	checksumRecord := ChecksumRecord{}
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(checksumTable),
		Key: map[string]types.AttributeValue{
			"Bucket": &types.AttributeValueMemberS{Value: record.Bucket},
			"Object": &types.AttributeValueMemberS{Value: record.Object},
		},
	})
	if err != nil {
		return checksumRecord, err
	}

	if result.Item == nil {
		return ChecksumRecord{}, fmt.Errorf("checksum record not found")
	}

	err = attributevalue.UnmarshalMap(result.Item, &checksumRecord)
	if err != nil {
		return ChecksumRecord{}, fmt.Errorf("failed to unmarshal checksum record: %v", err)
	}

	return checksumRecord, nil
}

func GetNextScheduledTime() (time.Time, error) {
	baseTime := time.Now().AddDate(0, 5, 14)

	jitterDays, err := rand.Int(rand.Reader, big.NewInt(30))
	if err != nil {
		return baseTime, fmt.Errorf("failed to generate day jitter: %v", err)
	}

	jitterHours, err := rand.Int(rand.Reader, big.NewInt(24))
	if err != nil {
		return baseTime, fmt.Errorf("failed to generate hour jitter: %v", err)
	}

	jitterMinutes, err := rand.Int(rand.Reader, big.NewInt(60))
	if err != nil {
		return baseTime, fmt.Errorf("failed to generate minute jitter: %v", err)
	}

	scheduledTime := baseTime.
		AddDate(0, 0, int(jitterDays.Int64())).
		Add(time.Duration(jitterHours.Int64()) * time.Hour).
		Add(time.Duration(jitterMinutes.Int64()) * time.Minute)

	return scheduledTime, nil
}

func PutChecksumRecord(
	ctx context.Context,
	client *dynamodb.Client,
	checksumTable string,
	record ChecksumRecord,
) error {
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(checksumTable),
		Item: map[string]types.AttributeValue{
			"Bucket":              &types.AttributeValueMemberS{Value: record.Bucket},
			"Object":              &types.AttributeValueMemberS{Value: record.Object},
			"Checksum":            &types.AttributeValueMemberS{Value: record.Checksum},
			"LastChecksumDate":    &types.AttributeValueMemberS{Value: record.LastChecksumDate},
			"LastChecksumMessage": &types.AttributeValueMemberS{Value: record.LastChecksumMessage},
			"LastChecksumSuccess": &types.AttributeValueMemberBOOL{Value: record.LastChecksumSuccess},
		},
	})
	return err
}

func ScheduleNextVerification(
	ctx context.Context,
	client *dynamodb.Client,
	schedulerTable string,
	record ChecksumRecord,
	scheduledTime time.Time,
) error {
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(schedulerTable),
		Item: map[string]types.AttributeValue{
			"Bucket":           &types.AttributeValueMemberS{Value: record.Bucket},
			"Object":           &types.AttributeValueMemberS{Value: record.Object},
			"NextChecksumDate": &types.AttributeValueMemberS{Value: scheduledTime.Format(time.RFC3339)},
			"TTL":              &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", scheduledTime.Unix())},
		},
	})
	return err
}
