# Remote state in S3 (+ DynamoDB lock). Configured with partial config so the
# bucket/table names aren't hard-coded here — pass them at init time:
#
#   terraform init -backend-config=backend.hcl
#
# The bucket and lock table must exist first; create them once with
# scripts/bootstrap-state.sh.
terraform {
  backend "s3" {}
}
