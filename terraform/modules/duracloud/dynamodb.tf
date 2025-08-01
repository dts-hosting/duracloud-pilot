resource "aws_dynamodb_table" "checksum_table" {
  name         = "${local.stack_name}-checksum-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "BucketName"
  range_key    = "ObjectKey"

  attribute {
    name = "BucketName"
    type = "S"
  }

  attribute {
    name = "ObjectKey"
    type = "S"
  }

  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name = "${local.stack_name}-checksum-table"
  }
}

resource "aws_dynamodb_table" "checksum_scheduler_table" {
  name         = "${local.stack_name}-checksum-scheduler-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "BucketName"
  range_key    = "ObjectKey"

  attribute {
    name = "BucketName"
    type = "S"
  }

  attribute {
    name = "ObjectKey"
    type = "S"
  }

  ttl {
    attribute_name = "TTL"
    enabled        = true
  }

  stream_enabled   = true
  stream_view_type = "OLD_IMAGE"

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name = "${local.stack_name}-checksum-scheduler-table"
  }
}