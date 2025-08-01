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
  description = "Docker image for Bucket Requested function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "checksum_exporter_image_uri" {
  description = "Docker image for Checksum Exporter function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "checksum_export_csv_report_image_uri" {
  description = "Docker image for Checksum Export CSV report function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "checksum_failure_image_uri" {
  description = "Docker image for Checksum Failure function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "checksum_verification_image_uri" {
  description = "Docker image for Checksum Verification function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "file_deleted_image_uri" {
  description = "Docker image for File Deleted function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "file_uploaded_image_uri" {
  description = "Docker image for File Uploaded function (leave empty for local build)"
  type        = string
  default     = ""
}

variable "report_generator_image_uri" {
  description = "Docker image for Report Generator function (leave empty for local build)"
  type        = string
  default     = ""
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