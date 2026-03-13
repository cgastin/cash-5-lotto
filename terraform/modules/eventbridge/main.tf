terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# ─────────────────────────────────────────────
# Scheduler Group
# ─────────────────────────────────────────────
resource "aws_scheduler_schedule_group" "main" {
  name = "${var.prefix}-schedules"
  tags = var.tags
}

# ─────────────────────────────────────────────
# Ingestion Job — Mon–Sat 11 PM CT (5 AM UTC)
# cron(minutes hours day-of-month month day-of-week year)
# ─────────────────────────────────────────────
resource "aws_scheduler_schedule" "ingestion_job" {
  name       = "${var.prefix}-ingestion-job"
  group_name = aws_scheduler_schedule_group.main.name

  # 5 AM UTC = 11 PM CST (UTC-6). Runs Mon–Sat in UTC, which corresponds
  # to Mon–Sat night in CST.
  schedule_expression          = "cron(0 5 ? * MON-SAT *)"
  schedule_expression_timezone = "UTC"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 15
  }

  target {
    arn      = var.ingestion_lambda_arn
    role_arn = var.scheduler_role_arn

    retry_policy {
      maximum_retry_attempts       = 2
      maximum_event_age_in_seconds = 3600
    }
  }
}

# ─────────────────────────────────────────────
# Reconciliation Job — Sunday 1 AM CT (7 AM UTC)
# ─────────────────────────────────────────────
resource "aws_scheduler_schedule" "reconciliation_job" {
  name       = "${var.prefix}-reconciliation-job"
  group_name = aws_scheduler_schedule_group.main.name

  # 7 AM UTC = 1 AM CST (UTC-6) Sunday
  schedule_expression          = "cron(0 7 ? * SUN *)"
  schedule_expression_timezone = "UTC"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 15
  }

  target {
    arn      = var.reconciliation_lambda_arn
    role_arn = var.scheduler_role_arn

    retry_policy {
      maximum_retry_attempts       = 2
      maximum_event_age_in_seconds = 3600
    }
  }
}
