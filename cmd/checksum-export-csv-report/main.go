package main

import (
	"compress/gzip"
	"context"
	"duracloud/internal/exports"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

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

	// Process the single file
	// TODO: handle empty files (0 bytes unzipped)
	csvData, err := getExportDataFile(ctx, bucketName, objectKey)
	if err != nil {
		return fmt.Errorf("failed to get export data for %s: %w", objectKey, err)
	}

	// TODO: add unique element to CSV filename if grouping by bucket
	csvFilename := getCsvKey(fileId, date, exportArn)

	if err := writeCSVFile(ctx, bucketName, csvFilename, csvData); err != nil {
		return fmt.Errorf("failed to write CSV for %s: %w", objectKey, err)
	}

	log.Printf("Successfully processed %s", objectKey)
	return nil
}

func getCsvKey(id string, date string, exportId string) string {
	return fmt.Sprintf("exports/checksum-table/%s/CSV/%s/export_%s.csv", date, exportId, id)
}

func getExportDataFile(ctx context.Context, bucket string, key string) (output string, err error) {
	obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get object: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(obj.Body)

	gzr, err := gzip.NewReader(obj.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func(gzr *gzip.Reader) {
		_ = gzr.Close()
	}(gzr)

	dec := json.NewDecoder(gzr)

	var b strings.Builder
	w := csv.NewWriter(&b)

	_ = w.Write(exports.ExportHeaders)

	for {
		var rec exports.ExportRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		_ = w.Write(rec.ToCSVRow())
		w.Flush()
	}
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeCSVFile(ctx context.Context, bucket string, key string, csv string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(csv),
	}
	_, err := s3Client.PutObject(ctx, input)

	if err != nil {
		return fmt.Errorf("failed to write CSV Report to S3: %w", err)
	}
	log.Printf("Successfully wrote CSV Report at %s to S3", key)
	return nil
}

func main() {
	lambda.Start(handler)
}
