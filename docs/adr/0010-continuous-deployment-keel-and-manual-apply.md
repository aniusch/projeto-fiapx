# ADR-0010: Continuous deployment via Keel + a manual apply workflow

- **Status:** Accepted
- **Date:** 2026-07-29

## Context

We want changes to reach the EKS cluster with as little manual work as possible.
But the target is the AWS Academy Learner Lab, whose credentials **rotate (~4h)**
and which **forbids IAM changes** — so GitHub OIDC (the keyless way for Actions to
assume an AWS role) is unavailable, and long-lived AWS credentials stored in
GitHub simply expire. A push-button "deploy on every push" pipeline is therefore
fragile in this environment.

Two things need to happen on a change: the manifests may need re-applying, and new
container images need rolling out.

## Decision

Split CD into a credential-free automated path and a manual, credentialed path:

1. **Image rollout — Keel (in-cluster, automated).** [Keel](https://keel.sh) runs
   in the cluster and polls public GHCR. When CI pushes a new `:latest` digest,
   Keel updates the annotated Deployments (rolling restart). This needs **no AWS
   credentials** and is decoupled from the lab session, so routine code pushes
   deploy on their own: `push → CI → GHCR → Keel → pods`.

2. **Manifest apply — a manual GitHub Actions workflow.** `deploy.yml` is
   `workflow_dispatch`-only. It uses the *current* lab session credentials (kept
   as repo secrets and refreshed when they expire) to update kubeconfig, ensure
   ESO + Keel via Helm, and apply the overlay. Manual because the credentials must
   be refreshed per lab session.

## Consequences

- Everyday code changes roll out automatically through Keel without touching AWS
  credentials — the common case is friction-free.
- Infrastructure/manifest changes are a deliberate, manually-triggered apply,
  which is acceptable since they are rare.
- The manual workflow depends on someone pasting fresh lab credentials into the
  repo secrets; it is not true push-button CD for manifest changes.
- Keel polls a mutable `:latest` tag with a short lag (~2 min). For stronger
  guarantees, switch to immutable/semver tags and a matching Keel policy.

## Alternatives considered

- **GitHub OIDC → EKS.** The clean, keyless, fully-automated approach — but it
  requires creating an IAM OIDC provider and role, which Learner Lab forbids.
  This is the recommended path on a real AWS account.
- **Auto-deploy on every push with stored AWS credentials.** The lab's session
  credentials expire every few hours, so the pipeline would break constantly.
- **GitOps (Argo CD / Flux).** Powerful, but heavier to run than needed here;
  Keel covers "auto-rollout on a new image" with far less machinery.
