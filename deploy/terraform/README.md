# Terraform — EKS on AWS

Provisions the AWS infrastructure to run FIAP X on Amazon EKS, tailored to the
**AWS Academy Learner Lab**.

## What it creates

| Resource | Notes |
| -------- | ----- |
| VPC + 2 public subnets + IGW | No NAT gateway (saves Learner Lab budget); nodes get public IPs |
| EKS cluster | Uses `LabRole`; creator gets cluster-admin via EKS access entries |
| Managed node group | `LabRole`, on-demand `t3.medium` ×2 (autoscales to 4) |
| Add-ons | vpc-cni, kube-proxy, coredns, aws-ebs-csi-driver (node-role, no IRSA) |
| RDS PostgreSQL | Private, reachable only from the cluster security group |
| ECR repos | `fiapx/gateway`, `fiapx/worker`, `fiapx/notifier` |
| S3 bucket | Video storage, encrypted, public access blocked, CORS for presigned URLs |

Redis, RabbitMQ, and Mailpit run **in-cluster** (see [`../k8s`](../k8s)); RDS and
S3 are managed.

## AWS Academy Learner Lab caveats

- **No IAM role creation.** Everything reuses the pre-provisioned **`LabRole`**
  for the control plane and nodes. Consequently **IRSA is not used** — pods rely
  on the node role. On a normal account, create dedicated roles and set
  `lab_role_name` accordingly.
- **Region is `us-east-1`.** Learner Lab only allows this region.
- **Credentials rotate (~4h).** Get them from the lab's *AWS Details → CLI* and
  put them in `~/.aws/credentials` (they include a **session token**). Re-fetch
  when they expire.
- **Budget is small.** EKS control plane, nodes, and RDS all bill hourly —
  `terraform destroy` when you're done, and don't leave it running overnight.
- **State bucket** lives in S3 (see below); it survives across lab sessions.

## Prerequisites

`terraform >= 1.5`, `aws` CLI, `kubectl`, `docker`, and valid lab credentials
(`aws sts get-caller-identity` should succeed).

## Deploy

```bash
cd deploy/terraform

# 1) One-time: create the remote-state bucket + lock table.
REGION=us-east-1 BUCKET=fiapx-tfstate-<account-id> ./scripts/bootstrap-state.sh
cp backend.hcl.example backend.hcl   # fill in the bucket name

# 2) Init with the backend config.
terraform init -backend-config=backend.hcl

# 3) Provide the DB password out-of-band (never commit it).
export TF_VAR_db_password='choose-a-strong-password'

# 4) Review and apply.
terraform plan
terraform apply

# 5) Point kubectl at the new cluster.
$(terraform output -raw kubeconfig_command)

# 6) Push images to ECR (from a machine with lab creds).
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text) \
  REGION=us-east-1 ./scripts/push-images-to-ecr.sh
```

`terraform output` then gives you the RDS endpoint, ECR URLs, and S3 bucket to
plug into the Kubernetes deployment.

## Wiring the app (next step)

The Kubernetes manifests in [`../k8s`](../k8s) target an all-in-cluster setup. An
**AWS overlay** is needed to: drop the in-cluster Postgres and point
`POSTGRES_DSN` at RDS; set `S3_ENDPOINT=s3.us-east-1.amazonaws.com`,
`S3_USE_SSL=true`, `S3_REGION`, and the bucket; and switch the image names to the
ECR URLs. (Not included here yet — ask and it can be generated.)

### S3 credentials for pods — read this

Because Learner Lab blocks IRSA, pods can't get a scoped IAM role. Two options:

1. **Inject the lab's temporary credentials as a Secret** (simplest to get
   working): put `S3_ACCESS_KEY`, `S3_SECRET_KEY`, **and the session token** into
   a Kubernetes Secret. Note the current app passes an empty session token to the
   S3 client, so this needs a small code change to read `S3_SESSION_TOKEN`. These
   creds rotate ~4h, so the Secret must be refreshed each session.
2. **Use the node role via IMDS**: `LabRole` already has S3 access. The app would
   use the AWS default credential chain (empty static keys), and the node group's
   IMDS hop limit must allow pod access. This avoids rotating secrets but needs
   both an app change and a launch-template tweak.

Ask if you want the storage client updated to support either path.

## Teardown

```bash
terraform destroy
```

Delete the state bucket/lock table separately if you no longer need remote state.
