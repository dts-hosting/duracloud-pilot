package reports

import (
	"bytes"
	"context"
	"duracloud/internal/buckets"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type StorageReportGenerator struct {
	s3Client  *s3.Client
	cwClient  *cloudwatch.Client
	stackName string
}

type BucketStats struct {
	Name         string
	TotalSize    int64
	TotalObjects int64
	PrefixStats  []PrefixStats
	Tags         map[string]string
}

type PrefixStats struct {
	Prefix      string
	Size        int64
	ObjectCount int64
}

type ReportData struct {
	GeneratedAt  time.Time
	StackName    string
	TotalBuckets int64
	TotalSize    int64
	TotalObjects int64
	BucketStats  []BucketStats
}

func NewStorageReportGenerator(s3Client *s3.Client, cwClient *cloudwatch.Client, stackName string) *StorageReportGenerator {
	return &StorageReportGenerator{
		s3Client:  s3Client,
		cwClient:  cwClient,
		stackName: stackName,
	}
}

func (g *StorageReportGenerator) GenerateReport(ctx context.Context, tmpl *template.Template) (string, error) {
	log.Println("Getting eligible buckets")
	reportBuckets, err := g.getEligibleBuckets(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get eligible buckets: %w", err)
	}

	if len(reportBuckets) == 0 {
		log.Println("No eligible buckets found")
		return "", nil
	}

	log.Printf("Collecting statistics for %d buckets\n", len(reportBuckets))
	bucketStats, err := g.collectBucketStatistics(ctx, reportBuckets)
	if err != nil {
		return "", fmt.Errorf("failed to collect bucket statistics: %w", err)
	}

	log.Println("Calculating aggregates")
	reportData := g.calculateAggregates(bucketStats)

	log.Printf("Report data: %+v\n", reportData)

	log.Println("Generating HTML report")
	htmlReport, err := g.generateHTML(reportData, tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML: %w", err)
	}

	return htmlReport, nil
}

func (g *StorageReportGenerator) getEligibleBuckets(ctx context.Context) ([]string, error) {
	result, err := g.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var eligibleBuckets []string

	for _, bucket := range result.Buckets {
		bucketName := aws.ToString(bucket.Name)
		if !strings.HasPrefix(bucketName, g.stackName) {
			continue
		}

		tags, err := g.getBucketTags(ctx, bucketName)
		if err != nil {
			continue
		}

		if g.isEligibleBucket(tags) {
			log.Printf("Found eligible bucket: %s\n", bucketName)
			eligibleBuckets = append(eligibleBuckets, bucketName)
		}
	}

	return eligibleBuckets, nil
}

func (g *StorageReportGenerator) getBucketTags(ctx context.Context, bucketName string) (map[string]string, error) {
	result, err := g.s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
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

func (g *StorageReportGenerator) isEligibleBucket(tags map[string]string) bool {
	application, hasApplicationTag := tags[buckets.ApplicationTagKey]
	if !hasApplicationTag {
		return false
	}

	if application != buckets.ApplicationTagValue {
		return false
	}

	bucketType, hasBucketType := tags[buckets.BucketTypeTagKey]
	if !hasBucketType {
		return false
	}

	if bucketType != buckets.StandardTagValue && bucketType != buckets.PublicTagValue {
		return false
	}

	stackName, hasStackName := tags[buckets.StackNameTagKey]
	return hasStackName && stackName == g.stackName
}

func (g *StorageReportGenerator) collectBucketStatistics(ctx context.Context, buckets []string) ([]BucketStats, error) {
	var wg sync.WaitGroup
	statsChan := make(chan BucketStats, len(buckets))
	errorChan := make(chan error, len(buckets))

	for _, bucketName := range buckets {
		log.Printf("Collecting stats for bucket: %s\n", bucketName)
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			stats, err := g.getBucketStatistics(ctx, name)
			if err != nil {
				errorChan <- fmt.Errorf("failed to get stats for bucket %s: %w", name, err)
				return
			}

			statsChan <- stats
		}(bucketName)
	}

	wg.Wait()
	close(statsChan)
	close(errorChan)

	for err := range errorChan {
		return nil, err
	}

	// Collect results
	var bucketStats []BucketStats
	for stats := range statsChan {
		bucketStats = append(bucketStats, stats)
	}

	// Sort by bucket name for consistent output
	sort.Slice(bucketStats, func(i, j int) bool {
		return bucketStats[i].Name < bucketStats[j].Name
	})

	return bucketStats, nil
}

func (g *StorageReportGenerator) getBucketStatistics(ctx context.Context, bucketName string) (BucketStats, error) {
	stats := BucketStats{
		Name: bucketName,
	}

	// Get bucket tags
	tags, err := g.getBucketTags(ctx, bucketName)
	if err != nil {
		tags = make(map[string]string) // Continue with empty tags
	}
	stats.Tags = tags

	// Get storage metrics from CloudWatch
	totalSize, totalObjects, err := g.getStorageMetrics(ctx, bucketName)
	if err != nil {
		return stats, fmt.Errorf("failed to get storage metrics: %w", err)
	}

	stats.TotalSize = totalSize
	stats.TotalObjects = totalObjects

	// Get prefix statistics
	prefixStats, err := g.getPrefixStatistics(ctx, bucketName)
	if err != nil {
		return stats, fmt.Errorf("failed to get prefix statistics: %w", err)
	}

	stats.PrefixStats = prefixStats

	return stats, nil
}

func (g *StorageReportGenerator) getStorageMetrics(ctx context.Context, bucketName string) (int64, int64, error) {
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	storageTypes := []string{
		"StandardStorage",
		"GlacierInstantRetrievalStorage",
	}

	var totalSize int64

	// Query each storage type separately
	for _, storageType := range storageTypes {
		sizeInput := &cloudwatch.GetMetricStatisticsInput{
			Namespace:  aws.String("AWS/S3"),
			MetricName: aws.String("BucketSizeBytes"),
			Dimensions: []cwTypes.Dimension{
				{
					Name:  aws.String("BucketName"),
					Value: aws.String(bucketName),
				},
				{
					Name:  aws.String("StorageType"),
					Value: aws.String(storageType),
				},
			},
			StartTime:  aws.Time(startTime),
			EndTime:    aws.Time(endTime),
			Period:     aws.Int32(86400),
			Statistics: []cwTypes.Statistic{cwTypes.StatisticMaximum},
		}

		sizeResult, err := g.cwClient.GetMetricStatistics(ctx, sizeInput)
		if err != nil {
			continue
		}

		if len(sizeResult.Datapoints) > 0 {
			totalSize += int64(*sizeResult.Datapoints[0].Maximum)
		}
	}

	// Get object count
	countInput := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/S3"),
		MetricName: aws.String("NumberOfObjects"),
		Dimensions: []cwTypes.Dimension{
			{
				Name:  aws.String("BucketName"),
				Value: aws.String(bucketName),
			},
			{
				Name:  aws.String("StorageType"),
				Value: aws.String("AllStorageTypes"),
			},
		},
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(86400),
		Statistics: []cwTypes.Statistic{cwTypes.StatisticMaximum},
	}

	countResult, err := g.cwClient.GetMetricStatistics(ctx, countInput)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get count metrics: %w", err)
	}

	var totalObjects int64
	if len(countResult.Datapoints) > 0 {
		totalObjects = int64(*countResult.Datapoints[0].Maximum)
	}

	return totalSize, totalObjects, nil
}

