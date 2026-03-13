output "ingestion_schedule_arn" {
  description = "ARN of the ingestion-job EventBridge schedule"
  value       = aws_scheduler_schedule.ingestion_job.arn
}

output "reconciliation_schedule_arn" {
  description = "ARN of the reconciliation-job EventBridge schedule"
  value       = aws_scheduler_schedule.reconciliation_job.arn
}

output "schedule_group_arn" {
  description = "ARN of the EventBridge Scheduler schedule group"
  value       = aws_scheduler_schedule_group.main.arn
}
