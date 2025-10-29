# Redrive IAM role for both Step Functions and EventBridge Scheduler
resource "aws_iam_role" "dlq_redrive" {
  name = "${local.stack_name}-dlq-redrive"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "states.amazonaws.com"
        }
      },
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "scheduler.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "${local.stack_name}-dlq-redrive"
  }
}

# Policy for SQS redrive operations and Step Functions execution
resource "aws_iam_role_policy" "dlq_redrive" {
  role = aws_iam_role.dlq_redrive.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
          "sqs:SendMessage"
        ]
        Resource = [
          aws_sqs_queue.object_created_dlq.arn,
          aws_sqs_queue.object_deleted_dlq.arn,
          aws_sqs_queue.object_created.arn,
          aws_sqs_queue.object_deleted.arn
        ]
      },
      {
        Effect   = "Allow"
        Action   = "sqs:StartMessageMoveTask"
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = "states:StartExecution"
        Resource = aws_sfn_state_machine.dlq_redrive.arn
      }
    ]
  })
}

# Step Functions state machine to redrive messages from DLQs
resource "aws_sfn_state_machine" "dlq_redrive" {
  name     = "${local.stack_name}-dlq-redrive"
  role_arn = aws_iam_role.dlq_redrive.arn

  definition = jsonencode({
    Comment = "Redrive messages from DLQ to main queue"
    StartAt = "RedriveObjectCreatedDLQ"
    States = {
      RedriveObjectCreatedDLQ = {
        Type     = "Task"
        Resource = "arn:aws:states:::aws-sdk:sqs:startMessageMoveTask"
        Parameters = {
          SourceArn = aws_sqs_queue.object_created_dlq.arn
        }
        Next = "RedriveObjectDeletedDLQ"
        Catch = [{
          ErrorEquals = ["States.ALL"]
          Next        = "RedriveObjectDeletedDLQ"
        }]
      }
      RedriveObjectDeletedDLQ = {
        Type     = "Task"
        Resource = "arn:aws:states:::aws-sdk:sqs:startMessageMoveTask"
        Parameters = {
          SourceArn = aws_sqs_queue.object_deleted_dlq.arn
        }
        End = true
        Catch = [{
          ErrorEquals = ["States.ALL"]
          Next        = "Done"
        }]
      }
      Done = {
        Type = "Succeed"
      }
    }
  })

  tags = {
    Name = "${local.stack_name}-dlq-redrive"
  }
}

resource "aws_scheduler_schedule" "dlq_redrive" {
  name = "${local.stack_name}-dlq-redrive"

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression = "rate(6 hours)"

  target {
    arn      = aws_sfn_state_machine.dlq_redrive.arn
    role_arn = aws_iam_role.dlq_redrive.arn
  }
}
