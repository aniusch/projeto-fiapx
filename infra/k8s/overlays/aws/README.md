# AWS overlay (EKS)

Deploys FIAP X to the EKS cluster from [`infra/terraform/envs/lab`](../../../terraform/envs/lab).
Differences from the base (all-in-cluster) manifests:

- **Postgres → RDS** and **MinIO → real S3** (the in-cluster manifests are dropped).
  S3 is authenticated via the **node role** (`S3_USE_IAM=true`, no keys).
- **Images** are pulled from public **GHCR** (no ECR, no pull secret).
- **Secrets** come from **AWS Secrets Manager** via the **External Secrets
  Operator (ESO)** — the `SecretStore` + `ExternalSecret` here materialize
  `fiapx-secrets` (`POSTGRES_DSN`, `JWT_SECRET`) from the `fiapx/app` secret that
  Terraform created. No plaintext Secret in git.

Redis, RabbitMQ, and Mailpit still run in-cluster.

## How the auth works (no IRSA)

Learner Lab can't provision IRSA, so both ESO and the app authenticate with the
**EKS node role (`LabRole`)** via IMDS. That's enabled by the node launch
template's IMDS hop limit of 2 (see the eks module). `LabRole` is permissive, so
it already has `secretsmanager:GetSecretValue` and S3 access.

## Deploy

Prereqs: `terraform apply` in `envs/lab` done, `kubectl` pointed at the cluster.

```bash
# 1) Install the External Secrets Operator and Keel (once).
helm repo add external-secrets https://charts.external-secrets.io
helm upgrade --install external-secrets external-secrets/external-secrets \
  -n external-secrets --create-namespace

helm repo add keel https://charts.keel.sh
helm upgrade --install keel keel/keel -n keel --create-namespace

# 2) Set the bucket name in configmap.yaml from Terraform output.
#    (cd ../../../terraform/envs/lab && terraform output -raw s3_bucket)
$EDITOR configmap.yaml   # S3_BUCKET: <the bucket>

# 3) Apply the overlay.
kubectl kustomize --load-restrictor LoadRestrictionsNone . | kubectl apply -f -

# 4) Watch ESO materialize the secret, then the app come up.
kubectl -n fiapx get externalsecret,secret
kubectl -n fiapx get pods -w
```

### Or via GitHub Actions

The [`Deploy (lab)`](../../../../.github/workflows/deploy.yml) workflow does steps
1–4 for you. It is **manually dispatched** (Learner Lab credentials rotate and
can't use OIDC), so add the current lab credentials as repo secrets —
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` — refreshing them
when they expire, then run the workflow with the `s3_bucket` input from
`terraform output -raw s3_bucket`.

## Continuous rollout (Keel)

The gateway/worker/notifier Deployments are annotated for [Keel](https://keel.sh)
(`keel.sh/policy: force`, `trigger: poll`, `pollSchedule: @every 2m`). Keel polls
GHCR and, when the `:latest` digest changes (i.e. CI pushed a new build), updates
the Deployment — a rolling restart onto the new image. This is pull-based CD that
needs no AWS credentials and is decoupled from the (manual) infra deploy: once the
cluster is up, a normal `git push` to `main` flows through CI → GHCR → Keel → pods.

```bash
kubectl -n keel logs deploy/keel   # watch it detect and apply updates
```

## Access & verify

```bash
kubectl -n fiapx port-forward svc/gateway 8080:8080
# then use the API as usual (see the top-level README)
```

## Troubleshooting

- **`fiapx-secrets` not created** → check ESO: `kubectl -n external-secrets logs deploy/external-secrets`
  and `kubectl -n fiapx describe externalsecret fiapx-secrets`. Usually the node
  role can't reach Secrets Manager (IMDS hop limit, or missing permission).
- **App pods `CreateContainerConfigError`** → they're waiting for `fiapx-secrets`;
  they start once ESO creates it.
