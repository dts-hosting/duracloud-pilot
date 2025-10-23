# S3 Replication Role
resource "aws_iam_role" "s3_replication_role" {
  name = "${local.stack_name}-s3-replication-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "s3.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-s3-replication-role"
  }
}

resource "aws_iam_role_policy" "s3_replication_policy" {
  name = "${local.stack_name}-s3-replication-policy"
  role = aws_iam_role.s3_replication_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetReplicationConfiguration",
          "s3:ListBucket"
        ]
        Resource = "arn:aws:s3:::${local.stack_name}*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObjectVersion",
          "s3:GetObjectVersionAcl",
          "s3:GetObjectVersionTagging"
        ]
        Resource = "arn:aws:s3:::${local.stack_name}*/*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObjectVersionTagging",
          "s3:ReplicateObject",
          "s3:ReplicateDelete",
          "s3:ReplicateTags"
        ]
        Resource = "arn:aws:s3:::${local.stack_name}*-repl/*"
      }
    ]
  })
}

# EventBridge Lambda Role
resource "aws_iam_role" "events_invoke_lambda_role" {
  name = "${local.stack_name}-events-invoke-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "events.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-events-invoke-lambda-role"
  }
}

resource "aws_iam_role_policy" "events_invoke_lambda_policy" {
  name = "${local.stack_name}-invoke-lambda-policy"
  role = aws_iam_role.events_invoke_lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "lambda:InvokeFunction"
        Resource = aws_lambda_function.bucket_requested_function.arn
      }
    ]
  })
}

# EventBridge SQS Role
resource "aws_iam_role" "events_invoke_sqs_role" {
  name = "${local.stack_name}-invoke-sqs-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "events.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-invoke-sqs-role"
  }
}

resource "aws_iam_role_policy" "events_invoke_sqs_policy" {
  name = "${local.stack_name}-invoke-sqs-policy"
  role = aws_iam_role.events_invoke_sqs_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = "sqs:SendMessage"
        Resource = [
          aws_sqs_queue.object_created.arn,
          aws_sqs_queue.object_deleted.arn
        ]
      }
    ]
  })
}

# Bucket Requested Function IAM
resource "aws_iam_role" "bucket_requested_function_role" {
  name = "${local.stack_name}-bucket-requested-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-bucket-requested-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "bucket_requested_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.bucket_requested_function_role.name
}

resource "aws_iam_role_policy" "bucket_requested_function_policy" {
  name = "${local.stack_name}-bucket-requested-function-policy"
  role = aws_iam_role.bucket_requested_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:CreateBucket",
          "s3:DeleteBucket",
          "s3:GetObject",
          "s3:PutObject",
          "s3:PutBucketLogging",
          "s3:DeleteBucketPolicy",
          "s3:PutBucketPolicy",
          "s3:PutBucketTagging",
          "s3:PutBucketVersioning",
          "s3:PutLifecycleConfiguration",
          "s3:PutBucketNotification",
          "s3:PutBucketNotificationConfiguration",
          "s3:PutBucketInventoryConfiguration",
          "s3:PutInventoryConfiguration",
          "s3:PutBucketAcl",
          "s3:PutBucketOwnershipControls",
          "s3:PutBucketPublicAccessBlock",
          "s3:PutBucketReplication",
          "s3:PutReplicationConfiguration",
          "s3:PutBucketLifecycleConfiguration"
        ]
        Resource = "arn:aws:s3:::*"
      },
      {
        Effect   = "Allow"
        Action   = "iam:PassRole"
        Resource = aws_iam_role.s3_replication_role.arn
        Condition = {
          StringEquals = {
            "iam:PassedToService" = "s3.amazonaws.com"
          }
        }
      }
    ]
  })
}

# Checksum Exporter Function IAM
resource "aws_iam_role" "checksum_exporter_function_role" {
  name = "${local.stack_name}-checksum-exporter-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-checksum-exporter-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "checksum_exporter_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.checksum_exporter_function_role.name
}

resource "aws_iam_role_policy" "checksum_exporter_function_policy" {
  name = "${local.stack_name}-checksum-exporter-function-policy"
  role = aws_iam_role.checksum_exporter_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DescribeTable",
          "dynamodb:ExportTableToPointInTime",
          "dynamodb:DescribeExport"
        ]
        Resource = [
          aws_dynamodb_table.checksum_table.arn,
          "${aws_dynamodb_table.checksum_table.arn}/export/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:PutObjectAcl",
          "s3:AbortMultipartUpload",
          "s3:ListBucketMultipartUploads",
          "s3:ListMultipartUploadParts"
        ]
        Resource = [
          "${aws_s3_bucket.managed_bucket.arn}/exports/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket"
        ]
        Resource = aws_s3_bucket.managed_bucket.arn
      }
    ]
  })
}

