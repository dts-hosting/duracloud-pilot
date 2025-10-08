# CloudWatch Log Groups
resource "aws_cloudwatch_log_group" "bucket_requested_function" {
  name              = "/aws/lambda/${local.stack_name}-bucket-requested"
  retention_in_days = 7

  tags = {
    Name = "${local.stack_name}-bucket-requested-logs"
  }
}

resource "aws_cloudwatch_log_group" "checksum_exporter_function" {
  name              = "/aws/lambda/${local.stack_name}-checksum-exporter"
  retention_in_days = 30

  tags = {
    Name = "${local.stack_name}-checksum-exporter-logs"
  }
}

resource "aws_cloudwatch_log_group" "checksum_export_csv_report_function" {
  name              = "/aws/lambda/${local.stack_name}-checksum-export-csv-report"
  retention_in_days = 30

  tags = {
    Name = "${local.stack_name}-checksum-export-csv-report-logs"
  }
}

resource "aws_cloudwatch_log_group" "checksum_failure_function" {
  name              = "/aws/lambda/${local.stack_name}-checksum-failure"
  retention_in_days = 7

  tags = {
    Name = "${local.stack_name}-checksum-failure-logs"
  }
}

resource "aws_cloudwatch_log_group" "checksum_verification_function" {
  name              = "/aws/lambda/${local.stack_name}-checksum-verification"
  retention_in_days = 7

  tags = {
    Name = "${local.stack_name}-checksum-verification-logs"
  }
}

resource "aws_cloudwatch_log_group" "file_deleted_function" {
  name              = "/aws/lambda/${local.stack_name}-file-deleted"
  retention_in_days = 7

  tags = {
    Name = "${local.stack_name}-file-deleted-logs"
  }
}

resource "aws_cloudwatch_log_group" "file_uploaded_function" {
  name              = "/aws/lambda/${local.stack_name}-file-uploaded"
  retention_in_days = 7

  tags = {
    Name = "${local.stack_name}-file-uploaded-logs"
  }
}

resource "aws_cloudwatch_log_group" "report_generator_function" {
  name              = "/aws/lambda/${local.stack_name}-report-generator"
  retention_in_days = 7

  tags = {
    Name = "${local.stack_name}-report-generator-logs"
  }
}

# Lambda Functions
resource "aws_lambda_function" "bucket_requested_function" {
  function_name = "${local.stack_name}-bucket-requested"
  role          = aws_iam_role.bucket_requested_function_role.arn
  image_uri     = local.bucket_requested_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 60
  memory_size   = 128
  description   = "DuraCloud function that processes bucket requested events"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.bucket_requested_function.name
  }

  environment {
    variables = {
      S3_BUCKET_PREFIX           = local.stack_name
      S3_INVENTORY_DEST_BUCKET   = aws_s3_bucket.managed_bucket.bucket
      S3_MANAGED_BUCKET          = aws_s3_bucket.managed_bucket.bucket
      S3_MAX_BUCKETS_PER_REQUEST = "5"
      S3_REPLICATION_ROLE_ARN    = aws_iam_role.s3_replication_role.arn
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.bucket_requested_function_basic,
    aws_iam_role_policy.bucket_requested_function_policy,
    aws_cloudwatch_log_group.bucket_requested_function,
  ]

  tags = {
    Name = "${local.stack_name}-bucket-requested-function"
  }
}

resource "aws_lambda_function" "checksum_exporter_function" {
  function_name = "${local.stack_name}-checksum-exporter"
  role          = aws_iam_role.checksum_exporter_function_role.arn
  image_uri     = local.checksum_exporter_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 900
  memory_size   = 256
  description   = "DuraCloud function that exports DynamoDB checksum table"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.checksum_exporter_function.name
  }

  environment {
    variables = {
      DYNAMODB_CHECKSUM_TABLE = aws_dynamodb_table.checksum_table.name
      S3_MANAGED_BUCKET       = aws_s3_bucket.managed_bucket.bucket
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.checksum_exporter_function_basic,
    aws_iam_role_policy.checksum_exporter_function_policy,
    aws_cloudwatch_log_group.checksum_exporter_function,
  ]

  tags = {
    Name = "${local.stack_name}-checksum-exporter-function"
  }
}

