variable "aws_region" {
  description = "AWS region to deploy resources into"
  type        = string
  default     = "us-east-1"
}

variable "prefix" {
  description = "Resource name prefix applied to all AWS resources"
  type        = string
  default     = "cash5-dev"
}

variable "cognito_callback_urls" {
  description = "OAuth callback URLs for the Cognito app client (e.g. mobile deep-link, localhost for dev)"
  type        = list(string)
  default     = ["http://localhost:3000/callback", "myapp://auth/callback"]
}
