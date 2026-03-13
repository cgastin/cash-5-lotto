# ── raw-snapshots ─────────────────────────────
output "raw_snapshots_bucket_name" {
  description = "Name of the raw-snapshots S3 bucket"
  value       = aws_s3_bucket.raw_snapshots.id
}

output "raw_snapshots_bucket_arn" {
  description = "ARN of the raw-snapshots S3 bucket"
  value       = aws_s3_bucket.raw_snapshots.arn
}

# ── model-artifacts ───────────────────────────
output "model_artifacts_bucket_name" {
  description = "Name of the model-artifacts S3 bucket"
  value       = aws_s3_bucket.model_artifacts.id
}

output "model_artifacts_bucket_arn" {
  description = "ARN of the model-artifacts S3 bucket"
  value       = aws_s3_bucket.model_artifacts.arn
}

# ── exports ───────────────────────────────────
output "exports_bucket_name" {
  description = "Name of the exports S3 bucket"
  value       = aws_s3_bucket.exports.id
}

output "exports_bucket_arn" {
  description = "ARN of the exports S3 bucket"
  value       = aws_s3_bucket.exports.arn
}
