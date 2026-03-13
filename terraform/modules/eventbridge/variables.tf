variable "prefix" {
  description = "Resource name prefix (e.g. cash5-dev)"
  type        = string
}

variable "ingestion_lambda_arn" {
  description = "ARN of the ingestion-job Lambda function"
  type        = string
}

variable "reconciliation_lambda_arn" {
  description = "ARN of the reconciliation-job Lambda function"
  type        = string
}

variable "scheduler_role_arn" {
  description = "IAM role ARN that EventBridge Scheduler assumes to invoke Lambda"
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
