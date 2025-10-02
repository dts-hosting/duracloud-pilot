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

const (
	MaxConcurrentWorkers      = 10               // Concurrent bucket workers
	BucketWorkerTimeout       = 10 * time.Minute // Per-bucket timeout
	PrefixWorkerPoolSize      = 5                // Concurrent prefix processors per bucket
	PrefixCollectionTimeout   = 5 * time.Minute  // Total timeout for all prefix stats
	PrefixProgressLogInterval = 10               // Log every N pages (10 pages = 10k objects)
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
	statsChan := make(chan BucketStats, len(buckets))
	errorChan := make(chan error, len(buckets))

	g.processBuckets(ctx, buckets, statsChan, errorChan)

	close(statsChan)
	close(errorChan)

	var bucketStats []BucketStats
	for stats := range statsChan {
		bucketStats = append(bucketStats, stats)
	}

	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	// Log errors but continue with partial data
	if len(errors) > 0 {
		for _, err := range errors {
			log.Printf("Warning: %v", err)
		}
		// Only fail if we got NO successful results
		if len(bucketStats) == 0 {
			return nil, fmt.Errorf("failed to collect any bucket statistics: %d errors", len(errors))
		}
		log.Printf("Successfully collected stats for %d/%d buckets (%d errors)",
			len(bucketStats), len(buckets), len(errors))
	}

	sort.Slice(bucketStats, func(i, j int) bool {
		return bucketStats[i].Name < bucketStats[j].Name
	})

	return bucketStats, nil
}

func (g *StorageReportGenerator) processBuckets(
	ctx context.Context,
	buckets []string,
	statsChan chan<- BucketStats,
	errorChan chan<- error,
) {
	var wg sync.WaitGroup
	bucketQueue := make(chan string, len(buckets))

	for _, bucket := range buckets {
		bucketQueue <- bucket
	}
	close(bucketQueue)

	numWorkers := MaxConcurrentWorkers
	if len(buckets) < numWorkers {
		numWorkers = len(buckets)
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for bucketName := range bucketQueue {
				log.Printf("Worker %d processing bucket: %s", workerID, bucketName)

				bucketCtx, cancel := context.WithTimeout(ctx, BucketWorkerTimeout)
				stats, err := g.getBucketStatistics(bucketCtx, bucketName)
				cancel()

				if err != nil {
					errorChan <- fmt.Errorf("worker %d failed for bucket %s: %w",
						workerID, bucketName, err)
					continue
				}

				statsChan <- stats
			}
		}(i)
	}

	wg.Wait()
}

func (g *StorageReportGenerator) getBucketStatistics(ctx context.Context, bucketName string) (BucketStats, error) {
	if err := ctx.Err(); err != nil {
		return BucketStats{}, fmt.Errorf("context cancelled: %w", err)
	}

	stats := BucketStats{
		Name: bucketName,
	}

	tags, err := g.getBucketTags(ctx, bucketName)
	if err != nil {
		tags = make(map[string]string)
		log.Printf("Warning: failed to get tags for %s: %v", bucketName, err)
	}
	stats.Tags = tags

	totalSize, totalObjects, err := g.getStorageMetrics(ctx, bucketName)
	if err != nil {
		return stats, fmt.Errorf("failed to get storage metrics: %w", err)
	}

	stats.TotalSize = totalSize
	stats.TotalObjects = totalObjects

	prefixStats, err := g.getPrefixStatistics(ctx, bucketName)
	if err != nil {
		return stats, fmt.Errorf("failed to get prefix statistics: %w", err)
	}

	stats.PrefixStats = prefixStats

	return stats, nil
}

func (g *StorageReportGenerator) getStorageMetrics(ctx context.Context, bucketName string) (int64, int64, error) {
	endTime := time.Now()
	startTime := endTime.Add(-48 * time.Hour)

	// Target just the storage classes we care about
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
			log.Printf("Warning: failed to get %s metrics for %s: %v",
				storageType, bucketName, err)
			continue
		}

		if len(sizeResult.Datapoints) > 0 && sizeResult.Datapoints[0].Maximum != nil {
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
	if len(countResult.Datapoints) > 0 && countResult.Datapoints[0].Maximum != nil {
		totalObjects = int64(*countResult.Datapoints[0].Maximum)
	}

	return totalSize, totalObjects, nil
}

