package exports

import (
	"encoding/json"
	"io"
	"strconv"
)

const ManifestFile = "manifest-files.json"

var ExportHeaders = []string{
	"BucketName",
	"ObjectKey",
	"Checksum",
	"LastChecksumSuccess",
	"LastChecksumDate",
	"LastChecksumMessage",
}

// ManifestEntry represents the fields in a DynamoDB manifest
type ManifestEntry struct {
	ItemCount     int    `json:"itemCount"`
	DataFileS3Key string `json:"dataFileS3Key"`
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
