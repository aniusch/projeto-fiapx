#!/usr/bin/env bash
# Destroys the Terraform remote-state backend (the S3 bucket + DynamoDB lock made
# by bootstrap/). The state bucket is versioned, so every object version and
# delete-marker is purged before the bucket can be destroyed. Run this from the
# machine that applied bootstrap (its state is local, under bootstrap/).
#
#   AWS_PROFILE=fiap ./scripts/teardown-bootstrap.sh
#
# Only do this once you no longer need any Terraform state (i.e. after the lab
# environment itself has been destroyed).
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)" # infra/terraform
BOOT="$HERE/bootstrap"

BUCKET="${BUCKET:-$(terraform -chdir="$BOOT" output -raw state_bucket 2>/dev/null || true)}"
: "${BUCKET:?Could not determine the state bucket; set BUCKET=fiapx-tfstate-<account-id>}"

echo "==> Emptying versioned state bucket s3://$BUCKET"
purge() {
  local key="$1" n batch
  while :; do
    n="$(aws s3api list-object-versions --bucket "$BUCKET" \
      --query "length(${key} || \`[]\`)" --output text 2>/dev/null || echo 0)"
    if [ "$n" = "0" ] || [ "$n" = "None" ]; then
      break
    fi
    batch="$(aws s3api list-object-versions --bucket "$BUCKET" --max-items 1000 \
      --query "{Objects: ${key}[].{Key:Key,VersionId:VersionId}}" --output json)"
    aws s3api delete-objects --bucket "$BUCKET" --delete "$batch" >/dev/null
  done
}
purge Versions
purge DeleteMarkers

echo "==> terraform destroy: bootstrap"
terraform -chdir="$BOOT" destroy -var "state_bucket_name=$BUCKET"

echo "Remote-state backend removed."
