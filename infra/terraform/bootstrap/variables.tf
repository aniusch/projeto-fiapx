variable "region" {
  type    = string
  default = "us-east-1"
}

variable "state_bucket_name" {
  description = "Globally-unique name for the Terraform state bucket, e.g. fiapx-tfstate-<account-id>."
  type        = string
}

variable "lock_table_name" {
  type    = string
  default = "fiapx-tflock"
}

variable "tags" {
  type = map(string)
  default = {
    Project   = "fiapx"
    ManagedBy = "terraform"
    Component = "bootstrap"
  }
}
