output "api_endpoint" {
  description = "HTTP API invoke URL ($default stage)"
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "api_id" {
  description = "API Gateway HTTP API ID"
  value       = aws_apigatewayv2_api.main.id
}

output "execution_arn" {
  description = "API Gateway execution ARN (used for WAF association)"
  value       = aws_apigatewayv2_api.main.execution_arn
}
