# Account-wide resources shared across environments. ECR lives here so the same
# image repositories serve every environment rather than being recreated per-env.
module "ecr" {
  source = "../modules/ecr"

  project      = var.project
  repositories = var.ecr_repositories
}
