package db

import (
	"context"
	"crypto/rand"
	"duracloud/internal/files"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"math/big"
	"time"
)

// ChecksumTableId represents a string type identifier for checksum table field names
type ChecksumTableId string

const (
	ChecksumTableBucketNameId ChecksumTableId = "BucketName"
	ChecksumTableObjectKeyId  ChecksumTableId = "ObjectKey"
	ChecksumTableMessageId    ChecksumTableId = "LastChecksumMessage"
	ChecksumTableStatusId     ChecksumTableId = "LastChecksumSuccess"
)

type ChecksumRecord struct {
	BucketName          string    `dynamodbav:"BucketName"`
	ObjectKey           string    `dynamodbav:"ObjectKey"`
	Checksum            string    `dynamodbav:"Checksum"`
	LastChecksumDate    time.Time `dynamodbav:"LastChecksumDate"`
	LastChecksumMessage string    `dynamodbav:"LastChecksumMessage"`
	LastChecksumSuccess bool      `dynamodbav:"LastChecksumSuccess"`
	NextChecksumDate    time.Time `dynamodbav:"NextChecksumDate"`
}

func DeleteChecksumRecord(
	ctx context.Context,
	client *dynamodb.Client,
	table string,
	obj files.S3Object,
) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BucketName": &types.AttributeValueMemberS{Value: obj.Bucket},
			"ObjectKey":  &types.AttributeValueMemberS{Value: obj.Key},
		},
	})
	return err
}

func GetChecksumRecord(
	ctx context.Context,
	client *dynamodb.Client,
	checksumTable string,
	obj files.S3Object,
) (ChecksumRecord, error) {
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(checksumTable),
		Key: map[string]types.AttributeValue{
			"BucketName": &types.AttributeValueMemberS{Value: obj.Bucket},
			"ObjectKey":  &types.AttributeValueMemberS{Value: obj.Key},
		},
	})
	if err != nil {
		return ChecksumRecord{}, err
	}

	checksumRecord := ChecksumRecord{}
	if result.Item == nil {
		return ChecksumRecord{}, ChecksumRecordNotFoundError(obj.Bucket, obj.Key)
	}

	err = attributevalue.UnmarshalMap(result.Item, &checksumRecord)
	if err != nil {
		return ChecksumRecord{}, UnmarshallingChecksumError(err)
	}

	return checksumRecord, nil
}

func GetNextScheduledTime() (time.Time, error) {
	baseTime := time.Now().AddDate(0, 5, 14)

	jitterDays, err := rand.Int(rand.Reader, big.NewInt(30))
	if err != nil {
		return baseTime, JitterGenerationError("day", err)
	}

	jitterHours, err := rand.Int(rand.Reader, big.NewInt(24))
	if err != nil {
		return baseTime, JitterGenerationError("hour", err)
	}

	jitterMinutes, err := rand.Int(rand.Reader, big.NewInt(60))
	if err != nil {
		return baseTime, JitterGenerationError("minute", err)
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
			"BucketName":          &types.AttributeValueMemberS{Value: record.BucketName},
			"ObjectKey":           &types.AttributeValueMemberS{Value: record.ObjectKey},
			"Checksum":            &types.AttributeValueMemberS{Value: record.Checksum},
			"LastChecksumDate":    &types.AttributeValueMemberS{Value: record.LastChecksumDate.Format(time.RFC3339)},
			"LastChecksumMessage": &types.AttributeValueMemberS{Value: record.LastChecksumMessage},
			"LastChecksumSuccess": &types.AttributeValueMemberBOOL{Value: record.LastChecksumSuccess},
			"NextChecksumDate":    &types.AttributeValueMemberS{Value: record.NextChecksumDate.Format(time.RFC3339)},
		},
	})
	return err
}

func ScheduleNextVerification(
	ctx context.Context,
	client *dynamodb.Client,
	schedulerTable string,
	record ChecksumRecord,
) error {
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(schedulerTable),
		Item: map[string]types.AttributeValue{
			"BucketName":       &types.AttributeValueMemberS{Value: record.BucketName},
			"ObjectKey":        &types.AttributeValueMemberS{Value: record.ObjectKey},
			"NextChecksumDate": &types.AttributeValueMemberS{Value: record.NextChecksumDate.Format(time.RFC3339)},
			"TTL":              &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", record.NextChecksumDate.Unix())},
		},
	})
	return err
}
