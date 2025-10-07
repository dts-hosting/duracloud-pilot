variable "stack_name" {
  description = "Stack name prefix for resources"
  type        = string
}

variable "alert_email_address" {
  description = "Email address for alarm notifications (leave empty to disable email alerts)"
  type        = string
  default     = ""
}

variable "bucket_requested_image_uri" {
  description = "Docker image for Bucket Requested function"
  type        = string
  default     = "docker.io/duracloud/bucket-requested:latest"
}

variable "checksum_exporter_image_uri" {
  description = "Docker image for Checksum Exporter function"
  type        = string
  default     = "docker.io/duracloud/checksum-exporter:latest"
}

variable "checksum_exporter_schedule" {
  description = "Cron schedule for checksum table exports"
  type        = string
  default     = "cron(0 8 1 * ? *)"
}

variable "checksum_export_csv_report_image_uri" {
  description = "Docker image for Checksum Export CSV report function"
  type        = string
  default     = "docker.io/duracloud/checksum-export-csv-report:latest"
}

variable "checksum_failure_image_uri" {
  description = "Docker image for Checksum Failure function"
  type        = string
  default     = "docker.io/duracloud/checksum-failure:latest"
}

variable "checksum_verification_image_uri" {
  description = "Docker image for Checksum Verification function"
  type        = string
  default     = "docker.io/duracloud/checksum-verification:latest"
}

variable "file_deleted_image_uri" {
  description = "Docker image for File Deleted function"
  type        = string
  default     = "docker.io/duracloud/file-deleted:latest"
}

variable "file_uploaded_image_uri" {
  description = "Docker image for File Uploaded function"
  type        = string
  default     = "docker.io/duracloud/file-uploaded:latest"
}

variable "report_generator_image_uri" {
  description = "Docker image for Report Generator function"
  type        = string
  default     = "docker.io/duracloud/report-generator:latest"
}

variable "report_generator_schedule" {
  description = "Cron schedule for storage report generation"
  type        = string
  default     = "cron(0 8 ? * SUN *)"
}

variable "lambda_architecture" {
  description = "Architecture for Lambda functions"
  type        = string
  default     = "x86_64"
  validation {
    condition     = contains(["arm64", "x86_64"], var.lambda_architecture)
    error_message = "Lambda architecture must be either arm64 or x86_64."
  }
}
