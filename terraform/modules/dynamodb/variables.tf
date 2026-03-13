variable "prefix" {
  description = "Resource name prefix (e.g. cash5-dev)"
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
