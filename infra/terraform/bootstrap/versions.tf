terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.60"
    }
  }

  # Local state on purpose: this root creates the remote-state backend that every
  # other root uses, so it can't store its own state there (chicken-and-egg).
}

provider "aws" {
  region = var.region
  default_tags {
    tags = var.tags
  }
}