# Checksum Export CSV Report Function IAM
resource "aws_iam_role" "checksum_export_csv_report_function_role" {
  name = "${local.stack_name}-checksum-export-csv-report-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-checksum-export-csv-report-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "checksum_export_csv_report_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.checksum_export_csv_report_function_role.name
}

resource "aws_iam_role_policy" "checksum_export_csv_report_function_policy" {
  name = "${local.stack_name}-checksum-export-csv-report-function-policy"
  role = aws_iam_role.checksum_export_csv_report_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket",
          "s3:ListBucketMultipartUploads",
          "s3:ListMultipartUploadParts",
          "s3:AbortMultipartUpload",
          "s3:GetObject",
          "s3:PutObject",
          "s3:PutObjectAcl"
        ]
        Resource = [
          "${aws_s3_bucket.managed_bucket.arn}/exports/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket"
        ]
        Resource = aws_s3_bucket.managed_bucket.arn
      }
    ]
  })
}

# Checksum Failure Function IAM
resource "aws_iam_role" "checksum_failure_function_role" {
  name = "${local.stack_name}-checksum-failure-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-checksum-failure-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "checksum_failure_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.checksum_failure_function_role.name
}

resource "aws_iam_role_policy" "checksum_failure_function_policy" {
  name = "${local.stack_name}-checksum-failure-function-policy"
  role = aws_iam_role.checksum_failure_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject"
        ]
        Resource = "arn:aws:s3:::${local.stack_name}-managed/*"
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DescribeStream",
          "dynamodb:GetRecords",
          "dynamodb:GetShardIterator",
          "dynamodb:ListStreams"
        ]
        Resource = [
          aws_dynamodb_table.checksum_table.stream_arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = local.enable_email_alerts ? aws_sns_topic.email_alert_topic.arn : "*"
      }
    ]
  })
}

# Checksum Verification Function IAM
resource "aws_iam_role" "checksum_verification_function_role" {
  name = "${local.stack_name}-checksum-verification-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-checksum-verification-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "checksum_verification_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.checksum_verification_function_role.name
}

resource "aws_iam_role_policy" "checksum_verification_function_policy" {
  name = "${local.stack_name}-checksum-verification-function-policy"
  role = aws_iam_role.checksum_verification_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DeleteItem",
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem"
        ]
        Resource = [
          aws_dynamodb_table.checksum_table.arn,
          aws_dynamodb_table.checksum_scheduler_table.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DescribeStream",
          "dynamodb:GetRecords",
          "dynamodb:GetShardIterator",
          "dynamodb:ListStreams"
        ]
        Resource = [
          aws_dynamodb_table.checksum_scheduler_table.stream_arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:GetObjectVersion"
        ]
        Resource = "arn:aws:s3:::${local.stack_name}-*/*"
      },
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = local.enable_email_alerts ? aws_sns_topic.email_alert_topic.arn : "*"
      }
    ]
  })
}

# File Deleted Function IAM
resource "aws_iam_role" "file_deleted_function_role" {
  name = "${local.stack_name}-file-deleted-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-file-deleted-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "file_deleted_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.file_deleted_function_role.name
}

resource "aws_iam_role_policy_attachment" "file_deleted_function_sqs" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaSQSQueueExecutionRole"
  role       = aws_iam_role.file_deleted_function_role.name
}

resource "aws_iam_role_policy" "file_deleted_function_policy" {
  name = "${local.stack_name}-file-deleted-function-policy"
  role = aws_iam_role.file_deleted_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DeleteItem"
        ]
        Resource = [
          aws_dynamodb_table.checksum_table.arn,
          aws_dynamodb_table.checksum_scheduler_table.arn
        ]
      }
    ]
  })
}

# File Uploaded Function IAM
resource "aws_iam_role" "file_uploaded_function_role" {
  name = "${local.stack_name}-file-uploaded-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-file-uploaded-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "file_uploaded_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.file_uploaded_function_role.name
}

resource "aws_iam_role_policy_attachment" "file_uploaded_function_sqs" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaSQSQueueExecutionRole"
  role       = aws_iam_role.file_uploaded_function_role.name
}

resource "aws_iam_role_policy" "file_uploaded_function_policy" {
  name = "${local.stack_name}-file-uploaded-function-policy"
  role = aws_iam_role.file_uploaded_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem"
        ]
        Resource = [
          aws_dynamodb_table.checksum_table.arn,
          aws_dynamodb_table.checksum_scheduler_table.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:GetObjectVersion"
        ]
        Resource = "arn:aws:s3:::${local.stack_name}-*/*"
      }
    ]
  })
}

# Report Generator Function IAM
resource "aws_iam_role" "report_generator_function_role" {
  name = "${local.stack_name}-report-generator-function-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-report-generator-function-role"
  }
}

resource "aws_iam_role_policy_attachment" "report_generator_function_basic" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.report_generator_function_role.name
}