resource "aws_lambda_function" "checksum_export_csv_report_function" {
  function_name = "${local.stack_name}-checksum-export-csv-report"
  role          = aws_iam_role.checksum_export_csv_report_function_role.arn
  image_uri     = local.checksum_export_csv_report_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 900
  memory_size   = 256
  description   = "DuraCloud function that writes CSV Reports of DynamoDB table exports"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.checksum_export_csv_report_function.name
  }

  depends_on = [
    aws_iam_role_policy_attachment.checksum_export_csv_report_function_basic,
    aws_iam_role_policy.checksum_export_csv_report_function_policy,
    aws_cloudwatch_log_group.checksum_export_csv_report_function,
  ]

  tags = {
    Name = "${local.stack_name}-checksum-export-csv-report-function"
  }
}

resource "aws_lambda_function" "checksum_failure_function" {
  function_name = "${local.stack_name}-checksum-failure"
  role          = aws_iam_role.checksum_failure_function_role.arn
  image_uri     = local.checksum_failure_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 60
  memory_size   = 128
  description   = "DuraCloud function that processes checksum failure events"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.checksum_failure_function.name
  }

  environment {
    variables = {
      S3_MANAGED_BUCKET = aws_s3_bucket.managed_bucket.bucket
      SNS_TOPIC_ARN     = aws_sns_topic.email_alert_topic.arn
      STACK_NAME        = local.stack_name
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.checksum_failure_function_basic,
    aws_iam_role_policy.checksum_failure_function_policy,
    aws_cloudwatch_log_group.checksum_failure_function,
  ]

  tags = {
    Name = "${local.stack_name}-checksum-failure-function"
  }
}

resource "aws_lambda_function" "checksum_verification_function" {
  function_name = "${local.stack_name}-checksum-verification"
  role          = aws_iam_role.checksum_verification_function_role.arn
  image_uri     = local.checksum_verification_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 900
  memory_size   = 1024
  description   = "DuraCloud function that processes checksum verification via TTL events"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.checksum_verification_function.name
  }

  environment {
    variables = {
      DYNAMODB_CHECKSUM_TABLE  = aws_dynamodb_table.checksum_table.name
      DYNAMODB_SCHEDULER_TABLE = aws_dynamodb_table.checksum_scheduler_table.name
      SNS_TOPIC_ARN            = aws_sns_topic.email_alert_topic.arn
      STACK_NAME               = local.stack_name
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.checksum_verification_function_basic,
    aws_iam_role_policy.checksum_verification_function_policy,
    aws_cloudwatch_log_group.checksum_verification_function,
  ]

  tags = {
    Name = "${local.stack_name}-checksum-verification-function"
  }
}

resource "aws_lambda_function" "file_deleted_function" {
  function_name = "${local.stack_name}-file-deleted"
  role          = aws_iam_role.file_deleted_function_role.arn
  image_uri     = local.file_deleted_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 60
  memory_size   = 128
  description   = "DuraCloud function that processes s3 object deleted events"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.file_deleted_function.name
  }

  environment {
    variables = {
      DYNAMODB_CHECKSUM_TABLE  = aws_dynamodb_table.checksum_table.name
      DYNAMODB_SCHEDULER_TABLE = aws_dynamodb_table.checksum_scheduler_table.name
      S3_BUCKET_PREFIX         = local.stack_name
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.file_deleted_function_basic,
    aws_iam_role_policy.file_deleted_function_policy,
    aws_cloudwatch_log_group.file_deleted_function,
  ]

  tags = {
    Name = "${local.stack_name}-file-deleted-function"
  }
}

resource "aws_lambda_function" "file_uploaded_function" {
  function_name = "${local.stack_name}-file-uploaded"
  role          = aws_iam_role.file_uploaded_function_role.arn
  image_uri     = local.file_uploaded_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 900
  memory_size   = 1024
  description   = "DuraCloud function that processes s3 object uploaded events"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.file_uploaded_function.name
  }

  environment {
    variables = {
      DYNAMODB_CHECKSUM_TABLE  = aws_dynamodb_table.checksum_table.name
      DYNAMODB_SCHEDULER_TABLE = aws_dynamodb_table.checksum_scheduler_table.name
      S3_BUCKET_PREFIX         = local.stack_name
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.file_uploaded_function_basic,
    aws_iam_role_policy.file_uploaded_function_policy,
    aws_cloudwatch_log_group.file_uploaded_function,
  ]

  tags = {
    Name = "${local.stack_name}-file-uploaded-function"
  }
}

