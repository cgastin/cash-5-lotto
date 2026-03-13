# ── draws ─────────────────────────────────────
output "draws_table_name" {
  description = "Name of the draws DynamoDB table"
  value       = aws_dynamodb_table.draws.name
}

output "draws_table_arn" {
  description = "ARN of the draws DynamoDB table"
  value       = aws_dynamodb_table.draws.arn
}

# ── users ─────────────────────────────────────
output "users_table_name" {
  description = "Name of the users DynamoDB table"
  value       = aws_dynamodb_table.users.name
}

output "users_table_arn" {
  description = "ARN of the users DynamoDB table"
  value       = aws_dynamodb_table.users.arn
}

# ── subscriptions ─────────────────────────────
output "subscriptions_table_name" {
  description = "Name of the subscriptions DynamoDB table"
  value       = aws_dynamodb_table.subscriptions.name
}

output "subscriptions_table_arn" {
  description = "ARN of the subscriptions DynamoDB table"
  value       = aws_dynamodb_table.subscriptions.arn
}

# ── predictions ───────────────────────────────
output "predictions_table_name" {
  description = "Name of the predictions DynamoDB table"
  value       = aws_dynamodb_table.predictions.name
}

output "predictions_table_arn" {
  description = "ARN of the predictions DynamoDB table"
  value       = aws_dynamodb_table.predictions.arn
}

# ── pretest-results ───────────────────────────
output "pretest_results_table_name" {
  description = "Name of the pretest-results DynamoDB table"
  value       = aws_dynamodb_table.pretest_results.name
}

output "pretest_results_table_arn" {
  description = "ARN of the pretest-results DynamoDB table"
  value       = aws_dynamodb_table.pretest_results.arn
}

# ── sync-state ────────────────────────────────
output "sync_state_table_name" {
  description = "Name of the sync-state DynamoDB table"
  value       = aws_dynamodb_table.sync_state.name
}

output "sync_state_table_arn" {
  description = "ARN of the sync-state DynamoDB table"
  value       = aws_dynamodb_table.sync_state.arn
}

# ── stats-cache ───────────────────────────────
output "stats_cache_table_name" {
  description = "Name of the stats-cache DynamoDB table"
  value       = aws_dynamodb_table.stats_cache.name
}

output "stats_cache_table_arn" {
  description = "ARN of the stats-cache DynamoDB table"
  value       = aws_dynamodb_table.stats_cache.arn
}

# ── audit-log ─────────────────────────────────
output "audit_log_table_name" {
  description = "Name of the audit-log DynamoDB table"
  value       = aws_dynamodb_table.audit_log.name
}

output "audit_log_table_arn" {
  description = "ARN of the audit-log DynamoDB table"
  value       = aws_dynamodb_table.audit_log.arn
}
