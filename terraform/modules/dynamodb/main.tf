terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# ─────────────────────────────────────────────
# draws
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "draws" {
  name         = "${var.prefix}-draws"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "draw_date"

  attribute {
    name = "draw_date"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# users
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "users" {
  name         = "${var.prefix}-users"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_id"

  attribute {
    name = "user_id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# subscriptions
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "subscriptions" {
  name         = "${var.prefix}-subscriptions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_id"

  attribute {
    name = "user_id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# predictions
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "predictions" {
  name         = "${var.prefix}-predictions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "prediction_date"
  range_key    = "rank"

  attribute {
    name = "prediction_date"
    type = "S"
  }

  attribute {
    name = "rank"
    type = "N"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# pretest-results
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "pretest_results" {
  name         = "${var.prefix}-pretest-results"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "draw_date"
  range_key    = "test_number"

  attribute {
    name = "draw_date"
    type = "S"
  }

  attribute {
    name = "test_number"
    type = "N"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# sync-state
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "sync_state" {
  name         = "${var.prefix}-sync-state"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "state_key"

  attribute {
    name = "state_key"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# stats-cache
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "stats_cache" {
  name         = "${var.prefix}-stats-cache"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "stat_type"

  attribute {
    name = "stat_type"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# audit-log  (GSI: actor_id + timestamp)
# ─────────────────────────────────────────────
resource "aws_dynamodb_table" "audit_log" {
  name         = "${var.prefix}-audit-log"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "event_id"
  range_key    = "timestamp"

  attribute {
    name = "event_id"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "S"
  }

  attribute {
    name = "actor_id"
    type = "S"
  }

  global_secondary_index {
    name            = "actor_id-timestamp-index"
    hash_key        = "actor_id"
    range_key       = "timestamp"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = var.tags
}