func (g *StorageReportGenerator) getPrefixStatistics(ctx context.Context, bucketName string) ([]PrefixStats, error) {
	// Create timeout context for the entire prefix collection
	prefixCtx, cancel := context.WithTimeout(ctx, PrefixCollectionTimeout)
	defer cancel()

	prefixes, err := g.listTopLevelPrefixes(prefixCtx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to list top-level prefixes: %w", err)
	}

	if len(prefixes) == 0 {
		return []PrefixStats{}, nil
	}

	log.Printf("Found %d top-level prefixes for bucket %s, processing with worker pool",
		len(prefixes), bucketName)

	prefixStats, err := g.processPrefixes(prefixCtx, bucketName, prefixes)
	if err != nil {
		return nil, err
	}

	// Sort by size descending
	sort.Slice(prefixStats, func(i, j int) bool {
		return prefixStats[i].Size > prefixStats[j].Size
	})

	return prefixStats, nil
}

func (g *StorageReportGenerator) listTopLevelPrefixes(ctx context.Context, bucketName string) ([]string, error) {
	var prefixes []string

	paginator := s3.NewListObjectsV2Paginator(g.s3Client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(1000),
	})

	for paginator.HasMorePages() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled while listing prefixes: %w", err)
		}

		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, commonPrefix := range result.CommonPrefixes {
			prefixes = append(prefixes, aws.ToString(commonPrefix.Prefix))
		}
	}

	return prefixes, nil
}

func (g *StorageReportGenerator) processPrefixes(
	ctx context.Context,
	bucketName string,
	prefixes []string,
) ([]PrefixStats, error) {
	var wg sync.WaitGroup
	prefixQueue := make(chan string, len(prefixes))
	resultsChan := make(chan PrefixStats, len(prefixes))
	errorsChan := make(chan error, len(prefixes))

	for _, prefix := range prefixes {
		prefixQueue <- prefix
	}
	close(prefixQueue)

	numWorkers := PrefixWorkerPoolSize
	if len(prefixes) < numWorkers {
		numWorkers = len(prefixes)
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for prefix := range prefixQueue {
				// Check context before processing each prefix
				if err := ctx.Err(); err != nil {
					errorsChan <- fmt.Errorf("worker %d: context cancelled: %w", workerID, err)
					return
				}

				log.Printf("Prefix worker %d processing: %s/%s", workerID, bucketName, prefix)

				size, count, err := g.calculatePrefixStats(ctx, bucketName, prefix)
				if err != nil {
					errorsChan <- fmt.Errorf("worker %d failed for prefix %s: %w",
						workerID, prefix, err)
					continue
				}

				resultsChan <- PrefixStats{
					Prefix:      prefix,
					Size:        size,
					ObjectCount: count,
				}
			}
		}(i)
	}

	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Collect all results
	var results []PrefixStats
	for stats := range resultsChan {
		results = append(results, stats)
	}

	// Check for errors - if ANY prefix failed, the report is incomplete
	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		for _, err := range errors {
			log.Printf("Prefix processing error: %v", err)
		}
		return nil, fmt.Errorf("failed to process %d/%d prefixes completely - report would be incomplete",
			len(errors), len(prefixes))
	}

	// Verify we got ALL prefixes
	if len(results) != len(prefixes) {
		return nil, fmt.Errorf("incomplete prefix processing: expected %d, got %d",
			len(prefixes), len(results))
	}

	log.Printf("Successfully processed all %d prefixes for bucket %s", len(results), bucketName)
	return results, nil
}

func (g *StorageReportGenerator) calculatePrefixStats(
	ctx context.Context,
	bucketName,
	prefix string,
) (int64, int64, error) {
	var totalSize, totalCount int64
	pageCount := 0

	paginator := s3.NewListObjectsV2Paginator(g.s3Client, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1000), // Max batch size
	})

	for paginator.HasMorePages() {
		// Check context before each page
		if err := ctx.Err(); err != nil {
			return 0, 0, fmt.Errorf("context cancelled after processing %d objects in %s/%s: %w",
				totalCount, bucketName, prefix, err)
		}

		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to list page %d for %s/%s: %w",
				pageCount, bucketName, prefix, err)
		}

		for _, object := range result.Contents {
			totalSize += aws.ToInt64(object.Size)
			totalCount++
		}

		pageCount++

		// Log progress for large prefixes (every 10 pages = 10k objects)
		if pageCount%PrefixProgressLogInterval == 0 {
			log.Printf("Progress: %s/%s - processed %d pages, %d objects, %.2f GB so far",
				bucketName, prefix, pageCount, totalCount,
				float64(totalSize)/(1024*1024*1024))
		}
	}

	log.Printf("Completed: %s/%s - %d objects, %.2f GB",
		bucketName, prefix, totalCount, float64(totalSize)/(1024*1024*1024))

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
