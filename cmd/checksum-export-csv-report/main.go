package main

import (
	"compress/gzip"
	"context"
	"duracloud/internal/accounts"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	accountID         string
	awsCtx            accounts.AWSContext
	bucketPrefix      string
	managedBucketName string
	prefix            string
	region            string
	s3Client          *s3.Client
	today             string
)

type ManifestEntry struct {
	ItemCount     int    `json:"itemCount"`
	MD5Checksum   string `json:"md5Checksum"`
	ETag          string `json:"etag"`
	DataFileS3Key string `json:"dataFileS3Key"`
}

type ExportRecord struct {
	Item struct {
		BucketName          struct{ S string }  `json:"BucketName"`
		ObjectKey           struct{ S string }  `json:"ObjectKey"`
		Checksum            struct{ S string }  `json:"Checksum"`
		LastChecksumSuccess struct{ BOOL bool } `json:"LastChecksumSuccess"`
		LastChecksumDate    struct{ S string }  `json:"LastChecksumDate"`
		LastChecksumMessage struct{ S string }  `json:"LastChecksumMessage"`
	} `json:"Item"`
}

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	accountID, err = accounts.GetAccountID(context.Background(), awsConfig)
	if err != nil {
		log.Fatalf("Unable to get AWS account ID: %v", err)
	}

	bucketPrefix = os.Getenv("S3_BUCKET_PREFIX")
	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")
	today = time.Now().Format("2006-01-02")
	s3Client = s3.NewFromConfig(awsConfig)
	awsCtx = accounts.AWSContext{
		AccountID: accountID,
		Region:    region,
		StackName: bucketPrefix,
	}

}

func handler(ctx context.Context, event json.RawMessage) error {
	ctx = context.WithValue(ctx, accounts.AWSContextKey, awsCtx)

	// TODO: create fixture file that can be uploaded to match for manual trigger
	prefix = fmt.Sprintf("exports/checksum-table/%s/AWSDynamoDB/", today)
	log.Printf("loading from %s", prefix)

	arn, err := getExportArn(ctx, prefix)
	// TODO: this logs an error then continues ... update soon.
	if err != nil {
		log.Printf("failed to get export arn: %v", err)
	}
	log.Printf("found export arn: %s", arn)

	// TODO: this logs an error then continues ... update soon.
	manifest, err := getExportManifest(ctx, arn)
	if err != nil {
		log.Printf("failed to get export manifest: %v", err)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit to 10 goroutines at a time

	err = parseManifest(manifest, func(entry ManifestEntry) {
		wg.Add(1)
		go func(e ManifestEntry) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// TODO: this reads an entire json.gz into memory as csv
			// and returns it as a string. This may not hold-up.
			csvData, err := getExportDataFile(ctx, e.DataFileS3Key)
			if err != nil {
				log.Printf("Failed to get export data for %s: %v", e.DataFileS3Key, err)
				return // handle issue better
			}

			fileId := fmt.Sprintf("export_%s.csv", extractFileID(e.DataFileS3Key))
			csvFilename := getCsvKey(fileId, today, arn)

			if err := writeCSVFile(ctx, csvFilename, csvData); err != nil {
				log.Printf("Failed to write CSV for %s: %v", e.DataFileS3Key, err)
				return // handle issue better
			}

			log.Printf("Successfully processed %s", e.DataFileS3Key)
		}(entry)

	})

	if err != nil {
		log.Printf("failed to parse export data: %v", err)
		return err
	}
	wg.Wait()

	return nil
}

func extractFileID(key string) string {
	filename := path.Base(key)
	id := strings.TrimSuffix(filename, ".json.gz")
	return id
}

func getCsvKey(id string, date string, exportId string) string {
	return fmt.Sprintf("exports/checksum-table/%s/CSV/%s/export_%s.csv", date, exportId, id)
}

func getExportArn(ctx context.Context, prefix string) (exportArn string, err error) {
	result, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(managedBucketName),
		Prefix:    aws.String(prefix),
		MaxKeys:   aws.Int32(1),
		Delimiter: aws.String("/"), // This tells S3 to return "folders"
	})
	if err != nil {
		log.Fatalf("failed to list objects, %v", err)
	}
	var firstObject string
	// TODO: this is returning "" when there are no exports. Fix soon.
	if len(result.CommonPrefixes) > 0 {
		firstObject = aws.ToString(result.CommonPrefixes[0].Prefix)
		log.Printf("Found export %s", firstObject)
	} else {
		log.Printf("Managed Bucket %s is empty.", managedBucketName)
	}

	return firstObject, nil
}

func getExportDataFile(ctx context.Context, key string) (output string, err error) {
	obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(managedBucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Fatalf("failed to get object, %v", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(obj.Body)

	gzr, err := gzip.NewReader(obj.Body)
	if err != nil {
		log.Fatalf("failed to create gzip reader: %v", err)
	}
	defer func(gzr *gzip.Reader) {
		_ = gzr.Close()
	}(gzr)

	dec := json.NewDecoder(gzr)

	var b strings.Builder
	w := csv.NewWriter(&b)

	_ = w.Write([]string{
		"BucketName,ObjectKey,Checksum,LastChecksumSuccess,LastChecksumDate,LastChecksumMessage",
	})

	for {
		var rec ExportRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		_ = w.Write([]string{
			rec.Item.BucketName.S,
			rec.Item.ObjectKey.S,
			rec.Item.Checksum.S,
			strconv.FormatBool(rec.Item.LastChecksumSuccess.BOOL),
			rec.Item.LastChecksumDate.S,
			rec.Item.LastChecksumMessage.S,
		})
		w.Flush()
	}
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func getExportManifest(ctx context.Context, prefix string) (manifest string, err error) {
	key := fmt.Sprintf("%smanifest-files.json", prefix)
	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(managedBucketName),
		Key:    aws.String(key),
	})
	// TODO: should be returning errors
	if err != nil {
		log.Fatalf("failed to get object, %v", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read body: %v", err)
	}

	bodyString := string(bodyBytes)
	return bodyString, nil
}

func parseManifest(manifestBody string, processEntry func(ManifestEntry)) error {
	dec := json.NewDecoder(strings.NewReader(manifestBody))
	for {
		var e ManifestEntry
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed to decode manifest entry: %w", err)
		}

		processEntry(e)
	}

	return nil
}

func writeCSVFile(ctx context.Context, key string, csv string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(managedBucketName),
		Key:    aws.String(key),
		Body:   strings.NewReader(csv),
	}
	_, err := s3Client.PutObject(ctx, input)

	// TODO: should be returning errors
	if err != nil {
		log.Fatalf("Failed to write CSV Report to S3, %v", err)
	}
	log.Printf("Successfully wrote CSV Report at %s to S3", key)
	return nil
}

func main() {
	lambda.Start(handler)
}
