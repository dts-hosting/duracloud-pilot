terraform {
  required_version = ">= 1.4, < 2.0.0"

  cloud {
    organization = "Lyrasis"
    workspaces {
      tags = ["duracloud", "sites"]
    }
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

# These are workspace variables. Declared but not defined here.
variable "department" {}
variable "dns_account_id" {}
variable "environment" {}
variable "project_account_id" {}
variable "region" {}
variable "role" {}
variable "service" {}

# These are project variables set directly for testing.
variable "stacks" {
  description = "Configuration for DuraCloud stacks"
  type = map(object({
    alert_email_address = string
    lambda_architecture = string
  }))
  validation {
    condition = alltrue([
      for stack in values(var.stacks) : contains(["arm64", "x86_64"], stack.lambda_architecture)
    ])
    error_message = "Lambda architecture must be either 'arm64' or 'x86_64'."
  }
}

provider "aws" {
  region              = var.region
  allowed_account_ids = [var.project_account_id]

  assume_role {
    role_arn     = "arn:aws:iam::${var.project_account_id}:role/${var.role}"
    session_name = "${var.service}-${var.department}-${var.environment}"
    external_id  = "${var.service}-${var.department}-${var.environment}"
  }

  default_tags {
    tags = {
      Service     = var.service
      Department  = var.department
      Environment = var.environment
      Terraform   = true
    }
  }
}

module "duracloud" {
  source = "../../terraform/modules/duracloud"

  for_each = var.stacks

  stack_name                 = each.key
  alert_email_address        = each.value.alert_email_address
  checksum_exporter_schedule = coalesce(each.key.checksum_exporter_schedule, null)
  lambda_architecture        = each.value.lambda_architecture
  report_generator_schedule  = coalesce(each.key.report_generator_schedule, null)

  bucket_requested_image_uri           = "duracloud/bucket-requested:latest"
  checksum_exporter_image_uri          = "duracloud/checksum-exporter:latest"
  checksum_export_csv_report_image_uri = "duracloud/checksum-export-csv-report:latest"
  checksum_failure_image_uri           = "duracloud/checksum-failure:latest"
  checksum_verification_image_uri      = "duracloud/checksum-verification:latest"
  file_deleted_image_uri               = "duracloud/file-deleted:latest"
  file_uploaded_image_uri              = "duracloud/file-uploaded:latest"
  report_generator_image_uri           = "duracloud/report-generator:latest"
}
