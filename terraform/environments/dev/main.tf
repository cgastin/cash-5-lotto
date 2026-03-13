terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Remote state: use local for now.
  # Uncomment and configure the S3 backend before sharing state across machines.
  # backend "s3" {
  #   bucket         = "cash5-terraform-state"
  #   key            = "dev/terraform.tfstate"
  #   region         = "us-east-1"
  #   dynamodb_table = "cash5-terraform-locks"
  #   encrypt        = true
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.tags
  }
}

locals {
  tags = {
    Environment = "dev"
    Project     = "cash5"
    ManagedBy   = "terraform"
  }
}

# ─────────────────────────────────────────────
# DynamoDB Tables
# ─────────────────────────────────────────────
module "dynamodb" {
  source = "../../modules/dynamodb"

  prefix = var.prefix
  tags   = local.tags
}

# ─────────────────────────────────────────────
# S3 Buckets
# ─────────────────────────────────────────────
module "s3" {
  source = "../../modules/s3"

  prefix = var.prefix
  tags   = local.tags
}

# ─────────────────────────────────────────────
# Cognito User Pool
# ─────────────────────────────────────────────
module "cognito" {
  source = "../../modules/cognito"

  prefix            = var.prefix
  tags              = local.tags
  app_callback_urls = var.cognito_callback_urls
}

# ─────────────────────────────────────────────
# Lambda: api-handler
# ─────────────────────────────────────────────
module "lambda_api_handler" {
  source = "../../modules/lambda"

  function_name = "${var.prefix}-api-handler"
  description   = "HTTP API handler — routes all API Gateway requests"
  s3_bucket     = module.s3.model_artifacts_bucket_name
  s3_key        = "lambda/api-handler/latest.zip"
  timeout       = 30
  memory_size   = 256
  tags          = local.tags

  environment_variables = {
    ENVIRONMENT               = "dev"
    DRAWS_TABLE               = module.dynamodb.draws_table_name
    USERS_TABLE               = module.dynamodb.users_table_name
    SUBSCRIPTIONS_TABLE       = module.dynamodb.subscriptions_table_name
    PREDICTIONS_TABLE         = module.dynamodb.predictions_table_name
    PRETEST_RESULTS_TABLE     = module.dynamodb.pretest_results_table_name
    SYNC_STATE_TABLE          = module.dynamodb.sync_state_table_name
    STATS_CACHE_TABLE         = module.dynamodb.stats_cache_table_name
    AUDIT_LOG_TABLE           = module.dynamodb.audit_log_table_name
    EXPORTS_BUCKET            = module.s3.exports_bucket_name
    RAW_SNAPSHOTS_BUCKET      = module.s3.raw_snapshots_bucket_name
    COGNITO_USER_POOL_ID      = module.cognito.user_pool_id
    SCORING_WEIGHTS_SSM_PATH  = "/${var.prefix}/scoring-weights"
  }

  policy_statements = [
    {
      sid    = "DynamoDBAccess"
      effect = "Allow"
      actions = [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:BatchGetItem",
        "dynamodb:BatchWriteItem",
      ]
      resources = [
        module.dynamodb.draws_table_arn,
        module.dynamodb.users_table_arn,
        module.dynamodb.subscriptions_table_arn,
        module.dynamodb.predictions_table_arn,
        module.dynamodb.pretest_results_table_arn,
        module.dynamodb.sync_state_table_arn,
        module.dynamodb.stats_cache_table_arn,
        module.dynamodb.audit_log_table_arn,
        "${module.dynamodb.draws_table_arn}/index/*",
        "${module.dynamodb.users_table_arn}/index/*",
        "${module.dynamodb.subscriptions_table_arn}/index/*",
        "${module.dynamodb.predictions_table_arn}/index/*",
        "${module.dynamodb.pretest_results_table_arn}/index/*",
        "${module.dynamodb.sync_state_table_arn}/index/*",
        "${module.dynamodb.stats_cache_table_arn}/index/*",
        "${module.dynamodb.audit_log_table_arn}/index/*",
      ]
    },
    {
      sid    = "S3ExportsAccess"
      effect = "Allow"
      actions = [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
      ]
      resources = [
        "${module.s3.exports_bucket_arn}/*",
      ]
    },
    {
      sid    = "SSMScoringWeights"
      effect = "Allow"
      actions = [
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:GetParametersByPath",
      ]
      resources = [
        "arn:aws:ssm:${var.aws_region}:*:parameter/${var.prefix}/scoring-weights*",
      ]
    },
    {
      sid    = "SecretsManagerRead"
      effect = "Allow"
      actions = [
        "secretsmanager:GetSecretValue",
      ]
      resources = [
        "arn:aws:secretsmanager:${var.aws_region}:*:secret:${var.prefix}/*",
      ]
    },
  ]
}

