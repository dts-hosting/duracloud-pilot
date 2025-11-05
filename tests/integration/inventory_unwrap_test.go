package integration

import (
	"bytes"
	"context"
	"duracloud/internal/files"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryUnwrapWorkflow(t *testing.T) {
	t.Parallel()

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	// Use a unique test bucket name to avoid conflicts
	testBucketName := fmt.Sprintf("%s-inventory-test-%d", stackName, time.Now().UnixNano())
	managedBucket := fmt.Sprintf("%s-managed", stackName)

	// Read fixture files
	csvData, err := os.ReadFile("../../files/inventory-123456.csv.gz")
	require.NoError(t, err, "Failed to read inventory CSV fixture")

	// Create a dynamic manifest with the correct test bucket names
	manifestTemplate := `{
  "sourceBucket": "%s",
  "destinationBucket": "arn:aws:s3:::%s",
  "version": "2016-11-30",
  "creationTimestamp": "1761613201000",
  "fileFormat": "CSV",
  "fileSchema": "Bucket, Key, VersionId, IsLatest, IsDeleteMarker, Size, LastModifiedDate, StorageClass",
  "files": [
    {
      "key": "inventory/%s/inventory/data/inventory-123456.csv.gz",
      "size": 194,
      "MD5checksum": "4f39f9841460e250b494a6ab41a57c66"
    }
  ]
}`
	manifestData := fmt.Appendf(nil, manifestTemplate, testBucketName, managedBucket, testBucketName)

	// Upload inventory CSV file to managed bucket
	csvKey := fmt.Sprintf("inventory/%s/inventory/data/inventory-123456.csv.gz", testBucketName)
	csvObj := files.NewS3Object(managedBucket, csvKey)
	err = files.UploadObject(ctx, clients.S3, csvObj, bytes.NewReader(csvData), "application/gzip")
	require.NoError(t, err, "Failed to upload inventory CSV")

	// Upload manifest file to trigger the function
	manifestKey := fmt.Sprintf("inventory/%s/inventory/2000-01-01/manifest.json", testBucketName)
	manifestObj := files.NewS3Object(managedBucket, manifestKey)
	err = files.UploadObject(ctx, clients.S3, manifestObj, bytes.NewReader(manifestData), "application/json")
	require.NoError(t, err, "Failed to upload manifest")

	// Wait for inventory unwrap function to process
	// The function should create:
	// 1. Concatenated CSV: inventory/{testBucketName}/inventory/csv/inventory-{date}.csv
	// 2. Stats JSON: inventory/{testBucketName}/inventory/stats/stats-{date}.json

	waitConfig := DefaultWaitConfig()
	waitConfig.MaxTimeout = 60 * time.Second

	// Wait for concatenated CSV to be created
	csvCreated := WaitForCondition(t, "concatenated CSV creation", func() bool {
		result, err := clients.S3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(managedBucket),
			Prefix: aws.String(fmt.Sprintf("inventory/%s/inventory/csv/inventory-", testBucketName)),
		})
		if err != nil {
			t.Logf("Error listing objects: %v", err)
			return false
		}
		return len(result.Contents) > 0
	}, waitConfig)
	assert.True(t, csvCreated, "Concatenated CSV should be created")

	// Wait for stats JSON to be created
	statsCreated := WaitForCondition(t, "stats JSON creation", func() bool {
		result, err := clients.S3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(managedBucket),
			Prefix: aws.String(fmt.Sprintf("inventory/%s/inventory/stats/stats-", testBucketName)),
		})
		if err != nil {
			t.Logf("Error listing objects: %v", err)
			return false
		}
		return len(result.Contents) > 0
	}, waitConfig)
	assert.True(t, statsCreated, "Stats JSON should be created")

	// Verify the stats file contains valid JSON with expected structure
	if statsCreated {
		result, err := clients.S3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(managedBucket),
			Prefix: aws.String(fmt.Sprintf("inventory/%s/inventory/stats/stats-", testBucketName)),
		})
		require.NoError(t, err)
		require.Greater(t, len(result.Contents), 0)

		statsKey := aws.ToString(result.Contents[0].Key)
		statsObj := files.NewS3Object(managedBucket, statsKey)

		reader, err := files.DownloadObject(ctx, clients.S3, statsObj, false)
		require.NoError(t, err, "Failed to download stats file")
		defer func() { _ = reader.Close() }()

		// Read and parse the stats JSON to verify structure
		var buf []byte
		buf, err = io.ReadAll(reader)
		require.NoError(t, err, "Failed to read stats content")

		// Basic validation - should contain key fields
		statsContent := string(buf)
		assert.Contains(t, statsContent, "bucket_name", "Stats should contain bucket_name")
		assert.Contains(t, statsContent, "total_count", "Stats should contain total_count")
		assert.Contains(t, statsContent, "total_bytes", "Stats should contain total_bytes")
		assert.Contains(t, statsContent, "prefix_stats", "Stats should contain prefix_stats")

		t.Logf("Successfully verified inventory unwrap workflow for %s", testBucketName)
		t.Logf("Stats file created at: %s", statsKey)
	}
}
