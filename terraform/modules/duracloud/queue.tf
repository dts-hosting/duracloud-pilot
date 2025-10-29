# SQS Queues for S3 event processing
resource "aws_sqs_queue" "object_created_dlq" {
  name                      = "${local.stack_name}-object-created-dlq"
  message_retention_seconds = 604800 # 7 days

  redrive_allow_policy = jsonencode({
    redrivePermission = "allowAll"
  })

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
  message_retention_seconds = 604800 # 7 days

  redrive_allow_policy = jsonencode({
    redrivePermission = "allowAll"
  })

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