# ─────────────────────────────────────────────
# Lambda: ingestion-job
# ─────────────────────────────────────────────
module "lambda_ingestion_job" {
  source = "../../modules/lambda"

  function_name = "${var.prefix}-ingestion-job"
  description   = "Nightly lottery draw ingestion job (Mon–Sat 11 PM CT)"
  s3_bucket     = module.s3.model_artifacts_bucket_name
  s3_key        = "lambda/ingestion-job/latest.zip"
  timeout       = 300
  memory_size   = 512
  tags          = local.tags

  environment_variables = {
    ENVIRONMENT          = "dev"
    DRAWS_TABLE          = module.dynamodb.draws_table_name
    SYNC_STATE_TABLE     = module.dynamodb.sync_state_table_name
    STATS_CACHE_TABLE    = module.dynamodb.stats_cache_table_name
    RAW_SNAPSHOTS_BUCKET = module.s3.raw_snapshots_bucket_name
    AUDIT_LOG_TABLE      = module.dynamodb.audit_log_table_name
  }

  policy_statements = [
    {
      sid    = "DynamoDBIngestion"
      effect = "Allow"
      actions = [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:BatchWriteItem",
      ]
      resources = [
        module.dynamodb.draws_table_arn,
        module.dynamodb.sync_state_table_arn,
        module.dynamodb.stats_cache_table_arn,
        module.dynamodb.audit_log_table_arn,
        "${module.dynamodb.draws_table_arn}/index/*",
        "${module.dynamodb.sync_state_table_arn}/index/*",
        "${module.dynamodb.stats_cache_table_arn}/index/*",
        "${module.dynamodb.audit_log_table_arn}/index/*",
      ]
    },
    {
      sid    = "S3RawSnapshotsWrite"
      effect = "Allow"
      actions = [
        "s3:PutObject",
        "s3:GetObject",
      ]
      resources = [
        "${module.s3.raw_snapshots_bucket_arn}/*",
      ]
    },
    {
      sid    = "SecretsManagerRead"
      effect = "Allow"
      actions = [
        "secretsmanager:GetSecretValue",
      ]
      resources = [
        "arn:aws:secretsmanager:${var.aws_region}:*:secret:${var.prefix}/*",
      ]
    },
  ]
}

# ─────────────────────────────────────────────
# Lambda: reconciliation-job
# ─────────────────────────────────────────────
module "lambda_reconciliation_job" {
  source = "../../modules/lambda"

  function_name = "${var.prefix}-reconciliation-job"
  description   = "Weekly reconciliation job (Sunday 1 AM CT)"
  s3_bucket     = module.s3.model_artifacts_bucket_name
  s3_key        = "lambda/reconciliation-job/latest.zip"
  timeout       = 600
  memory_size   = 512
  tags          = local.tags

  environment_variables = {
    ENVIRONMENT             = "dev"
    DRAWS_TABLE             = module.dynamodb.draws_table_name
    PREDICTIONS_TABLE       = module.dynamodb.predictions_table_name
    PRETEST_RESULTS_TABLE   = module.dynamodb.pretest_results_table_name
    SYNC_STATE_TABLE        = module.dynamodb.sync_state_table_name
    STATS_CACHE_TABLE       = module.dynamodb.stats_cache_table_name
    AUDIT_LOG_TABLE         = module.dynamodb.audit_log_table_name
    MODEL_ARTIFACTS_BUCKET  = module.s3.model_artifacts_bucket_name
    SCORING_WEIGHTS_SSM_PATH = "/${var.prefix}/scoring-weights"
  }

