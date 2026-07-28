variable "cluster_name" {
  type = string
}

variable "kubernetes_version" {
  type = string
}

variable "cluster_role_arn" {
  description = "IAM role for the EKS control plane (LabRole in Learner Lab)."
  type        = string
}

variable "node_role_arn" {
  description = "IAM role for the worker nodes (LabRole in Learner Lab)."
  type        = string
}

variable "subnet_ids" {
  type = list(string)
}

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

variable "node_imds_hop_limit" {
  description = "IMDSv2 hop limit for nodes. 2 lets pods reach the metadata service and assume the node role (needed for S3 and ESO without IRSA)."
  type        = number
  default     = 2
}
