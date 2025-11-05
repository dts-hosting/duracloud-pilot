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

func TestReportGeneratorWorkflow(t *testing.T) {
	t.Parallel()

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	managedBucket := fmt.Sprintf("%s-managed", stackName)

	// Use unique test bucket names
	testBucket1 := fmt.Sprintf("%s-report-test-1-%d", stackName, time.Now().UnixNano())
	testBucket2 := fmt.Sprintf("%s-report-test-2-%d", stackName, time.Now().UnixNano())

	// Read stats fixture files
	stats1Data, err := os.ReadFile("../../files/stats-fixture-1.json")
	require.NoError(t, err, "Failed to read stats fixture 1")

	stats2Data, err := os.ReadFile("../../files/stats-fixture-2.json")
	require.NoError(t, err, "Failed to read stats fixture 2")

	// Upload stats files for test buckets
	dateStr := time.Now().UTC().Format("2006-01-02")

	stats1Key := fmt.Sprintf("inventory/%s/inventory/stats/stats-%s.json", testBucket1, dateStr)
	stats1Obj := files.NewS3Object(managedBucket, stats1Key)
	err = files.UploadObject(ctx, clients.S3, stats1Obj, bytes.NewReader(stats1Data), "application/json")
	require.NoError(t, err, "Failed to upload stats fixture 1")

	stats2Key := fmt.Sprintf("inventory/%s/inventory/stats/stats-%s.json", testBucket2, dateStr)
	stats2Obj := files.NewS3Object(managedBucket, stats2Key)
	err = files.UploadObject(ctx, clients.S3, stats2Obj, bytes.NewReader(stats2Data), "application/json")
	require.NoError(t, err, "Failed to upload stats fixture 2")

	// Invoke report generator function
	functionName := fmt.Sprintf("%s-report-generator", stackName)
	eventPayload := `{}`

	_, err = lambdaFunctionInvoke(ctx, clients.Lambda, functionName, []byte(eventPayload))
	require.NoError(t, err, "Failed to invoke report-generator lambda")

	// Wait for report to be created
	waitConfig := DefaultWaitConfig()
	waitConfig.MaxTimeout = 60 * time.Second

	var reportCreated bool
	var reportKey string

	reportCreated = WaitForCondition(t, "report HTML creation", func() bool {
		result, err := clients.S3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(managedBucket),
			Prefix: aws.String(fmt.Sprintf("reports/storage-report-%s.html", dateStr)),
		})
		if err != nil {
			t.Logf("Error listing reports: %v", err)
			return false
		}
		if len(result.Contents) > 0 {
			reportKey = aws.ToString(result.Contents[0].Key)
			return true
		}
		return false
	}, waitConfig)

	assert.True(t, reportCreated, "Report HTML should be created")

	// Verify the report contains expected content
	if reportCreated {
		reportObj := files.NewS3Object(managedBucket, reportKey)

		reader, err := files.DownloadObject(ctx, clients.S3, reportObj, false)
		require.NoError(t, err, "Failed to download report file")
		defer func() { _ = reader.Close() }()

		reportContent, err := io.ReadAll(reader)
		require.NoError(t, err, "Failed to read report content")

		reportHTML := string(reportContent)

		// Verify HTML structure (case-insensitive doctype check)
		assert.Contains(t, reportHTML, "<!doctype html>", "Report should be valid HTML")
		assert.Contains(t, reportHTML, "DuraCloud Storage Report", "Report should have title")
		assert.Contains(t, reportHTML, stackName, "Report should mention stack name")

		// Verify report contains bucket information
		assert.Contains(t, reportHTML, "Bucket Details", "Report should have bucket details section")
		assert.Contains(t, reportHTML, "Summary", "Report should have summary section")

		// Verify stats freshness information is displayed
		assert.Contains(t, reportHTML, "Stats Date", "Report should show stats date")
		assert.Contains(t, reportHTML, "Stats Generated", "Report should show stats generated time")

		// Verify formatting functions work
		assert.Contains(t, reportHTML, "Storage:", "Report should show storage metrics")
		assert.Contains(t, reportHTML, "Files:", "Report should show file counts")

		t.Logf("Successfully verified report generator workflow")
		t.Logf("Report created at: s3://%s/%s", managedBucket, reportKey)
		t.Logf("Report size: %d bytes", len(reportContent))
	}
}
