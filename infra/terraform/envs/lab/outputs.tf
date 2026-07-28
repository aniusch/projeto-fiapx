output "region" {
  value = var.region
}

output "account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "cluster_name" {
  value = module.eks.cluster_name
}

output "cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "kubeconfig_command" {
  description = "Run this to configure kubectl against the cluster."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name}"
}

output "rds_endpoint" {
  value = module.rds.endpoint
}

output "rds_port" {
  value = module.rds.port
}

output "rds_database" {
  value = module.rds.db_name
}

output "s3_bucket" {
  value = module.storage.bucket
}

output "app_secret_name" {
  description = "AWS Secrets Manager secret consumed by the ESO ExternalSecret."
  value       = aws_secretsmanager_secret.app.name
}
