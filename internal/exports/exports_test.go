package exports

import (
	"encoding/csv"
	"strings"
	"testing"
)

// createS3Event creates a minimal S3Event for testing
func createS3Event(objectKey string) *S3Event {
	return &S3Event{
		Records: []S3EventRecord{
			{
				S3: S3Data{
					Bucket: S3Bucket{Name: "test-bucket"},
					Object: S3Object{Key: objectKey},
				},
			},
		},
	}
}

func TestS3Event_ObjectDate(t *testing.T) {
	tests := []struct {
		name        string
		objectKey   string
		wantDate    string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid path with date",
			objectKey: "exports/checksum-table/2025-08-25/AWSDynamoDB/01234567890123456789/data/file.json.gz",
			wantDate:  "2025-08-25",
			wantErr:   false,
		},
		{
			name:      "minimal valid path",
			objectKey: "exports/checksum-table/2025-12-31",
			wantDate:  "2025-12-31",
			wantErr:   false,
		},
		{
			name:        "too few parts - 2 parts",
			objectKey:   "exports/checksum-table",
			wantDate:    "",
			wantErr:     true,
			errContains: "expected at least 3 parts but got 2",
		},
		{
			name:        "too few parts - 1 part",
			objectKey:   "exports",
			wantDate:    "",
			wantErr:     true,
			errContains: "expected at least 3 parts but got 1",
		},
		{
			name:        "empty path",
			objectKey:   "",
			wantDate:    "",
			wantErr:     true,
			errContains: "expected at least 3 parts but got 1",
		},
		{
			name:      "extra parts should work",
			objectKey: "exports/checksum-table/2023-01-15/extra/parts/here/file.json",
			wantDate:  "2023-01-15",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := createS3Event(tt.objectKey)
			gotDate, err := event.ObjectDate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("ObjectDate() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ObjectDate() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ObjectDate() unexpected error = %v", err)
				return
			}

			if gotDate != tt.wantDate {
				t.Errorf("ObjectDate() = %v, want %v", gotDate, tt.wantDate)
			}
		})
	}
}

func TestS3Event_ObjectExportArn(t *testing.T) {
	tests := []struct {
		name        string
		objectKey   string
		wantArn     string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid path with export ARN",
			objectKey: "exports/checksum-table/2025-08-25/AWSDynamoDB/01234567890123456789/data/file.json.gz",
			wantArn:   "01234567890123456789",
			wantErr:   false,
		},
		{
			name:      "minimal valid path",
			objectKey: "exports/checksum-table/2025-08-25/AWSDynamoDB/my-export-arn",
			wantArn:   "my-export-arn",
			wantErr:   false,
		},
		{
			name:        "too few parts - 4 parts",
			objectKey:   "exports/checksum-table/2025-08-25/AWSDynamoDB",
			wantArn:     "",
			wantErr:     true,
			errContains: "expected at least 5 parts but got 4",
		},
		{
			name:        "too few parts - 3 parts",
			objectKey:   "exports/checksum-table/2025-08-25",
			wantArn:     "",
			wantErr:     true,
			errContains: "expected at least 5 parts but got 3",
		},
		{
			name:        "too few parts - 2 parts",
			objectKey:   "exports/checksum-table",
			wantArn:     "",
			wantErr:     true,
			errContains: "expected at least 5 parts but got 2",
		},
		{
			name:        "empty path",
			objectKey:   "",
			wantArn:     "",
			wantErr:     true,
			errContains: "expected at least 5 parts but got 1",
		},
		{
			name:      "extra parts should work",
			objectKey: "exports/checksum-table/2025-08-25/AWSDynamoDB/test-arn/extra/parts/file.json",
			wantArn:   "test-arn",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := createS3Event(tt.objectKey)
			gotArn, err := event.ObjectExportArn()

			if tt.wantErr {
				if err == nil {
					t.Errorf("ObjectExportArn() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ObjectExportArn() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ObjectExportArn() unexpected error = %v", err)
				return
			}

			if gotArn != tt.wantArn {
				t.Errorf("ObjectExportArn() = %v, want %v", gotArn, tt.wantArn)
			}
		})
	}
}

func TestExportRecord_ToCSVRow(t *testing.T) {
	// Create a test record
	record := &ExportRecord{
		Item: ExportItem{
			BucketName:          struct{ S string }{"test-bucket"},
			ObjectKey:           struct{ S string }{"path/to/file.txt"},
			Checksum:            struct{ S string }{"abc123def456"},
			LastChecksumSuccess: struct{ BOOL bool }{true},
			LastChecksumDate:    struct{ S string }{"2025-08-26T10:30:00Z"},
			LastChecksumMessage: struct{ S string }{"Checksum verified successfully"},
		},
	}

	// Generate CSV
	var b strings.Builder
	w := csv.NewWriter(&b)

	// Write headers and data
	_ = w.Write(ExportHeaders)
	_ = w.Write(record.ToCSVRow())
	w.Flush()

	if err := w.Error(); err != nil {
		t.Fatalf("CSV writer error: %v", err)
	}

	csvOutput := b.String()

	// Verify the CSV content
	expectedLines := []string{
		"BucketName,ObjectKey,Checksum,LastChecksumSuccess,LastChecksumDate,LastChecksumMessage",
		"test-bucket,path/to/file.txt,abc123def456,true,2025-08-26T10:30:00Z,Checksum verified successfully",
	}

	lines := strings.Split(strings.TrimSpace(csvOutput), "\n")

	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	for i, expectedLine := range expectedLines {
		if lines[i] != expectedLine {
			t.Errorf("Line %d mismatch:\nGot:      %s\nExpected: %s", i+1, lines[i], expectedLine)
		}
	}
}
