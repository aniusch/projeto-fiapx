# Terraform — EKS on AWS

Infrastructure to run FIAP X on Amazon EKS, tailored to the **AWS Academy
Learner Lab**. Organized as bootstrap → reusable modules → shared → per-env roots.

```
infra/terraform/
├── bootstrap/     # creates the S3 state bucket + DynamoDB lock (LOCAL state, run once)
├── modules/       # reusable building blocks
│   ├── network/   #   VPC, public subnets, IGW, routes
│   ├── eks/       #   cluster, node group, add-ons
│   ├── rds/       #   PostgreSQL + security group
│   ├── ecr/       #   image repositories
│   └── storage/   #   S3 videos bucket
├── shared/        # account-wide root: ECR (one registry for all environments)
└── envs/
    └── lab/       # the AWS Academy environment: network + eks + rds + storage
```

Each of `bootstrap`, `shared`, and `envs/lab` is a **separate root** with its own
state. Modules are never applied directly — the roots compose them.

## AWS Academy Learner Lab caveats

- **No IAM role creation** → cluster and nodes reuse **`LabRole`** (a variable);
  **IRSA is not used**, add-ons run on the node role.
- **Region is `us-east-1`**; credentials rotate (~4h) and include a session token.
- **Budget is small** → no NAT gateways (public subnets), on-demand nodes;
  `terraform destroy` when done.

## Deploy order

Credentials must be configured first (`aws sts get-caller-identity` succeeds).

```bash
cd infra/terraform

# 1) Bootstrap remote state (once per account). Uses local state.
cd bootstrap
terraform init
terraform apply -var "state_bucket_name=fiapx-tfstate-$(aws sts get-caller-identity --query Account --output text)"
cd ..

# 2) Shared account resources (ECR). Copy the printed backend values into backend.hcl.
cd shared
cp backend.hcl.example backend.hcl   # fill in the bucket name
terraform init -backend-config=backend.hcl
terraform apply
cd ..

# 3) The lab environment (VPC, EKS, RDS, S3).
cd envs/lab
cp backend.hcl.example backend.hcl   # fill in the bucket name
terraform init -backend-config=backend.hcl
export TF_VAR_db_password='choose-a-strong-password'
terraform apply

# 4) Configure kubectl and push images.
$(terraform output -raw kubeconfig_command)
cd ../..
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text) \
  REGION=us-east-1 ./scripts/push-images-to-ecr.sh
```

`terraform output` in `envs/lab` gives the RDS endpoint and S3 bucket; in `shared`
it gives the ECR URLs — all needed to wire the Kubernetes deployment.

## Adding another environment

Copy `envs/lab` to `envs/dev`, change the backend `key` (e.g.
`envs/dev/terraform.tfstate`) and any variable values, and re-init. The modules
and `shared` ECR are reused unchanged.

## Wiring the app & the pod→S3 credential note

The Kubernetes manifests in [`../k8s`](../k8s) target an all-in-cluster setup; an
**AWS overlay** is still needed to point `POSTGRES_DSN` at RDS, set the real S3
endpoint/region/bucket, and switch image names to ECR. (Not included yet — ask.)

Because Learner Lab blocks IRSA, pods can't get a scoped IAM role for S3. Either
inject the lab's temporary credentials (access key + secret + **session token**)
as a Secret — which needs a small app change to read `S3_SESSION_TOKEN` — or use
the node role via IMDS. See the details below and ask if you want the storage
client updated.

## Teardown

Destroy in reverse: `envs/lab`, then `shared`, then (optionally) `bootstrap`.

```bash
cd envs/lab && terraform destroy && cd ../..
cd shared   && terraform destroy && cd ..
# bootstrap last, only if you no longer need remote state
```
