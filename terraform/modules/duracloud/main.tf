terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  stack_name = var.stack_name

  # Conditional logic for external images
  bucket_requested_image_uri           = var.bucket_requested_image_uri != "" ? var.bucket_requested_image_uri : null
  checksum_exporter_image_uri          = var.checksum_exporter_image_uri != "" ? var.checksum_exporter_image_uri : null
  checksum_export_csv_report_image_uri = var.checksum_export_csv_report_image_uri != "" ? var.checksum_export_csv_report_image_uri : null
  checksum_failure_image_uri           = var.checksum_failure_image_uri != "" ? var.checksum_failure_image_uri : null
  checksum_verification_image_uri      = var.checksum_verification_image_uri != "" ? var.checksum_verification_image_uri : null
  file_deleted_image_uri               = var.file_deleted_image_uri != "" ? var.file_deleted_image_uri : null
  file_uploaded_image_uri              = var.file_uploaded_image_uri != "" ? var.file_uploaded_image_uri : null
  report_generator_image_uri           = var.report_generator_image_uri != "" ? var.report_generator_image_uri : null

  enable_email_alerts = var.alert_email_address != ""
}