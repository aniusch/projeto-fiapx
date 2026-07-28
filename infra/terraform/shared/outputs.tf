output "ecr_repository_urls" {
  description = "Push targets for the service images."
  value       = module.ecr.repository_urls
}
