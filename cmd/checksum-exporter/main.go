package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"log"
	"os"
	"time"
)

var (
	dynamodbClient *dynamodb.Client
	checksumTable  string
	exportBucket   string
)

type ExportResponse struct {
	ExportArn string `json:"exportArn"`
	Message   string `json:"message"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
}

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	checksumTable = os.Getenv("DYNAMODB_CHECKSUM_TABLE")
	dynamodbClient = dynamodb.NewFromConfig(cfg)
	exportBucket = os.Getenv("S3_MANAGED_BUCKET")
}

func handler(ctx context.Context) (ExportResponse, error) {
	tableResult, err := dynamodbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(checksumTable),
	})
	if err != nil {
		return ExportResponse{}, fmt.Errorf("failed to describe table: %v", err)
	}

	tableArn := *tableResult.Table.TableArn
	timestamp := time.Now().Format("2006-01-02")
	prefix := fmt.Sprintf("exports/checksum-table/%s/", timestamp)

	exportArn, err := exportTable(ctx, dynamodbClient, tableArn, exportBucket, prefix)
	if err != nil {
		return ExportResponse{}, fmt.Errorf("failed to export checksum table: %v", err)
	}

	message := fmt.Sprintf("Monthly checksum export started: %s", exportArn)

	return ExportResponse{
		ExportArn: exportArn,
		Message:   message,
		Bucket:    exportBucket,
		Prefix:    prefix,
	}, nil
}

func exportTable(ctx context.Context, client *dynamodb.Client, tableArn, bucket, prefix string) (string, error) {
	result, err := client.ExportTableToPointInTime(ctx, &dynamodb.ExportTableToPointInTimeInput{
		TableArn:     aws.String(tableArn),
		S3Bucket:     aws.String(bucket),
		S3Prefix:     aws.String(prefix),
		ExportFormat: "DYNAMODB_JSON",
	})
	if err != nil {
		return "", err
	}

	return *result.ExportDescription.ExportArn, nil
}

func main() {
	lambda.Start(handler)
}