resource "aws_iam_role_policy" "report_generator_function_policy" {
  name = "${local.stack_name}-report-generator-function-policy"
  role = aws_iam_role.report_generator_function_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "cloudwatch:GetMetricStatistics",
          "cloudwatch:GetMetricData",
          "cloudwatch:ListMetrics",
          "s3:GetBucketTagging",
          "s3:GetObject",
          "s3:ListAllMyBuckets",
          "s3:ListBucket"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:PutObjectAcl",
          "s3:AbortMultipartUpload",
          "s3:ListBucketMultipartUploads",
          "s3:ListMultipartUploadParts"
        ]
        Resource = "${aws_s3_bucket.managed_bucket.arn}/reports/*"
      },
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = local.enable_email_alerts ? aws_sns_topic.email_alert_topic.arn : "*"
      }
    ]
  })
}

# IAM Groups and Policies
resource "aws_iam_group" "s3_power_users_group" {
  name = "${local.stack_name}-S3PowerUsers"
  path = "/"
}

resource "aws_iam_policy" "s3_power_users_policy" {
  name        = "${local.stack_name}-S3PowerUsersPolicy"
  description = "Policy for S3 power users"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:ListAllMyBuckets"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket",
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:AbortMultipartUpload",
          "s3:ListMultipartUploadParts",
          "s3:ListBucketMultipartUploads"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*",
          "arn:aws:s3:::${local.stack_name}*/*"
        ]
      },
      {
        Effect = "Deny"
        Action = [
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-managed",
          "arn:aws:s3:::${local.stack_name}*-managed/*"
        ]
      },
      {
        Effect = "Deny"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-repl",
          "arn:aws:s3:::${local.stack_name}*-repl/*"
        ]
      },
      {
        Effect = "Deny"
        Action = [
          "s3:*"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-logs",
          "arn:aws:s3:::${local.stack_name}*-logs/*"
        ]
      }
    ]
  })
}

resource "aws_iam_group_policy_attachment" "s3_power_users_policy_attachment" {
  group      = aws_iam_group.s3_power_users_group.name
  policy_arn = aws_iam_policy.s3_power_users_policy.arn
}

resource "aws_iam_group" "s3_users_group" {
  name = "${local.stack_name}-S3Users"
  path = "/"
}

resource "aws_iam_policy" "s3_users_policy" {
  name        = "${local.stack_name}-S3UsersPolicy"
  description = "Policy for S3 regular users"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:ListAllMyBuckets"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket",
          "s3:PutObject",
          "s3:AbortMultipartUpload",
          "s3:ListMultipartUploadParts",
          "s3:ListBucketMultipartUploads"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*",
          "arn:aws:s3:::${local.stack_name}*/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-managed",
          "arn:aws:s3:::${local.stack_name}*-managed/*"
        ]
      },
      {
        Effect = "Deny"
        Action = [
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-managed",
          "arn:aws:s3:::${local.stack_name}*-managed/*"
        ]
      },
      {
        Effect = "Deny"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-repl",
          "arn:aws:s3:::${local.stack_name}*-repl/*"
        ]
      },
      {
        Effect = "Deny"
        Action = [
          "s3:*"
        ]
        Resource = [
          "arn:aws:s3:::${local.stack_name}*-logs",
          "arn:aws:s3:::${local.stack_name}*-logs/*"
        ]
      }
    ]
  })
}

resource "aws_iam_group_policy_attachment" "s3_users_policy_attachment" {
  group      = aws_iam_group.s3_users_group.name
  policy_arn = aws_iam_policy.s3_users_policy.arn
}

# Test User
resource "aws_iam_user" "test_user" {
  name = "${local.stack_name}-TestUser"
  path = "/"

  tags = {
    Name = "${local.stack_name}-TestUser"
  }
}

resource "aws_iam_user_group_membership" "test_user_membership" {
  user = aws_iam_user.test_user.name
  groups = [
    aws_iam_group.s3_power_users_group.name
  ]
}

resource "aws_iam_access_key" "test_user_access_key" {
  user = aws_iam_user.test_user.name
}

# SSM Parameters for test user access keys
resource "aws_ssm_parameter" "test_user_access_key_parameter" {
  name        = "/${local.stack_name}/iam/test/access-key-id"
  description = "Access Key ID for the S3 test user"
  type        = "String"
  value       = aws_iam_access_key.test_user_access_key.id
  tier        = "Standard"

  tags = {
    Name = "${local.stack_name}-test-user-access-key-id"
  }
}

resource "aws_ssm_parameter" "test_user_secret_access_key_parameter" {
  name        = "/${local.stack_name}/iam/test/secret-access-key"
  description = "Secret Access Key for the S3 test user"
  type        = "SecureString"
  value       = aws_iam_access_key.test_user_access_key.secret
  tier        = "Standard"

  tags = {
    Name = "${local.stack_name}-test-user-secret-access-key"
  }
}
