package inventory

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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const INVENTORY_FORMAT = "CSV"

// InventoryManifest represents the S3 Inventory manifest.json structure
type InventoryManifest struct {
	SourceBucket      string          `json:"sourceBucket"`
	DestinationBucket string          `json:"destinationBucket"`
	Version           string          `json:"version"`
	CreationTimestamp string          `json:"creationTimestamp"`
	FileFormat        string          `json:"fileFormat"`
	FileSchema        string          `json:"fileSchema"`
	Files             []InventoryFile `json:"files"`
}

// Bucket returns the bucket name used for inventory storage
func (m *InventoryManifest) Bucket() string {
	return parseDestinationBucket(m.DestinationBucket)
}

// Inventory returns an S3Object inventory location for upload
func (m *InventoryManifest) Inventory() files.S3Object {
	createdAt, _ := time.Parse(time.RFC3339, m.CreationTimestamp)
	d := createdAt.Format("2006-01-02")
	b := m.Bucket()
	o := filepath.Join("inventory", m.SourceBucket, "inventory", "data", fmt.Sprintf("inventory-%s.csv", d))
	return files.NewS3Object(b, o)
}

// ParseFileSchema parses the comma-separated schema string into a slice of header names
func (m *InventoryManifest) ParseFileSchema() []string {
	headers := strings.Split(m.FileSchema, ", ")
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}
	return headers
}

// InventoryFile represents a single inventory data file
type InventoryFile struct {
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	MD5Checksum string `json:"MD5checksum"`
}

// ConcatenateInventoryFiles downloads inventory CSV files, concatenates them with headers,
// and uploads to S3 as a plain CSV file.
func ConcatenateInventoryFiles(
	ctx context.Context, s3Client *s3.Client, manifestObj files.S3Object) error {

	manifest, err := GetInventoryManifest(ctx, s3Client, manifestObj)
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	if manifest.FileFormat != INVENTORY_FORMAT {
		return fmt.Errorf("unsupported format: %s", manifest.FileFormat)
	}

	log.Printf("Processing %d inventory files for bucket %s", len(manifest.Files), manifest.SourceBucket)

	tmpCSV, err := os.CreateTemp("", "inventory-*.csv")
	if err != nil {
		return fmt.Errorf("failed to create temp CSV: %w", err)
	}
	defer func() {
		_ = tmpCSV.Close()
		_ = os.Remove(tmpCSV.Name())
	}()

	headers := manifest.ParseFileSchema()
	csvWriter := csv.NewWriter(tmpCSV)
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}

	for _, file := range manifest.Files {
		mobj := files.NewS3Object(manifest.Bucket(), file.Key)
		reader, err := files.DownloadObject(ctx, s3Client, mobj, true)
		if err != nil {
			return fmt.Errorf("failed to download inventory file %s: %w", file.Key, err)
		}

		if _, err := io.Copy(tmpCSV, reader); err != nil {
			_ = reader.Close()
			return fmt.Errorf("failed to copy inventory file %s: %w", file.Key, err)
		}
		_ = reader.Close()
	}

	if _, err := tmpCSV.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek CSV file: %w", err)
	}

	err = files.UploadObject(ctx, s3Client, manifest.Inventory(), tmpCSV)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

func GetInventoryManifest(ctx context.Context, s3Client *s3.Client, obj files.S3Object) (*InventoryManifest, error) {
	manifestReader, err := files.DownloadObject(ctx, s3Client, obj, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = manifestReader.Close() }()

	var manifest InventoryManifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

func parseDestinationBucket(bucketRef string) string {
	// If it's an ARN format
	if len(bucketRef) > 13 && bucketRef[:13] == "arn:aws:s3:::" {
		return bucketRef[13:]
	}
	return bucketRef
}
