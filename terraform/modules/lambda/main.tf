terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# ─────────────────────────────────────────────
# CloudWatch Log Group  (created before Lambda
# so Terraform owns the lifecycle)
# ─────────────────────────────────────────────
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = 30
  tags              = var.tags
}

# ─────────────────────────────────────────────
# IAM Role
# ─────────────────────────────────────────────
data "aws_iam_policy_document" "assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.function_name}-role"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "basic_execution" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ─────────────────────────────────────────────
# Custom IAM policy  (only when statements
# are provided by the caller)
# ─────────────────────────────────────────────
data "aws_iam_policy_document" "custom" {
  count = length(var.policy_statements) > 0 ? 1 : 0

  dynamic "statement" {
    for_each = var.policy_statements

    content {
      sid       = lookup(statement.value, "sid", null)
      effect    = lookup(statement.value, "effect", "Allow")
      actions   = statement.value.actions
      resources = statement.value.resources

      dynamic "condition" {
        for_each = lookup(statement.value, "conditions", [])

        content {
          test     = condition.value.test
          variable = condition.value.variable
          values   = condition.value.values
        }
      }
    }
  }
}

resource "aws_iam_policy" "custom" {
  count = length(var.policy_statements) > 0 ? 1 : 0

  name   = "${var.function_name}-policy"
  policy = data.aws_iam_policy_document.custom[0].json
  tags   = var.tags
}

resource "aws_iam_role_policy_attachment" "custom" {
  count = length(var.policy_statements) > 0 ? 1 : 0

  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.custom[0].arn
}

# ─────────────────────────────────────────────
# Lambda Function
# ─────────────────────────────────────────────
resource "aws_lambda_function" "this" {
  function_name = var.function_name
  description   = var.description
  role          = aws_iam_role.lambda.arn

  s3_bucket = var.s3_bucket
  s3_key    = var.s3_key

  handler       = var.handler
  runtime       = var.runtime
  architectures = ["arm64"]

  timeout     = var.timeout
  memory_size = var.memory_size

  dynamic "environment" {
    for_each = length(var.environment_variables) > 0 ? [1] : []

    content {
      variables = var.environment_variables
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.lambda,
    aws_iam_role_policy_attachment.basic_execution,
  ]

  tags = var.tags
}
