package notifications

import (
	"os"
	"path/filepath"
	"testing"
	"text/template"
)

func TestChecksumFailureNotificationMessage(t *testing.T) {
	templatePath := filepath.Join("..", "..", "cmd", "checksum-failure", "templates", "failure-notification.txt")
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("Failed to read template file: %v", err)
	}

	tmpl, err := template.New("test").Parse(string(templateBytes))
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	notification := ChecksumFailureNotification{
		Account:      "123456789012",
		Stack:        "duracloud-pilot",
		Date:         "2025-06-26 14:30:25 +0000 UTC",
		Bucket:       "duracloud-pilot-private-files",
		Object:       "documents/report-2024.pdf",
		ErrorMessage: "S3 ETag mismatch: expected abc123, got def456",
		Title:        "DuraCloud Checksum Failure: duracloud-pilot-private-files",
		Template:     tmpl,
		Topic:        "arn:aws:sns:us-east-1:123456789012:test-topic",
	}

	message, err := notification.Message()
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	// Verify the output
	expected := `Checksum verification failed for:

Account: 123456789012
Stack: duracloud-pilot
Time: 2025-06-26 14:30:25 +0000 UTC

Bucket: duracloud-pilot-private-files
Object: documents/report-2024.pdf
Error: S3 ETag mismatch: expected abc123, got def456
`

	if message != expected {
		t.Errorf("Template output mismatch.\nExpected:\n%s\nGot:\n%s", expected, message)
	}

	if notification.Subject() != "DuraCloud Checksum Failure: duracloud-pilot-private-files" {
		t.Errorf("Unexpected subject: %s", notification.Subject())
	}

	if notification.TopicArn() != "arn:aws:sns:us-east-1:123456789012:test-topic" {
		t.Errorf("Unexpected topic ARN: %s", notification.TopicArn())
	}
}
