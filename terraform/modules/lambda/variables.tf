variable "function_name" {
  description = "Lambda function name"
  type        = string
}

variable "description" {
  description = "Human-readable description of the Lambda function"
  type        = string
  default     = ""
}

variable "handler" {
  description = "Lambda handler entrypoint"
  type        = string
  default     = "bootstrap"
}

variable "runtime" {
  description = "Lambda runtime identifier"
  type        = string
  default     = "provided.al2023"
}

variable "s3_bucket" {
  description = "S3 bucket containing the deployment artifact"
  type        = string
}

variable "s3_key" {
  description = "S3 key of the deployment artifact zip"
  type        = string
}

variable "timeout" {
  description = "Lambda function timeout in seconds"
  type        = number
  default     = 30
}

variable "memory_size" {
  description = "Lambda function memory size in MB"
  type        = number
  default     = 256
}

variable "environment_variables" {
  description = "Environment variables for the Lambda function"
  type        = map(string)
  default     = {}
}

variable "policy_statements" {
  description = "List of IAM policy statement objects to attach as a custom policy"
  type = list(object({
    sid       = optional(string)
    effect    = optional(string)
    actions   = list(string)
    resources = list(string)
    conditions = optional(list(object({
      test     = string
      variable = string
      values   = list(string)
    })), [])
  }))
  default = []
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
