output "stack_name" {
  description = "The stack name used for resource naming"
  value       = local.stack_name
}

output "managed_bucket_name" {
  description = "Name of the managed S3 bucket"
  value       = aws_s3_bucket.managed_bucket.bucket
}

output "bucket_requested_name" {
  description = "Name of the bucket requested S3 bucket"
  value       = aws_s3_bucket.bucket_requested.bucket
}

output "checksum_table_name" {
  description = "Name of the DynamoDB checksum table"
  value       = aws_dynamodb_table.checksum_table.name
}

output "checksum_scheduler_table_name" {
  description = "Name of the DynamoDB checksum scheduler table"
  value       = aws_dynamodb_table.checksum_scheduler_table.name
}

output "sns_topic_arn" {
  description = "ARN of the SNS email alert topic"
  value       = local.enable_email_alerts ? aws_sns_topic.email_alert_topic[0].arn : null
}

output "lambda_functions" {
  description = "Map of Lambda function names and ARNs"
  value = {
    bucket_requested_function           = aws_lambda_function.bucket_requested_function.arn
    checksum_exporter_function          = aws_lambda_function.checksum_exporter_function.arn
    checksum_export_csv_report_function = aws_lambda_function.checksum_export_csv_report_function.arn
    checksum_failure_function           = aws_lambda_function.checksum_failure_function.arn
    checksum_verification_function      = aws_lambda_function.checksum_verification_function.arn
    file_deleted_function               = aws_lambda_function.file_deleted_function.arn
    file_uploaded_function              = aws_lambda_function.file_uploaded_function.arn
    report_generator_function           = aws_lambda_function.report_generator_function.arn
  }
}