variable "project" {
  type = string
}

variable "repositories" {
  description = "Repository names (created as <project>/<name>)."
  type        = list(string)
  default     = ["gateway", "worker", "notifier"]
}
