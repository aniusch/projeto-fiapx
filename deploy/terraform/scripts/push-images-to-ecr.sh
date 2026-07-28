#!/usr/bin/env bash
# Tags local images and pushes them to the ECR repos created by Terraform.
#
# In Learner Lab, GitHub Actions can't easily push to ECR (OIDC/IAM is
# restricted), so build or pull the images on a machine with lab credentials and
# run this. By default it re-tags the GHCR images that CI already builds; set
# SOURCE=local to push locally-built compose images instead.
#
#   REGION=us-east-1 ACCOUNT_ID=<id> TAG=latest ./scripts/push-images-to-ecr.sh
set -euo pipefail

REGION="${REGION:-us-east-1}"
ACCOUNT_ID="${ACCOUNT_ID:?Set ACCOUNT_ID (see: aws sts get-caller-identity)}"
PROJECT="${PROJECT:-fiapx}"
TAG="${TAG:-latest}"
SOURCE="${SOURCE:-ghcr}" # ghcr | local
GHCR_OWNER="${GHCR_OWNER:-aniusch}"

REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

echo "Logging in to ${REGISTRY}..."
aws ecr get-login-password --region "${REGION}" \
  | docker login --username AWS --password-stdin "${REGISTRY}"

for svc in gateway worker notifier; do
  case "${SOURCE}" in
    ghcr)  src="ghcr.io/${GHCR_OWNER}/projeto-fiapx-${svc}:${TAG}" ;;
    local) src="projeto-fiapx-${svc}:${TAG}" ;;
    *) echo "SOURCE must be 'ghcr' or 'local'"; exit 1 ;;
  esac
  dst="${REGISTRY}/${PROJECT}/${svc}:${TAG}"

  echo "==> ${src}  ->  ${dst}"
  [ "${SOURCE}" = "ghcr" ] && docker pull "${src}"
  docker tag "${src}" "${dst}"
  docker push "${dst}"
done

echo "Done. Images are in ECR under ${REGISTRY}/${PROJECT}/*"
