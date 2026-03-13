variable "aws_region" {
  description = "AWS region to deploy resources into"
  type        = string
  default     = "us-east-1"
}

variable "prefix" {
  description = "Resource name prefix applied to all AWS resources"
  type        = string
  default     = "cash5-prod"
}

variable "cognito_callback_urls" {
  description = "OAuth callback URLs for the Cognito app client"
  type        = list(string)
  default     = ["myapp://auth/callback"]
}
