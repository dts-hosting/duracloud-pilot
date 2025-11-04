package reports

import (
	"context"
	"duracloud/internal/files"
	"duracloud/internal/inventory"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BucketStatsReader handles reading pre-computed inventory stats from S3
type BucketStatsReader struct {
	ctx       context.Context
	s3Client  *s3.Client
	stackName string
}

// NewBucketStatsReader creates a new stats reader
func NewBucketStatsReader(ctx context.Context, s3Client *s3.Client, stackName string) *BucketStatsReader {
	return &BucketStatsReader{
		ctx:       ctx,
		s3Client:  s3Client,
		stackName: stackName,
	}
}

// FindBucketsWithStats returns buckets that have stats files in the managed bucket
func (r *BucketStatsReader) FindBucketsWithStats(managedBucket string) ([]string, error) {
	// List all inventory prefixes to find buckets with stats
	result, err := r.s3Client.ListObjectsV2(r.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(managedBucket),
		Prefix:    aws.String("inventory/"),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory prefixes: %w", err)
	}

	var bucketsWithStats []string

	// For each bucket prefix, check if it has stats files
	for _, prefix := range result.CommonPrefixes {
		prefixStr := aws.ToString(prefix.Prefix)
		// Extract bucket name from: inventory/{bucketName}/
		parts := strings.Split(strings.TrimSuffix(prefixStr, "/"), "/")
		if len(parts) >= 2 {
			bucketName := parts[1]

			// Check if this bucket has stats files
			statsPrefix := fmt.Sprintf("inventory/%s/inventory/stats/", bucketName)
			statsResult, err := r.s3Client.ListObjectsV2(r.ctx, &s3.ListObjectsV2Input{
				Bucket:  aws.String(managedBucket),
				Prefix:  aws.String(statsPrefix),
				MaxKeys: aws.Int32(1), // Just need to know if any exist
			})
			if err != nil {
				log.Printf("Warning: failed to check stats for %s: %v", bucketName, err)
				continue
			}

			if len(statsResult.Contents) > 0 {
				log.Printf("Found bucket with stats: %s", bucketName)
				bucketsWithStats = append(bucketsWithStats, bucketName)
			}
		}
	}

	return bucketsWithStats, nil
}

// GetBucketTags retrieves tags for a bucket
func (r *BucketStatsReader) GetBucketTags(bucketName string) (map[string]string, error) {
	result, err := r.s3Client.GetBucketTagging(r.ctx, &s3.GetBucketTaggingInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	for _, tag := range result.TagSet {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags, nil
}

// GetLatestStats retrieves the most recent stats file for a bucket
func (r *BucketStatsReader) GetLatestStats(bucketName, managedBucket string) (*inventory.InventoryStats, error) {
	// Stats are stored in: inventory/{bucketName}/inventory/stats/stats-{date}.json
	statsPrefix := fmt.Sprintf("inventory/%s/inventory/stats/", bucketName)

	// List stats files to find the latest
	result, err := r.s3Client.ListObjectsV2(r.ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(managedBucket),
		Prefix:  aws.String(statsPrefix),
		MaxKeys: aws.Int32(100), // Should be plenty for recent stats
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list stats files: %w", err)
	}

	if len(result.Contents) == 0 {
		return nil, fmt.Errorf("no stats files found for bucket %s", bucketName)
	}

	// Find the most recent stats file (they're named stats-YYYY-MM-DD.json, so lexical sort works)
	var latestKey string
	for _, obj := range result.Contents {
		key := aws.ToString(obj.Key)
		if strings.HasSuffix(key, ".json") {
			if latestKey == "" || key > latestKey {
				latestKey = key
			}
		}
	}

	if latestKey == "" {
		return nil, fmt.Errorf("no valid stats files found for bucket %s", bucketName)
	}

	log.Printf("Loading stats from s3://%s/%s", managedBucket, latestKey)

	statsObj := files.NewS3Object(managedBucket, latestKey)
	reader, err := files.DownloadObject(r.ctx, r.s3Client, statsObj, false)
	if err != nil {
		return nil, fmt.Errorf("failed to download stats file: %w", err)
	}
	defer func() { _ = reader.Close() }()

	var stats inventory.InventoryStats
	if err := json.NewDecoder(reader).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats file: %w", err)
	}

	return &stats, nil
}