resource "aws_lambda_function" "report_generator_function" {
  function_name = "${local.stack_name}-report-generator"
  role          = aws_iam_role.report_generator_function_role.arn
  image_uri     = local.report_generator_image_uri
  package_type  = "Image"
  architectures = [local.lambda_architecture]
  timeout       = 900
  memory_size   = 256
  description   = "DuraCloud function that generates storage stats report"

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.report_generator_function.name
  }

  environment {
    variables = {
      S3_MANAGED_BUCKET = aws_s3_bucket.managed_bucket.bucket
      STACK_NAME        = local.stack_name
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.report_generator_function_basic,
    aws_iam_role_policy.report_generator_function_policy,
    aws_cloudwatch_log_group.report_generator_function,
  ]

  tags = {
    Name = "${local.stack_name}-report-generator-function"
  }
}

# Lambda Event Source Mappings
resource "aws_lambda_event_source_mapping" "dynamodb_checksum_failure_source" {
  event_source_arn                   = aws_dynamodb_table.checksum_table.stream_arn
  function_name                      = aws_lambda_function.checksum_failure_function.arn
  starting_position                  = "TRIM_HORIZON"
  batch_size                         = 10
  maximum_batching_window_in_seconds = 5

  filter_criteria {
    filter {
      pattern = jsonencode({
        eventName = ["INSERT", "MODIFY"]
        dynamodb = {
          NewImage = {
            LastChecksumSuccess = {
              BOOL = [false]
            }
          }
        }
      })
    }
  }

  depends_on = [
    aws_lambda_function.checksum_failure_function,
    aws_dynamodb_table.checksum_table
  ]
}

resource "aws_lambda_event_source_mapping" "dynamodb_checksum_scheduler_source" {
  event_source_arn                   = aws_dynamodb_table.checksum_scheduler_table.stream_arn
  function_name                      = aws_lambda_function.checksum_verification_function.arn
  starting_position                  = "TRIM_HORIZON"
  batch_size                         = 10
  maximum_batching_window_in_seconds = 5

  filter_criteria {
    filter {
      pattern = jsonencode({
        eventName = ["REMOVE"]
      })
    }
  }

  depends_on = [
    aws_lambda_function.checksum_verification_function,
    aws_dynamodb_table.checksum_scheduler_table
  ]
}

resource "aws_lambda_event_source_mapping" "sqs_object_created_source" {
  event_source_arn                   = aws_sqs_queue.object_created.arn
  function_name                      = aws_lambda_function.file_uploaded_function.arn
  batch_size                         = 10
  maximum_batching_window_in_seconds = 5
  function_response_types            = ["ReportBatchItemFailures"]

  depends_on = [
    aws_lambda_function.file_uploaded_function,
    aws_sqs_queue.object_created
  ]
}

resource "aws_lambda_event_source_mapping" "sqs_object_deleted_source" {
  event_source_arn                   = aws_sqs_queue.object_deleted.arn
  function_name                      = aws_lambda_function.file_deleted_function.arn
  batch_size                         = 10
  maximum_batching_window_in_seconds = 5
  function_response_types            = ["ReportBatchItemFailures"]

  depends_on = [
    aws_lambda_function.file_deleted_function,
    aws_sqs_queue.object_deleted
  ]
}

# Lambda Permissions
resource "aws_lambda_permission" "bucket_request_invoke_permission" {
  statement_id  = "AllowExecutionFromS3Bucket"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.bucket_requested_function.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.bucket_requested.arn

  depends_on = [aws_lambda_function.bucket_requested_function]
}

resource "aws_lambda_permission" "checksum_exporter_invoke_permission" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.checksum_exporter_function.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.checksum_exporter_schedule.arn

  depends_on = [aws_lambda_function.checksum_exporter_function]
}

resource "aws_lambda_permission" "s3_managed_bucket_invoke_permission" {
  statement_id   = "AllowExecutionFromS3Bucket"
  action         = "lambda:InvokeFunction"
  function_name  = aws_lambda_function.checksum_export_csv_report_function.function_name
  principal      = "s3.amazonaws.com"
  source_account = data.aws_caller_identity.current.account_id
  source_arn     = aws_s3_bucket.managed_bucket.arn

  depends_on = [aws_lambda_function.checksum_export_csv_report_function]
}


resource "aws_lambda_permission" "report_generator_invoke_permission" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.report_generator_function.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.report_generator_schedule.arn

  depends_on = [aws_lambda_function.report_generator_function]
}