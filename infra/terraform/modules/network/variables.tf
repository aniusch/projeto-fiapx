variable "project" {
  type = string
}

variable "cluster_name" {
  description = "Used to tag subnets for the EKS cluster."
  type        = string
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "az_count" {
  type    = number
  default = 2
}
