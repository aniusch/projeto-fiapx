# Committed, non-secret values for the AWS Academy lab environment.
# Apply with:  terraform apply -var-file=lab.tfvars  (db_password via TF_VAR_db_password)
# NEVER put db_password or any credential here.

region             = "us-east-1"
project            = "fiapx"
cluster_name       = "fiapx"
kubernetes_version = "1.30"
lab_role_name      = "LabRole"

az_count = 2

node_instance_type = "t3.medium"
node_desired_size  = 2
node_min_size      = 2
node_max_size      = 4
node_disk_size     = 20

db_engine_version    = "16.9"
db_instance_class    = "db.t3.micro"
db_allocated_storage = 20
db_name              = "fiapx"
db_username          = "fiapx"
