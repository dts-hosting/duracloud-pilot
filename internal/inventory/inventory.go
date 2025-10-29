package inventory

import (
	"context"
	"duracloud/internal/files"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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

func GetInventoryManifest(ctx context.Context, s3Client *s3.Client, obj files.S3Object) (*InventoryManifest, error) {
	manifestReader, err := files.DownloadObject(ctx, s3Client, obj.Bucket, obj.Key, false)
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
