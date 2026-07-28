# Terraform — EKS on AWS

Infrastructure to run FIAP X on Amazon EKS, tailored to the **AWS Academy
Learner Lab**. Organized as bootstrap → reusable modules → per-env roots.

```
infra/terraform/
├── bootstrap/     # creates the S3 state bucket + DynamoDB lock (LOCAL state, run once)
├── modules/       # reusable building blocks
│   ├── network/   #   VPC, public subnets, IGW, routes
│   ├── eks/       #   cluster, node group, add-ons
│   ├── rds/       #   PostgreSQL + security group
│   └── storage/   #   S3 videos bucket
└── envs/
    └── lab/       # the AWS Academy environment: network + eks + rds + storage
```

Both `bootstrap` and `envs/lab` are **separate roots** with their own state.
Modules are never applied directly — the env root composes them.

> **Images come from GHCR, not ECR.** CI already builds and pushes to
> `ghcr.io/aniusch/projeto-fiapx-{gateway,worker,notifier}`. Make those packages
> **public** (GitHub → repo → Packages → *Package settings* → Change visibility)
> and EKS pulls them with no registry secret and no AWS credentials in CI — the
> lowest-friction path for the Learner Lab. (If you later move to a real AWS
> account, ECR + GitHub OIDC can be added back; ask.)

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

# 2) The lab environment (VPC, EKS, RDS, S3).
cd envs/lab
cp backend.hcl.example backend.hcl   # fill in the bucket name from step 1
terraform init -backend-config=backend.hcl
export TF_VAR_db_password='choose-a-strong-password'
terraform apply

# 3) Configure kubectl.
$(terraform output -raw kubeconfig_command)
```

`terraform output` in `envs/lab` gives the RDS endpoint and S3 bucket needed to
wire the Kubernetes deployment.

## Adding another environment

Copy `envs/lab` to `envs/dev`, change the backend `key` (e.g.
`envs/dev/terraform.tfstate`) and any variable values, and re-init. The modules
are reused unchanged.

## Deploying the app

The AWS deployment overlay is [`../k8s/overlays/aws`](../k8s/overlays/aws): it
points Postgres at RDS, uses real S3 via the node role, pulls images from public
GHCR, and sources secrets from AWS Secrets Manager through the **External Secrets
Operator**. Follow that overlay's README (`terraform output` here provides the
S3 bucket and RDS endpoint it needs).

Because Learner Lab blocks IRSA, both ESO and the app authenticate with the **node
role (`LabRole`) via IMDS** — enabled by the node launch template's IMDS hop limit
of 2. So there are no static/rotating credentials in the cluster.

The in-cluster services (Redis, RabbitMQ, Mailpit) run **without
PersistentVolumeClaims**, so the cluster provisions no EBS volumes and **no EBS CSI
driver is installed**. Trade-off: a RabbitMQ pod restart drops any queued jobs —
fine for a demo. For durable RabbitMQ, add a PVC plus the `aws-ebs-csi-driver`
add-on (it works in Learner Lab via the node role).

## Teardown

```bash
cd envs/lab && terraform destroy && cd ../..
# bootstrap last, only if you no longer need remote state
```
