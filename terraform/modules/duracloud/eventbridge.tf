# EventBridge Rules
resource "aws_cloudwatch_event_rule" "checksum_exporter_schedule" {
  name                = "${local.stack_name}-checksum-exporter-schedule"
  description         = "Trigger monthly DynamoDB checksum table exports"
  schedule_expression = "cron(0 6 * * ? *)"
  state               = "ENABLED"

  tags = {
    Name = "${local.stack_name}-checksum-exporter-schedule"
  }
}

resource "aws_cloudwatch_event_target" "checksum_exporter_target" {
  rule      = aws_cloudwatch_event_rule.checksum_exporter_schedule.name
  target_id = "ChecksumExporterTarget"
  arn       = aws_lambda_function.checksum_exporter_function.arn
}

resource "aws_cloudwatch_event_rule" "report_generator_schedule" {
  name                = "${local.stack_name}-report-generator-schedule"
  description         = "Trigger weekly stats report generation"
  schedule_expression = "cron(0 8 * * ? *)"
  state               = "ENABLED"

  tags = {
    Name = "${local.stack_name}-report-generator-schedule"
  }
}

resource "aws_cloudwatch_event_target" "report_generator_target" {
  rule      = aws_cloudwatch_event_rule.report_generator_schedule.name
  target_id = "ReportGeneratorTarget"
  arn       = aws_lambda_function.report_generator_function.arn
}

# Object Created/Deleted Rules
resource "aws_cloudwatch_event_rule" "object_created_rule" {
  name        = "${local.stack_name}-object-created-rule"
  description = "S3 Object Created Events"

  event_pattern = jsonencode({
    source      = ["aws.s3"]
    detail-type = ["Object Created"]
    detail = {
      bucket = {
        name = [{
          prefix = "${local.stack_name}-"
        }]
      }
    }
  })

  tags = {
    Name = "${local.stack_name}-object-created-rule"
  }
}

resource "aws_cloudwatch_event_target" "object_created_target" {
  rule      = aws_cloudwatch_event_rule.object_created_rule.name
  target_id = "SendToSQSOnCreate"
  arn       = aws_sqs_queue.object_created.arn
  role_arn  = aws_iam_role.events_invoke_sqs_role.arn
}

resource "aws_cloudwatch_event_rule" "object_deleted_rule" {
  name        = "${local.stack_name}-object-deleted-rule"
  description = "S3 Object Deleted Events"

  event_pattern = jsonencode({
    source      = ["aws.s3"]
    detail-type = ["Object Deleted"]
    detail = {
      bucket = {
        name = [{
          prefix = "${local.stack_name}-"
        }]
      }
    }
  })

  tags = {
    Name = "${local.stack_name}-object-deleted-rule"
  }
}

resource "aws_cloudwatch_event_target" "object_deleted_target" {
  rule      = aws_cloudwatch_event_rule.object_deleted_rule.name
  target_id = "SendToSQSOnDelete"
  arn       = aws_sqs_queue.object_deleted.arn
  role_arn  = aws_iam_role.events_invoke_sqs_role.arn
}