terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  stack_name                         = var.stack_name
  checksum_exporter_schedule         = coalesce(var.checksum_exporter_schedule, null)
  checksum_export_csv_report_storage = var.checksum_export_csv_report_storage
  enable_email_alerts                = var.alert_email_address != ""
  lambda_architecture                = var.lambda_architecture
  report_generator_schedule          = coalesce(var.report_generator_schedule, null)

  # Conditional logic for external images
  bucket_requested_image_uri           = coalesce(var.bucket_requested_image_uri, null)
  checksum_exporter_image_uri          = coalesce(var.checksum_exporter_image_uri, null)
  checksum_export_csv_report_image_uri = coalesce(var.checksum_export_csv_report_image_uri, null)
  checksum_failure_image_uri           = coalesce(var.checksum_failure_image_uri, null)
  checksum_verification_image_uri      = coalesce(var.checksum_verification_image_uri, null)
  file_deleted_image_uri               = coalesce(var.file_deleted_image_uri, null)
  file_uploaded_image_uri              = coalesce(var.file_uploaded_image_uri, null)
  report_generator_image_uri           = coalesce(var.report_generator_image_uri, null)
}
