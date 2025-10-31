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

const InventoryFormat = "CSV"
const S3ArnLength = "arn:aws:s3:::"

// InventoryUnwrapper provides utilites for processing an inventory manifest
type InventoryUnwrapper struct {
	ctx      context.Context
	s3Client *s3.Client
	manifest files.S3Object
}

func NewInventoryUnwrapper(ctx context.Context, s3Client *s3.Client, manifest files.S3Object) *InventoryUnwrapper {
	return &InventoryUnwrapper{
		ctx:      ctx,
		s3Client: s3Client,
		manifest: manifest,
	}
}

// ConcatenateInventoryFiles downloads inventory CSV files, concatenates them with headers,
// and uploads to S3 as a plain CSV file.
func (i *InventoryUnwrapper) ConcatenateInventoryFiles() error {
	manifest, err := i.GetManifest()
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	if manifest.FileFormat != InventoryFormat {
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

	err = i.writeCSVHeaders(tmpCSV, manifest.ParseFileSchema())
	if err != nil {
		return err
	}

	for _, file := range manifest.Files {
		err := i.processFile(files.NewS3Object(manifest.Bucket(), file.Key), tmpCSV)
		if err != nil {
			return err
		}
	}

	if _, err := tmpCSV.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek CSV file: %w", err)
	}

	err = files.UploadObject(i.ctx, i.s3Client, manifest.Inventory(), tmpCSV, "text/csv")
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

func (i *InventoryUnwrapper) GetManifest() (*InventoryManifest, error) {
	manifestReader, err := files.DownloadObject(i.ctx, i.s3Client, i.manifest, false)
	if err != nil {
		return nil, fmt.Errorf("failed to download manifest: %w", err)
	}
	defer func() { _ = manifestReader.Close() }()

	var manifest InventoryManifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

func (i *InventoryUnwrapper) processFile(obj files.S3Object, f *os.File) error {
	reader, err := files.DownloadObject(i.ctx, i.s3Client, obj, true)
	if err != nil {
		return fmt.Errorf("failed to download inventory file %s: %w", obj.Key, err)
	}

	defer func() { _ = reader.Close() }()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("failed to copy inventory file %s: %w", obj.Key, err)
	}

	return nil
}

func (i *InventoryUnwrapper) writeCSVHeaders(f *os.File, headers []string) error {
	csvWriter := csv.NewWriter(f)
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}
	return nil
}

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
	return m.parseDestinationBucket()
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

func (m *InventoryManifest) parseDestinationBucket() string {
	// If it's an ARN format
	dest := m.DestinationBucket
	arnLength := len(S3ArnLength)
	if len(dest) > arnLength && dest[:arnLength] == S3ArnLength {
		return dest[arnLength:]
	}
	return dest
}

// InventoryFile represents a single inventory data file
type InventoryFile struct {
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	MD5Checksum string `json:"MD5checksum"`
}
