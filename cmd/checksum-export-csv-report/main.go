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
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// This is an approx. overestimation of average csv row size (bytes)
const ROW_SIZE_ESTIMATE = 180

var (
	s3Client *s3.Client
)

type csvOutput struct {
	file   *os.File
	writer *csv.Writer
}

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context, event json.RawMessage) error {
	var s3Event exports.S3Event
	var manifestFiles []string
	var totalItems int
	outputFiles := make(map[string]*csvOutput)

	if err := json.Unmarshal(event, &s3Event); err != nil {
		return fmt.Errorf("failed to parse S3 event: %w", err)
	}

	if len(s3Event.Records) == 0 {
		return fmt.Errorf("no S3 records in event")
	}

	bucketName := s3Event.BucketName()
	objectKey := s3Event.ObjectKey()

	if !strings.HasSuffix(objectKey, exports.ManifestFile) {
		return fmt.Errorf("invalid manifest file: %s", objectKey)
	}

	log.Printf("Processing manifest: %s, Key: %s", bucketName, objectKey)

	// Process the manifest and collect the files to process
	manifest, err := files.DownloadObject(ctx, s3Client, bucketName, objectKey, false)
	if err != nil {
		return fmt.Errorf("failed to download manifest for %s: %w", objectKey, err)
	}
	defer func() { _ = manifest.Close() }()

	_, err = exports.ProcessExport(manifest, func(rec *exports.ManifestEntry) error {
		if rec.ItemCount > 0 {
			manifestFiles = append(manifestFiles, rec.DataFileS3Key)
		}
		totalItems += rec.ItemCount
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to process manifest: %w", err)
	}

	log.Printf(
		"Manifest processed: %s, Key: %s, Items: %d, EstimatedSize: %d",
		bucketName, objectKey, totalItems, totalItems*ROW_SIZE_ESTIMATE,
	)

	// Process each file
	for _, manifestFile := range manifestFiles {
		log.Printf("Processing export: %s, Key: %s", bucketName, manifestFile)

		if err := processFile(ctx, bucketName, manifestFile, outputFiles); err != nil {
			for _, output := range outputFiles {
				output.writer.Flush()
				_ = output.file.Close()
				_ = os.Remove(output.file.Name())
			}
			return fmt.Errorf("failed to process file %s: %w", manifestFile, err)
		}
	}

	// Cleanup before upload
	for _, output := range outputFiles {
		output.writer.Flush()
		if err := output.writer.Error(); err != nil {
			return fmt.Errorf("error flushing CSV writer: %w", err)
		}
		_ = output.file.Close()
	}

	// Now upload the CSV report files
	date := time.Now().UTC().Format("2006-01-02")
	for bucket, output := range outputFiles {
		uploadFilename := fmt.Sprintf("exports/checksum-table/%s/CSV/%s.csv", date, bucket)
		tempFilePath := output.file.Name()

		uploadFile, err := os.Open(tempFilePath)
		if err != nil {
			return fmt.Errorf("failed to reopen CSV file: %w", err)
		}

		log.Printf("Uploading CSV Report: %s, Key: %s", bucketName, uploadFilename)

		input := &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(uploadFilename),
			Body:   uploadFile,
		}

		_, err = s3Client.PutObject(ctx, input)
		_ = uploadFile.Close()

		if err != nil {
			return fmt.Errorf("failed to upload CSV for %s to %s: %w", bucket, uploadFilename, err)
		}

		_ = os.Remove(tempFilePath)
		log.Printf("Successfully wrote CSV Report to S3: %s, Key: %s", bucket, uploadFilename)
	}

	log.Printf("Successfully processed manifest: %s, Key: %s", bucketName, objectKey)

	return nil
}

func main() {
	lambda.Start(handler)
}

func processFile(ctx context.Context, bucket string, object string, out map[string]*csvOutput) error {
	file, err := files.DownloadObject(ctx, s3Client, bucket, object, true)
	if err != nil {
		return fmt.Errorf("failed to download export data for %s: %w", object, err)
	}
	defer func() { _ = file.Close() }()

	_, err = exports.ProcessExport(file, func(rec *exports.ExportRecord) error {
		// Use the bucket name from the record as the key
		bucketName := rec.Item.BucketName.S
		output, ok := out[bucketName]

		if !ok {
			// Create new temp file and writer for this bucket
			csvFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.csv", bucketName))
			if err != nil {
				return fmt.Errorf("failed to create temp CSV file: %w", err)
			}

			csvWriter := csv.NewWriter(csvFile)
			if err := csvWriter.Write(exports.ExportHeaders); err != nil {
				_ = csvFile.Close()
				_ = os.Remove(csvFile.Name())
				return fmt.Errorf("failed to write CSV headers: %w", err)
			}

			output = &csvOutput{
				file:   csvFile,
				writer: csvWriter,
			}
			out[bucketName] = output
		}

		if err := output.writer.Write(rec.ToCSVRow()); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error during processing of export file: %w", err)
	}

	return nil
}
