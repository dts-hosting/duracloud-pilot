package db

import (
	"context"
	"crypto/rand"
	"duracloud/internal/files"
	"fmt"
	"math/big"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

type DB struct {
	ctx            context.Context
	client         *dynamodb.Client
	checksumTable  string
	schedulerTable string
}

func NewDB(ctx context.Context, client *dynamodb.Client, checksumTable, schedulerTable string) *DB {
	return &DB{
		ctx:            ctx,
		client:         client,
		checksumTable:  checksumTable,
		schedulerTable: schedulerTable,
	}
}

func (d *DB) Delete(obj files.S3Object) error {
	err := d.delete(d.checksumTable, obj)
	if err != nil {
		return err
	}

	return d.delete(d.schedulerTable, obj)
}

func (d *DB) delete(table string, obj files.S3Object) error {
	_, err := d.client.DeleteItem(d.ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BucketName": &types.AttributeValueMemberS{Value: obj.Bucket},
			"ObjectKey":  &types.AttributeValueMemberS{Value: obj.Key},
		},
	})
	return err
}

func (d *DB) Get(obj files.S3Object) (ChecksumRecord, error) {
	return d.get(d.checksumTable, obj)
}

func (d *DB) get(table string, obj files.S3Object) (ChecksumRecord, error) {
	result, err := d.client.GetItem(d.ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
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
		return ChecksumRecord{}, ErrorChecksumRecordNotFound(obj.Bucket, obj.Key)
	}

	err = attributevalue.UnmarshalMap(result.Item, &checksumRecord)
	if err != nil {
		return ChecksumRecord{}, ErrorUnmarshallingChecksum(err)
	}

	return checksumRecord, nil
}

func (d *DB) Next(obj files.S3Object) (ChecksumRecord, error) {
	return d.get(d.schedulerTable, obj)
}

func (d *DB) Put(record ChecksumRecord) error {
	_, err := d.client.PutItem(d.ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.checksumTable),
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

func (d *DB) Schedule(record ChecksumRecord) error {
	_, err := d.client.PutItem(d.ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.schedulerTable),
		Item: map[string]types.AttributeValue{
			"BucketName":       &types.AttributeValueMemberS{Value: record.BucketName},
			"ObjectKey":        &types.AttributeValueMemberS{Value: record.ObjectKey},
			"NextChecksumDate": &types.AttributeValueMemberS{Value: record.NextChecksumDate.Format(time.RFC3339)},
			"TTL":              &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", record.NextChecksumDate.Unix())},
		},
	})
	return err
}

func GetNextScheduledTime() (time.Time, error) {
	baseTime := time.Now().AddDate(0, 5, 14)

	jitterDays, err := rand.Int(rand.Reader, big.NewInt(30))
	if err != nil {
		return baseTime, ErrorGeneratingJitter("day", err)
	}

	jitterHours, err := rand.Int(rand.Reader, big.NewInt(24))
	if err != nil {
		return baseTime, ErrorGeneratingJitter("hour", err)
	}

	jitterMinutes, err := rand.Int(rand.Reader, big.NewInt(60))
	if err != nil {
		return baseTime, ErrorGeneratingJitter("minute", err)
	}

	scheduledTime := baseTime.
		AddDate(0, 0, int(jitterDays.Int64())).
		Add(time.Duration(jitterHours.Int64()) * time.Hour).
		Add(time.Duration(jitterMinutes.Int64()) * time.Minute)

	return scheduledTime, nil
}
