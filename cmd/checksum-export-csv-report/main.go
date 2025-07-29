package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"duracloud/internal/accounts"
	"encoding/csv"
	"encoding/json"
	"fmt"
	//"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

type AttributeValue map[string]string
type Item map[string]AttributeValue

type ManifestEntry struct {
	ItemCount     int    `json:"itemCount"`
	MD5Checksum   string `json:"md5Checksum"`
	ETag          string `json:"etag"`
	DataFileS3Key string `json:"dataFileS3Key"`
}

var (
	accountID    string
	awsCtx       accounts.AWSContext
	bucketPrefix string
	//exportBucket      string
	//items             []Item
	managedBucketName string
	prefix            string
	region            string
	s3Client          *s3.Client
	today             string
)

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
	if len(result.Contents) > 0 {
		firstObject = aws.ToString(result.Contents[0].Key)
	} else {
		fmt.Println("Bucket is empty.")
	}

	return firstObject, nil
}

func getExportManifest(ctx context.Context, prefix string) (manifest string, err error) {
	key := fmt.Sprintf("%s/manifest-files.json", prefix)
	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(managedBucketName),
		Key:    aws.String(key),
	})
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

func getExportDataFile(ctx context.Context, key string) (output string, err error) {
	key = fmt.Sprintf("%s/manifest-files.json", prefix)
	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(managedBucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Fatalf("failed to get object, %v", err)
	}

	// Create gzip reader
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		log.Fatalf("failed to create gzip reader: %v", err)
	}
	//defer gzr.Close()

	scanner := bufio.NewScanner(gzr)
	var out string
	for scanner.Scan() {
		line := scanner.Bytes()
		var entry ManifestEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			log.Printf("error parsing JSON line: %v", err)
			continue
		}
		out += fmt.Sprintf("%s,%d,%s\n", entry.DataFileS3Key, entry.ItemCount, entry.MD5Checksum)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner error: %v", err)
	}
	return out, nil
}

//func parseDataFile() (ctx context.Context, key string, err error) {}

func parseManifest(ctx context.Context, manifestBody string) (csv string, err error) {
	dec := json.NewDecoder(strings.NewReader(manifestBody))
	var csvReport string
	for {
		var e ManifestEntry
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		csv, err := getExportDataFile(ctx, e.DataFileS3Key)
		if err != nil {
			log.Fatal(err)
		}
		csvReport = fmt.Sprintf("%s\n%s", csvReport, csv)
	}

	return csvReport, nil
}

func parseExport(os.File) error {
	// Create CSV file
	file, err := os.Create("output.csv")
	if err != nil {
		log.Fatalf("failed to create output file, %v", err)
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Determine header order (e.g., sorted keys from the first map)
	var headers []string
	//for key := range data[0] {
	//	headers = append(headers, key)
	//}
	sort.Strings(headers)
	return nil
}

func decodeLine(raw []byte) (map[string]interface{}, error) {
	// Step 1: Unmarshal into map[string]types.AttributeValue
	var avMap map[string]types.AttributeValue
	if err := json.Unmarshal(raw, &avMap); err != nil {
		return nil, fmt.Errorf("parse export line: %w", err)
	}

	// Step 2: Convert to Go-native map[string]interface{}
	var out map[string]interface{}
	if err := attributevalue.UnmarshalMap(avMap, &out); err != nil {
		return nil, fmt.Errorf("unmarshal attr values: %w", err)
	}
	return out, nil
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

	prefix = fmt.Sprintf("/exports/checksum-table/%s/AWSDynamoDB/", today)
	log.Printf("loading from %s", prefix)

	arn, err := getExportArn(ctx, prefix)
	if err != nil {
		log.Printf("failed to get export arn: %v", err)
	}
	log.Printf("found export arn: %s", arn)

	manifest, err := getExportManifest(ctx, prefix)
	if err != nil {
		log.Printf("failed to get export manifest: %v", err)
	}

	parseManifest(ctx, manifest)
	return nil
}

func main() {
	lambda.Start(handler)
}
