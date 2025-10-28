package exports

import (
	"encoding/csv"
	"strings"
	"testing"
)

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
