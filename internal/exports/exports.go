package exports

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

const ExportSuffix = ".json.gz"

// ExportItem represents the fields in a DynamoDB export record
type ExportItem struct {
	BucketName          struct{ S string }  `json:"BucketName"`
	ObjectKey           struct{ S string }  `json:"ObjectKey"`
	Checksum            struct{ S string }  `json:"Checksum"`
	LastChecksumSuccess struct{ BOOL bool } `json:"LastChecksumSuccess"`
	LastChecksumDate    struct{ S string }  `json:"LastChecksumDate"`
	LastChecksumMessage struct{ S string }  `json:"LastChecksumMessage"`
}

var ExportHeaders = []string{
	"BucketName",
	"ObjectKey",
	"Checksum",
	"LastChecksumSuccess",
	"LastChecksumDate",
	"LastChecksumMessage",
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

// ObjectDate extracts from path like: exports/checksum-table/2025-08-25/AWSDynamoDB/...
func (e *S3Event) ObjectDate() (string, error) {
	parts := strings.Split(e.ObjectKey(), "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid path format: expected at least 3 parts but got %d in path %s", len(parts), e.ObjectKey())
	}
	return parts[2], nil // The date part
}

// ObjectExportArn extracts from path like: exports/checksum-table/2025-08-25/AWSDynamoDB/01234567890123456789/data/file.json.gz
func (e *S3Event) ObjectExportArn() (string, error) {
	parts := strings.Split(e.ObjectKey(), "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid path format: expected at least 5 parts but got %d in path %s", len(parts), e.ObjectKey())
	}
	return parts[4], nil // The export ARN part
}

func (e *S3Event) FileId() string {
	filename := path.Base(e.ObjectKey())
	id := strings.TrimSuffix(filename, ExportSuffix)
	return id
}

func (e *S3Event) ObjectKey() string {
	return e.Records[0].S3.Object.Key
}

// ProcessExport processes JSON export data, calling callback for each record
// Returns the number of records processed
func ProcessExport(reader io.Reader, callback func(*ExportRecord) error) (int, error) {
	dec := json.NewDecoder(reader)
	count := 0

	for {
		var rec ExportRecord
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
