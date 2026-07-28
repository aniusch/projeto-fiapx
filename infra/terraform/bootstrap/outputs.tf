output "state_bucket" {
  value = aws_s3_bucket.state.bucket
}

output "lock_table" {
  value = aws_dynamodb_table.lock.name
}

output "backend_config_hint" {
  description = "Values to put in each root's backend.hcl."
  value = {
    bucket         = aws_s3_bucket.state.bucket
    region         = var.region
    dynamodb_table = aws_dynamodb_table.lock.name
    encrypt        = true
  }
}
