variable "region" {
  description = "AWS region. AWS Academy Learner Lab is limited to us-east-1."
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Short name used to prefix resources."
  type        = string
  default     = "fiapx"
}

variable "cluster_name" {
  description = "EKS cluster name."
  type        = string
  default     = "fiapx"
}

variable "kubernetes_version" {
  description = "EKS Kubernetes version."
  type        = string
  default     = "1.30"
}

variable "lab_role_name" {
  description = <<-EOT
    Name of the pre-provisioned IAM role used for the EKS cluster and nodes.
    In AWS Academy Learner Lab this is "LabRole" (you cannot create IAM roles).
    On a normal account, create suitable roles and set this accordingly.
  EOT
  type        = string
  default     = "LabRole"
}

# --- Networking -----------------------------------------------------------

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Number of Availability Zones (>= 2 for EKS and RDS)."
  type        = number
  default     = 2
}

# --- Node group -----------------------------------------------------------

variable "node_instance_type" {
  description = "EC2 instance type for worker nodes."
  type        = string
  default     = "t3.medium"
}

variable "node_desired_size" {
  type    = number
  default = 2
}

variable "node_min_size" {
  type    = number
  default = 2
}

variable "node_max_size" {
  type    = number
  default = 4
}

variable "node_disk_size" {
  description = "Node root EBS volume size (GiB)."
  type        = number
  default     = 20
}

# --- RDS (PostgreSQL) -----------------------------------------------------

variable "db_engine_version" {
  description = "PostgreSQL engine version available in the target account/region."
  type        = string
  default     = "16.4"
}

variable "db_instance_class" {
  type    = string
  default = "db.t3.micro"
}

variable "db_allocated_storage" {
  type    = number
  default = 20
}

variable "db_name" {
  type    = string
  default = "fiapx"
}

variable "db_username" {
  type    = string
  default = "fiapx"
}

variable "db_password" {
  description = "Master password for the RDS instance. Provide via TF_VAR_db_password or a tfvars file; never commit it."
  type        = string
  sensitive   = true
}

# --- Registry -------------------------------------------------------------

variable "ecr_repositories" {
  description = "ECR repositories to create (one per service image)."
  type        = list(string)
  default     = ["gateway", "worker", "notifier"]
}

# --- Tags -----------------------------------------------------------------

variable "tags" {
  description = "Tags applied to all resources."
  type        = map(string)
  default = {
    Project   = "fiapx"
    ManagedBy = "terraform"
  }
}
