output "region" {
  value = var.region
}

output "cluster_name" {
  value = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.this.endpoint
}

output "kubeconfig_command" {
  description = "Run this to configure kubectl against the cluster."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${aws_eks_cluster.this.name}"
}

output "ecr_repository_urls" {
  description = "Push targets for the service images."
  value       = { for name, repo in aws_ecr_repository.this : name => repo.repository_url }
}

output "s3_bucket" {
  value = aws_s3_bucket.videos.bucket
}

output "rds_endpoint" {
  description = "RDS host (use in POSTGRES_DSN)."
  value       = aws_db_instance.this.address
}

output "rds_port" {
  value = aws_db_instance.this.port
}

output "rds_database" {
  value = aws_db_instance.this.db_name
}

output "account_id" {
  value = data.aws_caller_identity.current.account_id
}
