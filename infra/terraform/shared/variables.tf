variable "region" {
  type    = string
  default = "us-east-1"
}

variable "project" {
  type    = string
  default = "fiapx"
}

variable "ecr_repositories" {
  type    = list(string)
  default = ["gateway", "worker", "notifier"]
}

variable "tags" {
  type = map(string)
  default = {
    Project   = "fiapx"
    ManagedBy = "terraform"
    Scope     = "shared"
  }
}