func (g *StorageReportGenerator) getPrefixStatistics(ctx context.Context, bucketName string) ([]PrefixStats, error) {
	var prefixStats []PrefixStats

	paginator := s3.NewListObjectsV2Paginator(g.s3Client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(1000),
	})

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Process each common prefix (top-level directory) in this page
		for _, commonPrefix := range result.CommonPrefixes {
			prefix := aws.ToString(commonPrefix.Prefix)

			// Get statistics for this prefix by listing its contents
			size, count, err := g.calculatePrefixStats(ctx, bucketName, prefix)
			if err != nil {
				continue
			}

			prefixStats = append(prefixStats, PrefixStats{
				Prefix:      prefix,
				Size:        size,
				ObjectCount: count,
			})
		}
	}

	sort.Slice(prefixStats, func(i, j int) bool {
		return prefixStats[i].Size > prefixStats[j].Size
	})

	return prefixStats, nil
}

func (g *StorageReportGenerator) calculatePrefixStats(ctx context.Context, bucketName, prefix string) (int64, int64, error) {
	var totalSize, totalCount int64

	// List all objects with this prefix
	paginator := s3.NewListObjectsV2Paginator(g.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, 0, err
		}

		for _, object := range result.Contents {
			totalSize += aws.ToInt64(object.Size)
			totalCount++
		}
	}

	return totalSize, totalCount, nil
}

func (g *StorageReportGenerator) calculateAggregates(bucketStats []BucketStats) ReportData {
	report := ReportData{
		GeneratedAt:  time.Now(),
		StackName:    g.stackName,
		TotalBuckets: int64(len(bucketStats)),
		BucketStats:  bucketStats,
	}

	// Calculate totals
	for _, bucket := range bucketStats {
		log.Printf("Calculating totals for bucket: %s\n", bucket.Name)
		report.TotalSize += bucket.TotalSize
		report.TotalObjects += bucket.TotalObjects
	}

	return report
}

func (g *StorageReportGenerator) generateHTML(data ReportData, tmpl *template.Template) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (g *StorageReportGenerator) UploadReport(ctx context.Context, bucketName, key, content string) error {
	log.Printf("Uploading report to bucket: %s/%s\n", bucketName, key)

	_, err := g.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(content),
		ContentType: aws.String("text/html"),
	})

	return err
}
