# DuraCloud Terraform Module

This Terraform module creates a complete DuraCloud infrastructure on AWS, including Lambda functions, DynamoDB tables, S3 buckets, IAM roles, CloudWatch alarms, and EventBridge rules.

## Features

- **Lambda Functions**: 8 Lambda functions for processing various DuraCloud operations
- **DynamoDB Tables**: Checksum and scheduler tables with streams and TTL
- **S3 Buckets**: Managed and bucket-requested buckets with notifications
- **SQS Queues**: Object created/deleted queues with dead letter queues
- **SNS Topics**: Email alert notifications (optional)
- **IAM Roles**: Least-privilege roles for all Lambda functions and services
- **CloudWatch Alarms**: Monitoring for Lambda errors, DynamoDB capacity, and SQS DLQs
- **EventBridge Rules**: Scheduled and S3 event-driven processing

## Usage

```hcl
module "duracloud" {
  source = "./terraform/modules/duracloud"

  stack_name          = "duracloud-dev"
  alert_email_address = "admin@example.com"
  lambda_architecture = "x86_64"

  # Optional: Specify Docker image URIs (leave empty for local builds)
  bucket_requested_image_uri           = ""
  checksum_exporter_image_uri          = ""
  checksum_export_csv_report_image_uri = ""
  checksum_failure_image_uri           = ""
  checksum_verification_image_uri      = ""
  file_deleted_image_uri               = ""
  file_uploaded_image_uri              = ""
  inventory_unwrap_img_uri             = ""
  report_generator_image_uri           = ""
}
```

## Requirements

| Name      | Version |
| --------- | ------- |
| terraform | >= 1.0  |

## Providers

| Name | Version |
| ---- | ------- |
| aws  | ~> 6.0  |

## Inputs

Review the [variables](variables.tf) file.

## Outputs

Review the [outputs](outputs.tf) file.

## Resources Created

### Lambda Functions

- **Bucket Requested Function**: Processes bucket requested events
- **Checksum Exporter Function**: Exports DynamoDB checksum table
- **Checksum Export CSV Report Function**: Writes CSV reports of DynamoDB table exports
- **Checksum Failure Function**: Processes checksum failure events
- **Checksum Verification Function**: Processes checksum verification via TTL events
- **File Deleted Function**: Processes S3 object deleted events
- **File Uploaded Function**: Processes S3 object uploaded events
- **Inventory Unwrap Function**: Converts csv.gz to .csv with headers, generates stats
- **Report Generator Function**: Generates storage stats reports

### DynamoDB Tables

- **Checksum Table**: Stores file checksums with streams enabled
- **Checksum Scheduler Table**: Manages checksum verification scheduling with TTL

### S3 Buckets

- **Managed Bucket**: Primary storage bucket for DuraCloud
- **Bucket Requested Bucket**: Handles bucket creation requests

### SQS Queues

- **Object Created Queue**: Processes S3 object creation events
- **Object Deleted Queue**: Processes S3 object deletion events
- **Dead Letter Queues**: For failed message processing

### CloudWatch Alarms

- Lambda function error monitoring
- Lambda concurrency monitoring
- DynamoDB write capacity monitoring
- SQS dead letter queue monitoring

Review [cloudwatch.tf] for full details.

### EventBridge Rules

- Scheduled checksum exports (monthly)
- Scheduled report generation (weekly)
- S3 object created/deleted event processing

Review [eventbridge.tf] for full details.

## Resource Naming

All resources are prefixed with the `stack_name` variable to ensure uniqueness and avoid conflicts. For example, with `stack_name = "duracloud-dev"`:

- S3 buckets: `duracloud-dev-managed`, `duracloud-dev-bucket-requested`
- Lambda functions: `duracloud-dev-bucket-requested-function`, etc.
- DynamoDB tables: `duracloud-dev-checksum-table`, etc.
- IAM roles: `duracloud-dev-bucket-requested-function-role`, etc.

## Email Alerts

Email alerts are optional and controlled by the `alert_email_address` variable:

- If provided, an SNS topic is created and CloudWatch alarms will send notifications
- If empty, no SNS topic is created and alarms will not send email notifications

The alert email address will receive a confirmation email that has to be accepted.

## License

This module is part of the DuraCloud project.
