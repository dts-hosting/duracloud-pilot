package exports

import (
	"context"
	"duracloud/internal/files"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const ManifestFile = "manifest-files.json"

// This is an approx. overestimation of average csv row size (bytes)
const RowSizeEstimate = 180

var ExportHeaders = []string{
	"BucketName",
	"ObjectKey",
	"Checksum",
	"LastChecksumSuccess",
	"LastChecksumDate",
	"LastChecksumMessage",
}

type csvOutput struct {
	file   *os.File
	writer *csv.Writer
}

// Exporter provides utilites for processing an export manifest
type Exporter struct {
	ctx           context.Context
	s3Client      *s3.Client
	manifest      files.S3Object
	manifestFiles []string
	outputFiles   map[string]*csvOutput
	totalItems    int
}

func NewExporter(ctx context.Context, s3Client *s3.Client, manifest files.S3Object) *Exporter {
	return &Exporter{
		ctx:         ctx,
		s3Client:    s3Client,
		manifest:    manifest,
		outputFiles: make(map[string]*csvOutput),
	}
}

func (e *Exporter) ProcessManifest() error {
	manifest, err := files.DownloadObject(e.ctx, e.s3Client, e.manifest, false)
	if err != nil {
		return fmt.Errorf("failed to download manifest for %s: %w", e.manifest.Key, err)
	}
	defer func() { _ = manifest.Close() }()

	_, err = ProcessExport(manifest, func(rec *ManifestEntry) error {
		if rec.ItemCount > 0 {
			e.manifestFiles = append(e.manifestFiles, rec.DataFileS3Key)
		}
		e.totalItems += rec.ItemCount
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to process manifest: %w", err)
	}

	log.Printf(
		"Manifest processed: %s, Key: %s, Items: %d, EstimatedSize: %d",
		e.manifest.Bucket, e.manifest.Key, e.totalItems, e.totalItems*RowSizeEstimate,
	)

	// Process each file, abort if any fail
	err = e.processFiles()
	if err != nil {
		return err
	}

	// Cleanup before upload
	for _, output := range e.outputFiles {
		output.writer.Flush()
		if err := output.writer.Error(); err != nil {
			return fmt.Errorf("error flushing CSV writer: %w", err)
		}
		_ = output.file.Close()
	}

	// Upload each file, abort if any fail
	err = e.uploadFiles()
	if err != nil {
		return err
	}

	return nil
}

func (e *Exporter) processFiles() error {
	for _, manifestFile := range e.manifestFiles {
		log.Printf("Processing export: %s, Key: %s", e.manifest.Bucket, manifestFile)
		mobj := files.NewS3Object(e.manifest.Bucket, manifestFile)

		if err := e.processFile(mobj); err != nil {
			for _, output := range e.outputFiles {
				output.writer.Flush()
				_ = output.file.Close()
				_ = os.Remove(output.file.Name())
			}
			return fmt.Errorf("failed to process file %s: %w", manifestFile, err)
		}
	}
	return nil
}

func (e *Exporter) processFile(obj files.S3Object) error {
	file, err := files.DownloadObject(e.ctx, e.s3Client, obj, true)
	if err != nil {
		return fmt.Errorf("failed to download export data for %s: %w", obj.Key, err)
	}
	defer func() { _ = file.Close() }()

	_, err = ProcessExport(file, func(rec *ExportRecord) error {
		// Use the bucket name from the record as the key
		bucketName := rec.Item.BucketName.S
		output, ok := e.outputFiles[bucketName]

		if !ok {
			// Create new temp file and writer for this bucket
			csvFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.csv", bucketName))
			if err != nil {
				return fmt.Errorf("failed to create temp CSV file: %w", err)
			}

			csvWriter := csv.NewWriter(csvFile)
			if err := csvWriter.Write(ExportHeaders); err != nil {
				_ = csvFile.Close()
				_ = os.Remove(csvFile.Name())
				return fmt.Errorf("failed to write CSV headers: %w", err)
			}

			output = &csvOutput{
				file:   csvFile,
				writer: csvWriter,
			}
			e.outputFiles[bucketName] = output
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

func (e *Exporter) uploadFiles() error {
	defer func() {
		for _, output := range e.outputFiles {
			_ = os.Remove(output.file.Name())
		}
	}()

	date := time.Now().UTC().Format("2006-01-02")
	for bucket, output := range e.outputFiles {
		uploadFilename := filepath.Join("exports", "checksum-table", date, "CSV", fmt.Sprintf("%s.csv", bucket))
		tempFilePath := output.file.Name()

		uploadFile, err := os.Open(tempFilePath)
		if err != nil {
			return fmt.Errorf("failed to reopen CSV file: %w", err)
		}

		log.Printf("Uploading CSV Report: %s, Key: %s", e.manifest.Bucket, uploadFilename)

		uploadObj := files.NewS3Object(e.manifest.Bucket, uploadFilename)
		err = files.UploadObject(e.ctx, e.s3Client, uploadObj, uploadFile, "text/csv")
		_ = uploadFile.Close()

		if err != nil {
			return fmt.Errorf("failed to upload CSV for %s to %s: %w", bucket, uploadFilename, err)
		}

		log.Printf("Successfully wrote CSV Report to S3: %s, Key: %s", bucket, uploadFilename)
	}

	return nil
}

// ExportItem represents the fields in a DynamoDB export record
type ExportItem struct {
	BucketName          struct{ S string }  `json:"BucketName"`
	ObjectKey           struct{ S string }  `json:"ObjectKey"`
	Checksum            struct{ S string }  `json:"Checksum"`
	LastChecksumSuccess struct{ BOOL bool } `json:"LastChecksumSuccess"`
	LastChecksumDate    struct{ S string }  `json:"LastChecksumDate"`
	LastChecksumMessage struct{ S string }  `json:"LastChecksumMessage"`
}

// ExportRecord represents a single record from the exports table
type ExportRecord struct {
	Item ExportItem `json:"Item"`
}

// ToCSVRow returns the record values in the same order as ExportHeaders
func (r *ExportRecord) ToCSVRow() []string {
	return []string{
		r.Item.BucketName.S,
		r.Item.ObjectKey.S,
		r.Item.Checksum.S,
		strconv.FormatBool(r.Item.LastChecksumSuccess.BOOL),
		r.Item.LastChecksumDate.S,
		r.Item.LastChecksumMessage.S,
	}
}

// ManifestEntry represents the fields in a DynamoDB manifest
type ManifestEntry struct {
	ItemCount     int    `json:"itemCount"`
	DataFileS3Key string `json:"dataFileS3Key"`
}

// S3Bucket represents the bucket part of an S3 event
type S3Bucket struct {
	Name string `json:"name"`
}

// S3Object represents the object part of an S3 event
type S3Object struct {
	Key string `json:"key"`
}

// S3Data represents the S3 section of an event record
type S3Data struct {
	Bucket S3Bucket `json:"bucket"`
	Object S3Object `json:"object"`
}

// S3EventRecord represents a single S3 event record
type S3EventRecord struct {
	S3 S3Data `json:"s3"`
}

// S3Event represents an S3 event from Lambda
type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

func (e *S3Event) BucketName() string {
	return e.Records[0].S3.Bucket.Name
}

func (e *S3Event) ObjectKey() string {
	return e.Records[0].S3.Object.Key
}

// ExportTable triggers a DynamoDB table export to S3
func ExportTable(ctx context.Context, client *dynamodb.Client, tableArn string, obj files.S3Object) (string, error) {
	result, err := client.ExportTableToPointInTime(ctx, &dynamodb.ExportTableToPointInTimeInput{
		TableArn:     aws.String(tableArn),
		S3Bucket:     aws.String(obj.Bucket),
		S3Prefix:     aws.String(obj.Key),
		ExportFormat: "DYNAMODB_JSON",
	})
	if err != nil {
		return "", err
	}

	return *result.ExportDescription.ExportArn, nil
}

// ProcessExport processes JSON export data, calling callback for each record
// Returns the number of records processed
func ProcessExport[T ManifestEntry | ExportRecord](reader io.Reader, callback func(*T) error) (int, error) {
	dec := json.NewDecoder(reader)
	count := 0

	for {
		var rec T
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return count, err
		}

		if callback != nil {
			if err := callback(&rec); err != nil {
				return count, err
			}
		}
		count++
	}

	return count, nil
}
