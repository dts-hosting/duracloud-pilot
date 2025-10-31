package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInventoryManifest_ParseFileSchema(t *testing.T) {
	// Load the fixture
	fixturePath := filepath.Join("..", "..", "files", "inventory-manifest.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	// Parse the manifest
	var manifest InventoryManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Failed to parse manifest JSON: %v", err)
	}

	// Verify basic fields
	if manifest.SourceBucket != "duracloud-tftest-special" {
		t.Errorf("Expected sourceBucket 'duracloud-tftest-special', got '%s'", manifest.SourceBucket)
	}

	if manifest.DestinationBucket != "arn:aws:s3:::duracloud-tftest-managed" {
		t.Errorf("Expected destinationBucket 'arn:aws:s3:::duracloud-tftest-managed', got '%s'", manifest.DestinationBucket)
	}

	if manifest.FileFormat != "CSV" {
		t.Errorf("Expected fileFormat 'CSV', got '%s'", manifest.FileFormat)
	}

	// Test ParseFileSchema
	headers := manifest.ParseFileSchema()
	expectedHeaders := []string{
		"Bucket",
		"Key",
		"VersionId",
		"IsLatest",
		"IsDeleteMarker",
		"Size",
		"LastModifiedDate",
		"StorageClass",
	}

	if len(headers) != len(expectedHeaders) {
		t.Fatalf("Expected %d headers, got %d", len(expectedHeaders), len(headers))
	}

	for i, expected := range expectedHeaders {
		if headers[i] != expected {
			t.Errorf("Header[%d]: expected '%s', got '%s'", i, expected, headers[i])
		}
	}

	// Verify files array
	if len(manifest.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(manifest.Files))
	}

	file := manifest.Files[0]
	if file.Key != "inventory/duracloud-tftest-special/inventory/data/73c102a0-de11-4e95-9822-d10990fd83a8.csv.gz" {
		t.Errorf("Unexpected file key: %s", file.Key)
	}

	if file.Size != 194 {
		t.Errorf("Expected file size 194, got %d", file.Size)
	}

	if file.MD5Checksum != "4f39f9841460e250b494a6ab41a57c66" {
		t.Errorf("Expected MD5 checksum '4f39f9841460e250b494a6ab41a57c66', got '%s'", file.MD5Checksum)
	}
}

func TestParseDestinationBucket(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ARN format",
			input:    "arn:aws:s3:::duracloud-tftest-managed",
			expected: "duracloud-tftest-managed",
		},
		{
			name:     "Plain bucket name",
			input:    "duracloud-tftest-managed",
			expected: "duracloud-tftest-managed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var manifest InventoryManifest
			manifest.DestinationBucket = tt.input
			result := manifest.Bucket()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
