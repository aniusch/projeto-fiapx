#!/usr/bin/env bash
# Tears down the lab environment to stop the budget clock: destroys the EKS
# cluster, RDS, S3, Secrets Manager entry, VPC, etc. Run from a machine with the
# lab credentials (e.g. AWS_PROFILE=fiap) after you `terraform init`-ed envs/lab.
#
#   AWS_PROFILE=fiap TF_VAR_db_password=whatever ./scripts/teardown.sh
#
# The remote-state backend (bootstrap: S3 bucket + DynamoDB) is left in place —
# it costs almost nothing and holds state across sessions. Pass --all to destroy
# that too (empty the versioned state bucket first if it complains).
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)" # infra/terraform
LAB="$HERE/envs/lab"

: "${TF_VAR_db_password:?Set TF_VAR_db_password — Terraform needs it even to destroy}"

# Best-effort: remove the app namespace first so any Kubernetes-managed AWS
# resources (load balancers, volumes) are cleaned up before the cluster goes.
if command -v kubectl >/dev/null 2>&1; then
  echo "==> Deleting the fiapx namespace (best-effort)"
  kubectl delete namespace fiapx --ignore-not-found --wait=false || true
fi

echo "==> terraform destroy: envs/lab"
terraform -chdir="$LAB" destroy -var-file=lab.tfvars

if [ "${1:-}" = "--all" ]; then
  echo "==> terraform destroy: bootstrap (remote state)"
  terraform -chdir="$HERE/bootstrap" destroy
fi

echo "Teardown complete."
