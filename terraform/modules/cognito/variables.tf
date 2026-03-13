variable "prefix" {
  description = "Resource name prefix (e.g. cash5-dev)"
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}

variable "app_callback_urls" {
  description = "OAuth callback URLs for the Cognito app client"
  type        = list(string)
}
