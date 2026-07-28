variable "region" {
  description = "AWS Academy Learner Lab is limited to us-east-1."
  type        = string
  default     = "us-east-1"
}

variable "project" {
  type    = string
  default = "fiapx"
}

variable "cluster_name" {
  type    = string
  default = "fiapx"
}

variable "kubernetes_version" {
  type    = string
  default = "1.30"
}

variable "lab_role_name" {
  description = "Pre-provisioned IAM role reused for the cluster and nodes. LabRole in Learner Lab (IAM role creation is forbidden)."
  type        = string
  default     = "LabRole"
}

# --- Networking ---
variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "az_count" {
  type    = number
  default = 2
}

# --- Nodes ---
variable "node_instance_type" {
  type    = string
  default = "t3.medium"
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
  type    = number
  default = 20
}

# --- RDS ---
variable "db_engine_version" {
  type    = string
  default = "16.4"
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
  description = "RDS master password. Provide via TF_VAR_db_password; never commit it."
  type        = string
  sensitive   = true
}

variable "tags" {
  type = map(string)
  default = {
    Project   = "fiapx"
    ManagedBy = "terraform"
    Env       = "lab"
  }
}
