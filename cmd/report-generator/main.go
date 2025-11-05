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
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	//go:embed templates/storage-report.html
	storageReportTemplate string

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
	s3Client = s3.NewFromConfig(awsConfig)
}

func handler(ctx context.Context) error {
	log.Printf("Starting storage report generation for stack: %s", stackName)

	generator := reports.NewStorageReportGenerator(s3Client, stackName, managedBucketName)

	reportHTML, err := generator.GenerateReport(ctx, storageReportTmpl)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if reportHTML == "" {
		log.Println("No buckets found for report generation")
		return nil
	}

	// Upload report to the managed bucket (date only, one per day)
	reportKey := fmt.Sprintf("reports/storage-report-%s.html",
		time.Now().UTC().Format("2006-01-02"))

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
