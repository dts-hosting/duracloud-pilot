package main

import (
	"context"
	"duracloud/internal/exports"
	"duracloud/internal/files"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	s3Client *s3.Client
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	s3Client = s3.NewFromConfig(awsConfig)
}

// TODO: one file in, one file out. However we may want to create
// 1 file per bucket in the future if these export files are large.
func handler(ctx context.Context, event json.RawMessage) error {
	var s3Event exports.S3Event

	if err := json.Unmarshal(event, &s3Event); err != nil {
		return fmt.Errorf("failed to parse S3 event: %w", err)
	}

	if len(s3Event.Records) == 0 {
		return fmt.Errorf("no S3 records in event")
	}

	bucketName := s3Event.BucketName()
	objectKey := s3Event.ObjectKey()
	fileId := s3Event.FileId()

	log.Printf("Bucket: %s, Key: %s, Id: %s", bucketName, objectKey, fileId)

	exportArn, err := s3Event.ObjectExportArn()
	if err != nil {
		return fmt.Errorf("failed to extract export ARN: %w", err)
	}

	date, err := s3Event.ObjectDate()
	if err != nil {
		return fmt.Errorf("failed to extract date: %w", err)
	}

	log.Printf("Export ARN: %s, Date: %s", exportArn, date)

	reader, err := files.DownloadObject(ctx, s3Client, bucketName, objectKey, true)
	if err != nil {
		return fmt.Errorf("failed to download export data for %s: %w", objectKey, err)
	}
	defer func() { _ = reader.Close() }()

	csvFile, err := os.CreateTemp("", "export-*.csv")
	if err != nil {
		return fmt.Errorf("failed to create temp CSV file: %w", err)
	}
	defer func() { _ = os.Remove(csvFile.Name()) }()

	csvWriter := csv.NewWriter(csvFile)
	if err := csvWriter.Write(exports.ExportHeaders); err != nil {
		_ = csvFile.Close()
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	recordCount, err := exports.ProcessExport(reader, func(rec *exports.ExportRecord) error {
		if err := csvWriter.Write(rec.ToCSVRow()); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
		csvWriter.Flush()
		return csvWriter.Error()
	})

	if err != nil {
		_ = csvFile.Close()
		return fmt.Errorf("failed to process export data for %s: %w", objectKey, err)
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		_ = csvFile.Close()
		return fmt.Errorf("CSV writer error: %w", err)
	}

	if recordCount == 0 {
		log.Printf("Empty file processed: %s (headers only)", objectKey)
	} else {
		log.Printf("Processed %d records from %s", recordCount, objectKey)
	}

	csvFilename := fmt.Sprintf("exports/checksum-table/%s/CSV/%s/export_%s.csv", date, exportArn, fileId)

	// Close the write handle before reopening for read
	_ = csvFile.Close()

	uploadFile, err := os.Open(csvFile.Name())
	if err != nil {
		return fmt.Errorf("failed to reopen CSV file: %w", err)
	}
	defer func() { _ = uploadFile.Close() }()

	input := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(csvFilename),
		Body:   uploadFile,
	}

	_, err = s3Client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload CSV for %s: %w", objectKey, err)
	}

	log.Printf("Successfully wrote CSV Report at %s to S3", csvFilename)
	log.Printf("Successfully processed %s", objectKey)
	return nil
}

func main() {
	lambda.Start(handler)
}
