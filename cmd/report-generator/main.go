package main

import (
	"context"
	"duracloud/internal/reports"
	"duracloud/internal/templates"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	//go:embed templates/storage-report.html
	storageReportTemplate string

	cloudWatchClient  *cloudwatch.Client
	managedBucketName string
	s3Client          *s3.Client
	stackName         string
	storageReportTmpl *template.Template
)

func init() {
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), 5)
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("Unable to load AWS config: %v", err))
	}

	storageReportTmpl, err = template.New("storage-report").
		Funcs(templates.GetReportGeneratorFuncMap()).
		Parse(storageReportTemplate)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse storage report template: %v", err))
	}

	managedBucketName = os.Getenv("S3_MANAGED_BUCKET")
	stackName = os.Getenv("STACK_NAME")
	cloudWatchClient = cloudwatch.NewFromConfig(awsConfig)
	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context) error {
	log.Printf("Starting storage report generation for stack: %s", stackName)

	generator := reports.NewStorageReportGenerator(s3Client, cloudWatchClient, stackName)

	reportHTML, err := generator.GenerateReport(ctx, storageReportTmpl)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if reportHTML == "" {
		log.Println("No buckets found for report generation")
		return nil
	}

	// Upload report to the managed bucket
	reportKey := fmt.Sprintf("reports/%s-storage-report.html",
		time.Now().Format("2006-01-02T15-04-05"))

	err = generator.UploadReport(ctx, managedBucketName, reportKey, reportHTML)
	if err != nil {
		return fmt.Errorf("failed to upload report: %w", err)
	}

	log.Printf("Storage report uploaded to s3://%s/%s", managedBucketName, reportKey)
	return nil
}

func main() {
	lambda.Start(handler)
}
