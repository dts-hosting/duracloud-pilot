# CloudWatch Alarms
resource "aws_cloudwatch_metric_alarm" "checksum_exporter_function_error_alarm" {
  alarm_name          = "${local.stack_name}-checksum-exporter-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Sum"
  threshold           = "0"
  alarm_description   = "Error generating checksum table export"

  dimensions = {
    FunctionName = aws_lambda_function.checksum_exporter_function.function_name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-checksum-exporter-errors"
  }
}

resource "aws_cloudwatch_metric_alarm" "checksum_verification_function_concurrency_alarm" {
  alarm_name          = "${local.stack_name}-checksum-verification-concurrency"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "ConcurrentExecutions"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Maximum"
  threshold           = "800"
  alarm_description   = "High concurrency processing checksum verifications"

  dimensions = {
    FunctionName = aws_lambda_function.checksum_verification_function.function_name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []
  ok_actions    = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-checksum-verification-concurrency"
  }
}

resource "aws_cloudwatch_metric_alarm" "checksum_verification_function_error_alarm" {
  alarm_name          = "${local.stack_name}-checksum-verification-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Sum"
  threshold           = "0"
  alarm_description   = "Error processing checksum verification"

  dimensions = {
    FunctionName = aws_lambda_function.checksum_verification_function.function_name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-checksum-verification-errors"
  }
}

resource "aws_cloudwatch_metric_alarm" "dynamodb_checksum_table_write_capacity_alarm" {
  alarm_name          = "${local.stack_name}-checksum-table-high-write-capacity"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "3"
  metric_name         = "ConsumedWriteCapacityUnits"
  namespace           = "AWS/DynamoDB"
  period              = "300"
  statistic           = "Sum"
  threshold           = "50000"
  alarm_description   = "DynamoDB Checksum table consuming high write capacity"
  treat_missing_data  = "notBreaching"

  dimensions = {
    TableName = aws_dynamodb_table.checksum_table.name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []
  ok_actions    = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-checksum-table-high-write-capacity"
  }
}

resource "aws_cloudwatch_metric_alarm" "dynamodb_scheduler_table_write_capacity_alarm" {
  alarm_name          = "${local.stack_name}-scheduler-table-high-write-capacity"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "3"
  metric_name         = "ConsumedWriteCapacityUnits"
  namespace           = "AWS/DynamoDB"
  period              = "300"
  statistic           = "Sum"
  threshold           = "50000"
  alarm_description   = "DynamoDB Scheduler table consuming high write capacity"
  treat_missing_data  = "notBreaching"

  dimensions = {
    TableName = aws_dynamodb_table.checksum_scheduler_table.name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []
  ok_actions    = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-scheduler-table-high-write-capacity"
  }
}

resource "aws_cloudwatch_metric_alarm" "report_generator_function_error_alarm" {
  alarm_name          = "${local.stack_name}-report-generator-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Sum"
  threshold           = "0"
  alarm_description   = "Error generating storage stats report"

  dimensions = {
    FunctionName = aws_lambda_function.report_generator_function.function_name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-report-generator-errors"
  }
}

resource "aws_cloudwatch_metric_alarm" "sqs_object_created_alarm" {
  alarm_name          = "${local.stack_name}-object-created-dlq-messages"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = "1"
  metric_name         = "ApproximateNumberOfVisibleMessages"
  namespace           = "AWS/SQS"
  period              = "300"
  statistic           = "Average"
  threshold           = "1"
  alarm_description   = "Messages present in file uploaded DLQ"

  dimensions = {
    QueueName = aws_sqs_queue.object_created_dlq.name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-object-created-dlq-messages"
  }
}

resource "aws_cloudwatch_metric_alarm" "sqs_object_deleted_alarm" {
  alarm_name          = "${local.stack_name}-object-deleted-dlq-messages"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = "1"
  metric_name         = "ApproximateNumberOfVisibleMessages"
  namespace           = "AWS/SQS"
  period              = "300"
  statistic           = "Average"
  threshold           = "1"
  alarm_description   = "Messages present in file deleted DLQ"

  dimensions = {
    QueueName = aws_sqs_queue.object_deleted_dlq.name
  }

  alarm_actions = local.enable_email_alerts ? [aws_sns_topic.email_alert_topic.arn] : []

  tags = {
    Name = "${local.stack_name}-object-deleted-dlq-messages"
  }
}