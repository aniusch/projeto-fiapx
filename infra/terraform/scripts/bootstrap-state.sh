#!/usr/bin/env bash
# Creates the S3 bucket and DynamoDB table that hold Terraform remote state.
# Run once, before `terraform init`. Safe to re-run (it ignores "already exists").
#
#   REGION=us-east-1 BUCKET=fiapx-tfstate-<account-id> TABLE=fiapx-tflock \
#     ./scripts/bootstrap-state.sh
set -euo pipefail

REGION="${REGION:-us-east-1}"
TABLE="${TABLE:-fiapx-tflock}"
BUCKET="${BUCKET:?Set BUCKET to a globally-unique name, e.g. fiapx-tfstate-<account-id>}"

echo "Creating state bucket s3://${BUCKET} in ${REGION}..."
if [ "${REGION}" = "us-east-1" ]; then
  aws s3api create-bucket --bucket "${BUCKET}" --region "${REGION}" 2>/dev/null || true
else
  aws s3api create-bucket --bucket "${BUCKET}" --region "${REGION}" \
    --create-bucket-configuration "LocationConstraint=${REGION}" 2>/dev/null || true
fi

aws s3api put-bucket-versioning --bucket "${BUCKET}" \
  --versioning-configuration Status=Enabled

aws s3api put-bucket-encryption --bucket "${BUCKET}" \
  --server-side-encryption-configuration \
  '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'

echo "Creating lock table ${TABLE}..."
aws dynamodb create-table \
  --table-name "${TABLE}" \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region "${REGION}" 2>/dev/null || true

echo "Done. Put these into backend.hcl:"
echo "  bucket=\"${BUCKET}\" key=\"eks/terraform.tfstate\" region=\"${REGION}\" dynamodb_table=\"${TABLE}\" encrypt=true"
