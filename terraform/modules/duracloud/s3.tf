resource "aws_s3_bucket" "managed_bucket" {
  bucket = "${local.stack_name}-managed"

  tags = {
    Name = "${local.stack_name}-managed"
  }
}

resource "aws_s3_bucket_notification" "managed_bucket_notification" {
  bucket     = aws_s3_bucket.managed_bucket.id
  depends_on = [aws_lambda_permission.s3_managed_bucket_invoke_permission]

  lambda_function {
    lambda_function_arn = aws_lambda_function.checksum_export_csv_report_function.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "exports/checksum-table/"
    filter_suffix       = ".json.gz"
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "managed_bucket_lifecycle" {
  bucket = aws_s3_bucket.managed_bucket.id

  rule {
    id     = "DeleteAfter30Days"
    status = "Enabled"

    filter {}

    expiration {
      days = 30
    }

    noncurrent_version_expiration {
      noncurrent_days = 1
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 1
    }
  }

  depends_on = [aws_s3_bucket.managed_bucket]
}

resource "aws_s3_bucket" "bucket_requested" {
  bucket = "${local.stack_name}-bucket-requested"

  tags = {
    Name = "${local.stack_name}-bucket-requested"
  }
}

resource "aws_s3_bucket" "logs_bucket" {
  bucket = "${local.stack_name}-logs"

  tags = {
    Name = "${local.stack_name}-logs"
  }
}

resource "aws_s3_bucket_notification" "bucket_requested_notification" {
  bucket      = aws_s3_bucket.bucket_requested.id
  eventbridge = true

  lambda_function {
    lambda_function_arn = aws_lambda_function.bucket_requested_function.arn
    events              = ["s3:ObjectCreated:*"]
  }

  depends_on = [aws_lambda_permission.bucket_request_invoke_permission]
}

# S3 Bucket Policy for managed bucket
resource "aws_s3_bucket_policy" "managed_bucket_policy" {
  bucket = aws_s3_bucket.managed_bucket.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowAuditDestination"
        Effect = "Allow"
        Principal = {
          Service = "logging.s3.amazonaws.com"
        }
        Action = [
          "s3:PutObject"
        ]
        Resource = "${aws_s3_bucket.managed_bucket.arn}/audit/*"
        Condition = {
          ArnLike = {
            "aws:SourceArn" = "arn:aws:s3:::${local.stack_name}*"
          }
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
        }
      },
      {
        Sid    = "AllowInventoryDestination"
        Effect = "Allow"
        Principal = {
          Service = "s3.amazonaws.com"
        }
        Action = [
          "s3:PutObject"
        ]
        Resource = "${aws_s3_bucket.managed_bucket.arn}/inventory/*"
        Condition = {
          ArnLike = {
            "aws:SourceArn" = "arn:aws:s3:::${local.stack_name}*"
          }
          StringEquals = {
            "s3:x-amz-acl" = "bucket-owner-full-control"
          }
        }
      }
    ]
  })

  depends_on = [aws_s3_bucket.managed_bucket]
}

# SQS Queues for S3 event processing
resource "aws_sqs_queue" "object_created_dlq" {
  name                      = "${local.stack_name}-object-created-dlq"
  message_retention_seconds = 1209600 # 14 days

  tags = {
    Name = "${local.stack_name}-object-created-dlq"
  }
}

resource "aws_sqs_queue" "object_created" {
  name                       = "${local.stack_name}-object-created"
  visibility_timeout_seconds = 960 # 16 minutes (Lambda timeout + buffer)
  receive_wait_time_seconds  = 20

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.object_created_dlq.arn
    maxReceiveCount     = 3
  })

  tags = {
    Name = "${local.stack_name}-object-created"
  }
}

resource "aws_sqs_queue" "object_deleted_dlq" {
  name                      = "${local.stack_name}-object-deleted-dlq"
  message_retention_seconds = 1209600 # 14 days

  tags = {
    Name = "${local.stack_name}-object-deleted-dlq"
  }
}

resource "aws_sqs_queue" "object_deleted" {
  name                       = "${local.stack_name}-object-deleted"
  visibility_timeout_seconds = 300 # Lambda timeout + buffer
  receive_wait_time_seconds  = 20

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.object_deleted_dlq.arn
    maxReceiveCount     = 3
  })

  tags = {
    Name = "${local.stack_name}-object-deleted"
  }
}

# SNS Topic for email alerts
resource "aws_sns_topic" "email_alert_topic" {
  name = "${local.stack_name}-email-alert-notifications"

  tags = {
    Name = "${local.stack_name}-email-alert-notifications"
  }
}

resource "aws_sns_topic_subscription" "email_alert_subscription" {
  count     = local.enable_email_alerts ? 1 : 0
  topic_arn = aws_sns_topic.email_alert_topic.arn
  protocol  = "email"
  endpoint  = var.alert_email_address
}
