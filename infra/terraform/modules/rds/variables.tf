variable "project" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnet_ids" {
  description = "Subnets for the DB subnet group (>= 2 AZs)."
  type        = list(string)
}

variable "allowed_security_group_ids" {
  description = "Security groups permitted to reach Postgres (e.g. the EKS cluster SG)."
  type        = list(string)
}

variable "engine_version" {
  type    = string
  default = "16.4"
}

variable "instance_class" {
  type    = string
  default = "db.t3.micro"
}

variable "allocated_storage" {
  type    = number
  default = 20
}

variable "db_name" {
  type    = string
  default = "fiapx"
}

variable "username" {
  type    = string
  default = "fiapx"
}

variable "password" {
  type      = string
  sensitive = true
}
