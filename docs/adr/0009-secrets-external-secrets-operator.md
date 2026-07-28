# ADR-0009: Secrets via External Secrets Operator, authenticated by the node role

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

The Kubernetes deployment needs sensitive values — the Postgres connection string
(with the RDS password) and the JWT signing secret. The base manifests carry them
as a plaintext `Secret` committed to git, which is only acceptable for a local
demo.

The AWS target is the **AWS Academy Learner Lab**, which forbids creating IAM
roles. That rules out **IRSA** (IAM Roles for Service Accounts), the usual way to
give a pod (or an operator) scoped AWS permissions. Separately, the app itself
needs to reach S3, and any secrets tooling needs to reach a secret store — both
without IRSA.

## Decision

We will manage secrets with the **External Secrets Operator (ESO)** syncing from
**AWS Secrets Manager**:

- Terraform writes a `fiapx/app` secret (JSON: `POSTGRES_DSN`, `JWT_SECRET`) to
  Secrets Manager, building the DSN from the RDS endpoint. The JWT secret is
  Terraform-generated.
- An ESO `SecretStore` + `ExternalSecret` materialize a Kubernetes `Secret`
  (`fiapx-secrets`) from it, which the services consume via `envFrom`.

Because IRSA is unavailable, **both ESO and the app authenticate with the EKS node
role (`LabRole`) via IMDS**. This is enabled by a node launch template setting
IMDSv2 with a metadata hop limit of 2, so pods (not just the host) can reach the
instance metadata service. The same mechanism gives the app its S3 credentials
(`S3_USE_IAM=true`), so there is a single, credential-free auth path.

## Consequences

- No plaintext secrets in git; secret values live in AWS and are rotated/managed
  there. The workflow is cloud-native and mirrors real production practice.
- One auth mechanism (node role via IMDS) serves both S3 and Secrets Manager —
  no static keys and none of the Learner Lab's rotating session credentials leak
  into the cluster.
- Trade-off: the node role is broad, and hop-limit-2 lets **any** pod on the node
  assume it. That is acceptable in a sandbox lab but is *not* least-privilege. On
  a real account this should be replaced by **IRSA** (a scoped role per service
  account), which removes the need to relax pod IMDS access.
- Operational cost: ESO is another component to install (via Helm) and reason
  about; the `ExternalSecret` must reconcile before dependent pods can start.

## Alternatives considered

- **IRSA + Secrets Manager (the ideal).** The least-privilege, per-workload
  approach — but it requires creating an IAM OIDC provider and roles, which
  Learner Lab forbids. This is the recommended path on a real account.
- **Sealed Secrets.** In-cluster encryption with no AWS dependency; commit an
  encrypted `SealedSecret`. Simpler and works cleanly in the lab, but it is not
  AWS-native and does not exercise a cloud secret manager (a goal here).
- **Static/temporary credentials in a Secret.** Injecting the lab's rotating
  session credentials means refreshing them every session and re-introduces the
  bootstrap-secret problem ESO was meant to avoid.
- **Plaintext `Secret` (status quo).** Fine for the local demo, unacceptable for
  a shared/versioned deployment.
