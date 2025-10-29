# Development main.tf (this is for dev/test only)
# See the documentation for production deployment instructions
terraform {
  backend "s3" {}
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {}

variable "arch" {}
variable "email" { default = "" }
variable "repo" {}
variable "stack" {}

module "duracloud" {
  source = "./terraform/modules/duracloud"

  stack_name                         = var.stack
  alert_email_address                = var.email
  checksum_exporter_schedule         = "cron(0 6 * * ? *)"
  checksum_export_csv_report_storage = 512
  lambda_architecture                = var.arch
  report_generator_schedule          = "cron(0 8 * * ? *)"

  bucket_requested_image_uri           = "${var.repo}/bucket-requested:${var.stack}"
  checksum_exporter_image_uri          = "${var.repo}/checksum-exporter:${var.stack}"
  checksum_export_csv_report_image_uri = "${var.repo}/checksum-export-csv-report:${var.stack}"
  checksum_failure_image_uri           = "${var.repo}/checksum-failure:${var.stack}"
  checksum_verification_image_uri      = "${var.repo}/checksum-verification:${var.stack}"
  file_deleted_image_uri               = "${var.repo}/file-deleted:${var.stack}"
  file_uploaded_image_uri              = "${var.repo}/file-uploaded:${var.stack}"
  report_generator_image_uri           = "${var.repo}/report-generator:${var.stack}"
}
