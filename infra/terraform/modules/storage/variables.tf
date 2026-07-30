variable "project" {
  type = string
}

variable "bucket_prefix" {
  description = "Prefix for the bucket name; a random suffix keeps it globally unique."
  type        = string
  default     = "fiapx-videos"
}

variable "force_destroy" {
  description = "Allow `terraform destroy` to delete the bucket even if it still holds objects. Handy for a short-lived lab; leave false in production."
  type        = bool
  default     = false
}
