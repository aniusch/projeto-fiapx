module "network" {
  source = "../../modules/network"

  project      = var.project
  cluster_name = var.cluster_name
  vpc_cidr     = var.vpc_cidr
  az_count     = var.az_count
}

module "eks" {
  source = "../../modules/eks"

  cluster_name       = var.cluster_name
  kubernetes_version = var.kubernetes_version
  cluster_role_arn   = data.aws_iam_role.lab.arn
  node_role_arn      = data.aws_iam_role.lab.arn
  subnet_ids         = module.network.public_subnet_ids

  node_instance_type = var.node_instance_type
  node_desired_size  = var.node_desired_size
  node_min_size      = var.node_min_size
  node_max_size      = var.node_max_size
  node_disk_size     = var.node_disk_size
}

module "rds" {
  source = "../../modules/rds"

  project                    = var.project
  vpc_id                     = module.network.vpc_id
  subnet_ids                 = module.network.public_subnet_ids
  allowed_security_group_ids = [module.eks.cluster_security_group_id]

  engine_version    = var.db_engine_version
  instance_class    = var.db_instance_class
  allocated_storage = var.db_allocated_storage
  db_name           = var.db_name
  username          = var.db_username
  password          = var.db_password
}

module "storage" {
  source = "../../modules/storage"

  project       = var.project
  bucket_prefix = "${var.project}-videos"
  force_destroy = true # short-lived lab: allow teardown even with videos present
}
