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
const InventoryMinElements = 7
const ManifestFile = "manifest.json"
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

// CollectStats reads the inventory CSV and collects statistics by prefix
func (i *InventoryUnwrapper) CollectStats(csvReader io.Reader, manifest *InventoryManifest) (*InventoryStats, error) {
	stats := &InventoryStats{
		BucketName:          manifest.SourceBucket,
		InventoryDate:       time.Now().UTC().Format("2006-01-02"),
		GeneratedAt:         time.Now().UTC(),
		SourceInventoryDate: manifest.CreationTimestamp,
		PrefixStats:         make(map[string]PrefixStats),
	}

	csvParser := csv.NewReader(csvReader)

	_, err := csvParser.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	const (
		keyIndex            = 1
		isLatestIndex       = 3
		isDeleteMarkerIndex = 4
		sizeIndex           = 5
	)

	recordCount := 0
	for {
		line := recordCount + 2
		record, err := csvParser.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record at line %d: %w", line, err)
		}

		if len(record) < InventoryMinElements {
			log.Printf("Warning: skipping malformed record at line %d", line)
			continue
		}

		isLatest := record[isLatestIndex]
		isDeleteMarker := record[isDeleteMarkerIndex]

		if isLatest != "true" || isDeleteMarker == "true" {
			continue
		}

		var size int64
		if _, err := fmt.Sscanf(record[sizeIndex], "%d", &size); err != nil {
			log.Printf("Warning: failed to parse size at line %d: %v", line, err)
			continue
		}

		key := record[keyIndex]
		prefix := extractTopLevelPrefix(key)

		stats.TotalCount++
		stats.TotalBytes += size

		prefixStat := stats.PrefixStats[prefix]
		prefixStat.Count++
		prefixStat.Bytes += size
		stats.PrefixStats[prefix] = prefixStat

		recordCount++
		if recordCount%100000 == 0 {
			log.Printf("Processed %d records, total size: %.2f GB",
				recordCount, float64(stats.TotalBytes)/(1024*1024*1024))
		}
	}

	log.Printf("Stats collection complete: %d objects, %.2f GB, %d prefixes",
		stats.TotalCount, float64(stats.TotalBytes)/(1024*1024*1024), len(stats.PrefixStats))

	return stats, nil
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

// ProcessInventoryFiles downloads inventory CSV files, concatenates them with headers,
// generates stats and uploads to S3 as a plain CSV file and stats json.
func (i *InventoryUnwrapper) ProcessInventoryFiles() error {
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

	log.Printf("Collecting statistics for bucket %s", manifest.SourceBucket)
	if _, err := tmpCSV.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek CSV file for stats: %w", err)
	}

	stats, err := i.CollectStats(tmpCSV, manifest)
	if err != nil {
		return fmt.Errorf("failed to collect stats: %w", err)
	}

	err = i.UploadStats(stats, manifest)
	if err != nil {
		return fmt.Errorf("failed to upload stats: %w", err)
	}

	return nil
}

// UploadStats uploads the collected statistics to S3
func (i *InventoryUnwrapper) UploadStats(stats *InventoryStats, manifest *InventoryManifest) error {
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	// Build the stats file path: inventory/{sourceBucket}/inventory/stats/stats-{date}.json
	statsKey := filepath.Join("inventory", manifest.SourceBucket, "inventory", "stats",
		fmt.Sprintf("stats-%s.json", stats.InventoryDate))

	statsObj := files.NewS3Object(manifest.Bucket(), statsKey)

	err = files.UploadObject(i.ctx, i.s3Client, statsObj, strings.NewReader(string(statsJSON)), "application/json")
	if err != nil {
		return fmt.Errorf("failed to upload stats to S3: %w", err)
	}

	log.Printf("Stats uploaded to s3://%s/%s", statsObj.Bucket, statsObj.Key)
	return nil
}

// extractTopLevelPrefix extracts the top-level prefix (first directory) from a key
// Examples:
//
//	"logs/2023/file.txt" -> "logs/"
//	"file.txt" -> "" (root)
func extractTopLevelPrefix(key string) string {
	idx := strings.Index(key, "/")
	if idx == -1 {
		return "" // No prefix (root level)
	}
	return key[:idx+1]
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

// InventoryFile represents a single inventory data file
type InventoryFile struct {
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	MD5Checksum string `json:"MD5checksum"`
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
	d := time.Now().UTC().Format("2006-01-02")
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

// InventoryStats represents aggregated statistics from inventory data
type InventoryStats struct {
	BucketName          string                 `json:"bucket_name"`
	InventoryDate       string                 `json:"inventory_date"`
	GeneratedAt         time.Time              `json:"generated_at"`
	SourceInventoryDate string                 `json:"source_inventory_date"`
	TotalCount          int64                  `json:"total_count"`
	TotalBytes          int64                  `json:"total_bytes"`
	PrefixStats         map[string]PrefixStats `json:"prefix_stats"`
}

// PrefixStats represents aggregated statistics for a single prefix
type PrefixStats struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}
