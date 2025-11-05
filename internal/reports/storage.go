package reports

import (
	"bytes"
	"context"
	"duracloud/internal/files"
	"duracloud/internal/inventory"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type BucketStats struct {
	Name             string
	TotalSize        int64
	TotalObjects     int64
	PrefixStats      []PrefixStats
	Tags             map[string]string
	StatsDate        string
	StatsGeneratedAt time.Time
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

type StorageReportGenerator struct {
	s3Client          *s3.Client
	stackName         string
	managedBucketName string
}

func NewStorageReportGenerator(s3Client *s3.Client, stackName, managedBucketName string) *StorageReportGenerator {
	return &StorageReportGenerator{
		s3Client:          s3Client,
		stackName:         stackName,
		managedBucketName: managedBucketName,
	}
}

func (g *StorageReportGenerator) GenerateReport(ctx context.Context, tmpl *template.Template) (string, error) {
	log.Println("Finding buckets with stats")
	statsReader := NewBucketStatsReader(ctx, g.s3Client, g.stackName)

	buckets, err := statsReader.FindBucketsWithStats(g.managedBucketName)
	if err != nil {
		return "", fmt.Errorf("failed to find buckets with stats: %w", err)
	}

	if len(buckets) == 0 {
		log.Println("No buckets with stats found")
		return "", nil
	}

	log.Printf("Loading stats for %d buckets", len(buckets))
	bucketStats, err := g.loadBucketStats(statsReader, buckets)
	if err != nil {
		return "", fmt.Errorf("failed to load bucket stats: %w", err)
	}

	if len(bucketStats) == 0 {
		log.Println("No buckets with stats found")
		return "", nil
	}

	log.Println("Calculating aggregates")
	reportData := g.calculateAggregates(bucketStats)

	log.Println("Generating HTML report")
	htmlReport, err := g.generateHTML(reportData, tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML: %w", err)
	}

	return htmlReport, nil
}

func (g *StorageReportGenerator) UploadReport(ctx context.Context, bucketName, key, content string) error {
	log.Printf("Uploading report to bucket: %s/%s", bucketName, key)
	obj := files.NewS3Object(bucketName, key)
	err := files.UploadObject(ctx, g.s3Client, obj, strings.NewReader(content), "text/html")

	return err
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
		report.TotalSize += bucket.TotalSize
		report.TotalObjects += bucket.TotalObjects
	}

	return report
}

func (g *StorageReportGenerator) convertInventoryStats(stats *inventory.InventoryStats, tags map[string]string) BucketStats {
	// Convert prefix stats map to sorted slice
	var prefixStats []PrefixStats
	for prefix, stat := range stats.PrefixStats {
		prefixStats = append(prefixStats, PrefixStats{
			Prefix:      prefix,
			Size:        stat.Bytes,
			ObjectCount: stat.Count,
		})
	}

	// Sort by size descending
	sort.Slice(prefixStats, func(i, j int) bool {
		return prefixStats[i].Size > prefixStats[j].Size
	})

	return BucketStats{
		Name:             stats.BucketName,
		TotalSize:        stats.TotalBytes,
		TotalObjects:     stats.TotalCount,
		PrefixStats:      prefixStats,
		Tags:             tags,
		StatsDate:        stats.InventoryDate,
		StatsGeneratedAt: stats.GeneratedAt,
	}
}

func (g *StorageReportGenerator) generateHTML(data ReportData, tmpl *template.Template) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (g *StorageReportGenerator) loadBucketStats(
	statsReader *BucketStatsReader,
	buckets []string,
) ([]BucketStats, error) {
	var allStats []BucketStats

	for _, bucketName := range buckets {
		log.Printf("Loading stats for bucket: %s", bucketName)

		// Get the latest stats file
		stats, err := statsReader.GetLatestStats(bucketName, g.managedBucketName)
		if err != nil {
			log.Printf("Warning: failed to get stats for %s: %v (skipping)", bucketName, err)
			continue
		}

		// Get bucket tags
		tags, err := statsReader.GetBucketTags(bucketName)
		if err != nil {
			log.Printf("Warning: failed to get tags for %s: %v", bucketName, err)
			tags = make(map[string]string)
		}

		// Convert inventory stats to report stats
		bucketStats := g.convertInventoryStats(stats, tags)
		allStats = append(allStats, bucketStats)
	}

	// Sort by bucket name
	sort.Slice(allStats, func(i, j int) bool {
		return allStats[i].Name < allStats[j].Name
	})

	return allStats, nil
}
