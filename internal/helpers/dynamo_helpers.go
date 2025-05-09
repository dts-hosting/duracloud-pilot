package helpers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type ChecksumRecord struct {
	ObjectId            string `dynamodbav:"ObjectId"`
	ChecksumPurpose     string `dynamodbav:"ChecksumPurpose"`
	LastChecksumDate    string `dynamodbav:"LastChecksumDate"`
	LastChecksumSuccess bool   `dynamodbav:"LastChecksumSuccess"`
	Checksum            string `dynamodbav:"Checksum"`
}

// GetRecordsNeedingVerification retrieve records where last checksum date is older than 6 months
func GetRecordsNeedingVerification(ctx context.Context, client *dynamodb.Client, tableName string, batchSize int) ([]ChecksumRecord, map[string]types.AttributeValue, error) {
	cutoffDate := time.Now().AddDate(0, -6, 0).Format(time.RFC3339)

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ChecksumDateIndex"),
		KeyConditionExpression: aws.String("ChecksumPurpose = :purpose AND LastChecksumDate < :cutoffDate"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":purpose":    &types.AttributeValueMemberS{Value: "VERIFICATION"},
			":cutoffDate": &types.AttributeValueMemberS{Value: cutoffDate},
		},
		Limit: aws.Int32(int32(batchSize)),
	}

	resp, err := client.Query(ctx, queryInput)
	if err != nil {
		return nil, nil, fmt.Errorf("query failed: %v", err)
	}

	if len(resp.Items) == 0 {
		return []ChecksumRecord{}, nil, nil
	}

	var records []ChecksumRecord
	err = attributevalue.UnmarshalListOfMaps(resp.Items, &records)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	return records, resp.LastEvaluatedKey, nil
}

// ProcessChecksumVerifications passes records to a processor function
// The processor function is called for each record that needs verification
// which can then be processed inline or pushed to a queue to hand-off
func ProcessChecksumVerifications(
	ctx context.Context,
	dynamoClient *dynamodb.Client,
	tableName string,
	batchSize int,
	processor func(ctx context.Context, record ChecksumRecord) error,
) error {
	var lastEvaluatedKey map[string]types.AttributeValue
	processedCount := 0
	errorCount := 0

	for {
		records, nextKey, err := GetRecordsNeedingVerification(ctx, dynamoClient, tableName, batchSize)
		if err != nil {
			return fmt.Errorf("failed to get records: %v", err)
		}

		if len(records) == 0 {
			break
		}

		for _, record := range records {
			if err := processor(ctx, record); err != nil {
				fmt.Printf("Error processing record %s: %v\n", record.ObjectId, err)
				errorCount++
			}
			processedCount++
		}

		fmt.Printf("Processed %d records so far (%d errors)\n", processedCount, errorCount)

		lastEvaluatedKey = nextKey
		if lastEvaluatedKey == nil {
			break // No more records to process
		}
	}

	fmt.Printf("Completed processing. Processed %d records with %d errors.\n",
		processedCount, errorCount)
	return nil
}

func UpdateChecksumVerification(ctx context.Context, client *dynamodb.Client, tableName string,
	objectId string, checksumSuccess bool) error {
	now := time.Now().Format(time.RFC3339)

	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"ObjectId": &types.AttributeValueMemberS{Value: objectId},
		},
		UpdateExpression: aws.String("SET LastChecksumDate = :date, LastChecksumSuccess = :success"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":date":    &types.AttributeValueMemberS{Value: now},
			":success": &types.AttributeValueMemberBOOL{Value: checksumSuccess},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to update verification: %v", err)
	}

	return nil
}
