# ADR-0007: Ship both Docker Compose and Kubernetes

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

The system has several backing services (Postgres, Redis, RabbitMQ, MinIO, mail)
plus three application services. We need (a) a frictionless local development and
demo experience, and (b) a deployment target that demonstrates real horizontal
scalability, as the requirements ask.

## Decision

We will maintain **both** deployment definitions:

- **Docker Compose** for local development and the demo — one command brings up
  all infrastructure and runs migrations. Application services run on the host via
  `go run` (or as compose services) during development.
- **Kubernetes** manifests for the "scalable architecture" story — Deployments
  for each service, a HorizontalPodAutoscaler on the worker, ConfigMaps/Secrets
  for configuration, and probes wired to `/healthz` and `/readyz`.

## Consequences

- Contributors and reviewers can run the whole system locally with `docker
  compose up` — low barrier to entry and a reliable demo.
- The Kubernetes manifests concretely demonstrate scaling the CPU-heavy worker
  independently (HPA on the worker deployment).
- The liveness/readiness endpoints already built into the gateway map directly
  onto Kubernetes probes.
- Cost: two deployment definitions to keep in sync as configuration evolves.

## Alternatives considered

- **Compose only.** Simplest, and allowed by the rules, but a weaker
  demonstration of the horizontal-scaling requirement.
- **Kubernetes only.** Strong scaling story, but a heavier local/demo loop
  (needs a cluster like kind/minikube) and slower day-to-day iteration.
