package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	managedBucketName string
	s3Client          *s3.Client
)

type ExportRecord struct {
	Item struct {
		BucketName          struct{ S string }  `json:"BucketName"`
		ObjectKey           struct{ S string }  `json:"ObjectKey"`
		Checksum            struct{ S string }  `json:"Checksum"`
		LastChecksumSuccess struct{ BOOL bool } `json:"LastChecksumSuccess"`
		LastChecksumDate    struct{ S string }  `json:"LastChecksumDate"`
		LastChecksumMessage struct{ S string }  `json:"LastChecksumMessage"`
	} `json:"Item"`
}

// S3Event represents an S3 event from Lambda
type S3Event struct {
	Records []struct {
		S3 struct {
			Bucket struct {
				Name string `json:"name"`
			} `json:"bucket"`
			Object struct {
				Key string `json:"key"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")
	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event json.RawMessage) error {
	var s3Event S3Event

	if err := json.Unmarshal(event, &s3Event); err != nil {
		return fmt.Errorf("failed to parse S3 event: %w", err)
	}

	if len(s3Event.Records) == 0 {
		return fmt.Errorf("no S3 records in event")
	}

	record := s3Event.Records[0]
	objectKey := record.S3.Object.Key

	log.Printf("Processing export file: %s", objectKey)

	// Extract export info from the object key
	exportArn := extractExportArnFromKey(objectKey)
	date := extractDateFromKey(objectKey)

	// Process the single file
	// TODO: handle empty files (0 bytes unzipped)
	csvData, err := getExportDataFile(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("failed to get export data for %s: %w", objectKey, err)
	}

	// TODO: add unique element to CSV filename if grouping by bucket
	fileId := extractFileID(objectKey)
	csvFilename := getCsvKey(fileId, date, exportArn)

	if err := writeCSVFile(ctx, csvFilename, csvData); err != nil {
		return fmt.Errorf("failed to write CSV for %s: %w", objectKey, err)
	}

	log.Printf("Successfully processed %s", objectKey)
	return nil
}

func extractFileID(key string) string {
	filename := path.Base(key)
	id := strings.TrimSuffix(filename, ".json.gz")
	return id
}

func extractExportArnFromKey(key string) string {
	// Extract from path like: exports/checksum-table/2025-08-25/AWSDynamoDB/01234567890123456789/data/file.json.gz
	parts := strings.Split(key, "/")
	if len(parts) >= 5 {
		return parts[4] // The export ARN part
	}
	return "unknown"
}

func extractDateFromKey(key string) string {
	// Extract from path like: exports/checksum-table/2025-08-25/AWSDynamoDB/...
	parts := strings.Split(key, "/")
	if len(parts) >= 3 {
		return parts[2] // The date part
	}
	return time.Now().Format("2006-01-02")
}

func getCsvKey(id string, date string, exportId string) string {
	return fmt.Sprintf("exports/checksum-table/%s/CSV/%s/export_%s.csv", date, exportId, id)
}

func getExportDataFile(ctx context.Context, key string) (output string, err error) {
	obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(managedBucketName),
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

	_ = w.Write([]string{
		"BucketName,ObjectKey,Checksum,LastChecksumSuccess,LastChecksumDate,LastChecksumMessage",
	})

	for {
		var rec ExportRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		_ = w.Write([]string{
			rec.Item.BucketName.S,
			rec.Item.ObjectKey.S,
			rec.Item.Checksum.S,
			strconv.FormatBool(rec.Item.LastChecksumSuccess.BOOL),
			rec.Item.LastChecksumDate.S,
			rec.Item.LastChecksumMessage.S,
		})
		w.Flush()
	}
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeCSVFile(ctx context.Context, key string, csv string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(managedBucketName),
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
