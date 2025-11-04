# Pre-created S3 resources for reports and user create bucket requests
resource "aws_s3_bucket" "managed_bucket" {
  bucket = "${local.stack_name}-managed"

  tags = {
    Name = "${local.stack_name}-managed"
  }
}

resource "aws_s3_bucket_notification" "managed_bucket_notification" {
  bucket = aws_s3_bucket.managed_bucket.id

  lambda_function {
    lambda_function_arn = aws_lambda_function.checksum_export_csv_report_function.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "exports/checksum-table/"
    filter_suffix       = "manifest-files.json"
  }

  lambda_function {
    lambda_function_arn = aws_lambda_function.inventory_unwrap_function.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "inventory/"
    filter_suffix       = "manifest.json"
  }

  depends_on = [
    aws_lambda_permission.checksum_export_csv_report_invoke_permission,
    aws_lambda_permission.inventory_unwrap_invoke_permission
  ]
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
