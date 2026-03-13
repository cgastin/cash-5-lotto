variable "prefix" {
  description = "Resource name prefix (e.g. cash5-dev)"
  type        = string
}

variable "lambda_invoke_arn" {
  description = "Invoke ARN of the api-handler Lambda function"
  type        = string
}

variable "lambda_arn" {
  description = "ARN of the api-handler Lambda function (for permission resource)"
  type        = string
}

variable "cognito_user_pool_id" {
  description = "Cognito User Pool ID used to build the JWT issuer URL"
  type        = string
}

variable "cognito_audience" {
  description = "Cognito App Client ID used as the JWT audience"
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
