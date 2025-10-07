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
