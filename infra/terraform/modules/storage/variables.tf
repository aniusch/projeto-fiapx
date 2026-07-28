variable "project" {
  type = string
}

variable "bucket_prefix" {
  description = "Prefix for the bucket name; a random suffix keeps it globally unique."
  type        = string
  default     = "fiapx-videos"
}