  policy_statements = [
    {
      sid    = "DynamoDBReconciliation"
      effect = "Allow"
      actions = [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:BatchWriteItem",
      ]
      resources = [
        module.dynamodb.draws_table_arn,
        module.dynamodb.predictions_table_arn,
        module.dynamodb.pretest_results_table_arn,
        module.dynamodb.sync_state_table_arn,
        module.dynamodb.stats_cache_table_arn,
        module.dynamodb.audit_log_table_arn,
        "${module.dynamodb.draws_table_arn}/index/*",
        "${module.dynamodb.predictions_table_arn}/index/*",
        "${module.dynamodb.pretest_results_table_arn}/index/*",
        "${module.dynamodb.sync_state_table_arn}/index/*",
        "${module.dynamodb.stats_cache_table_arn}/index/*",
        "${module.dynamodb.audit_log_table_arn}/index/*",
      ]
    },
    {
      sid    = "S3ModelArtifacts"
      effect = "Allow"
      actions = [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket",
      ]
      resources = [
        module.s3.model_artifacts_bucket_arn,
        "${module.s3.model_artifacts_bucket_arn}/*",
      ]
    },
    {
      sid    = "SSMScoringWeights"
      effect = "Allow"
      actions = [
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:GetParametersByPath",
        "ssm:PutParameter",
      ]
      resources = [
        "arn:aws:ssm:${var.aws_region}:*:parameter/${var.prefix}/scoring-weights*",
      ]
    },
    {
      sid    = "SecretsManagerRead"
      effect = "Allow"
      actions = [
        "secretsmanager:GetSecretValue",
      ]
      resources = [
        "arn:aws:secretsmanager:${var.aws_region}:*:secret:${var.prefix}/*",
      ]
    },
  ]
}

# ─────────────────────────────────────────────
# API Gateway
# ─────────────────────────────────────────────
module "api_gateway" {
  source = "../../modules/api-gateway"

  prefix               = var.prefix
  lambda_invoke_arn    = module.lambda_api_handler.invoke_arn
  lambda_arn           = module.lambda_api_handler.function_arn
  cognito_user_pool_id = module.cognito.user_pool_id
  cognito_audience     = module.cognito.app_client_id
  tags                 = local.tags
}

# ─────────────────────────────────────────────
# IAM Role for EventBridge Scheduler → Lambda
# ─────────────────────────────────────────────
data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name               = "${var.prefix}-scheduler-role"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume.json
  tags               = local.tags
}

data "aws_iam_policy_document" "scheduler_invoke" {
  statement {
    effect  = "Allow"
    actions = ["lambda:InvokeFunction"]
    resources = [
      module.lambda_ingestion_job.function_arn,
      module.lambda_reconciliation_job.function_arn,
    ]
  }
}

resource "aws_iam_policy" "scheduler_invoke" {
  name   = "${var.prefix}-scheduler-invoke-policy"
  policy = data.aws_iam_policy_document.scheduler_invoke.json
  tags   = local.tags
}

resource "aws_iam_role_policy_attachment" "scheduler_invoke" {
  role       = aws_iam_role.scheduler.name
  policy_arn = aws_iam_policy.scheduler_invoke.arn
}

# ─────────────────────────────────────────────
# EventBridge Scheduler
# ─────────────────────────────────────────────
module "eventbridge" {
  source = "../../modules/eventbridge"

  prefix                    = var.prefix
  ingestion_lambda_arn      = module.lambda_ingestion_job.function_arn
  reconciliation_lambda_arn = module.lambda_reconciliation_job.function_arn
  scheduler_role_arn        = aws_iam_role.scheduler.arn
  tags                      = local.tags
}

# ─────────────────────────────────────────────
# Secrets Manager placeholders
# ─────────────────────────────────────────────
resource "aws_secretsmanager_secret" "lottery_api_key" {
  name                    = "${var.prefix}/lottery-api-key"
  description             = "API key for the Texas Lottery data source"
  recovery_window_in_days = 7
  tags                    = local.tags
}

resource "aws_secretsmanager_secret_version" "lottery_api_key" {
  secret_id     = aws_secretsmanager_secret.lottery_api_key.id
  secret_string = jsonencode({ api_key = "REPLACE_ME" })

  lifecycle {
    ignore_changes = [secret_string]
  }
}

resource "aws_secretsmanager_secret" "app_secrets" {
  name                    = "${var.prefix}/app-secrets"
  description             = "General application secrets (JWT signing key, etc.)"
  recovery_window_in_days = 7
  tags                    = local.tags
}

resource "aws_secretsmanager_secret_version" "app_secrets" {
  secret_id     = aws_secretsmanager_secret.app_secrets.id
  secret_string = jsonencode({ placeholder = "REPLACE_ME" })

  lifecycle {
    ignore_changes = [secret_string]
  }
}

# ─────────────────────────────────────────────
# SSM Parameter Store: scoring weights
# ─────────────────────────────────────────────
resource "aws_ssm_parameter" "scoring_weights" {
  name        = "/${var.prefix}/scoring-weights"
  description = "JSON scoring weights used by the prediction model"
  type        = "String"
  tier        = "Standard"

  value = jsonencode({
    frequency_weight   = 0.30
    recency_weight     = 0.25
    gap_weight         = 0.20
    pattern_weight     = 0.15
    correlation_weight = 0.10
  })

  tags = local.tags

  lifecycle {
    ignore_changes = [value]
  }
}
