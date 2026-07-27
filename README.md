# FIAP X — Sistema de Processamento de Vídeos

Re-architecture of the FIAP X video processor (POSTECH SOAT — Fase 5 / Hackathon)
from a single-file monolith into a scalable, resilient, microservice-based system.

The original base project is preserved for reference under [`legacy/base-project/`](./legacy/base-project/).

## Architecture (target)

```
Browser/API ──▶ gateway ──(job)──▶ RabbitMQ ──▶ worker ──(frames.zip)──▶ MinIO/S3
                  │                                 │
                  ├─ Postgres (users, videos)       └──(failure event)──▶ notifier ──▶ email
                  └─ Redis   (tokens, rate-limit)
```

| Service    | Responsibility                                                        |
| ---------- | --------------------------------------------------------------------- |
| `gateway`  | Auth (JWT), upload, enqueue jobs, per-user status, download           |
| `worker`   | Consume jobs, run ffmpeg, zip frames, upload to S3, update status     |
| `notifier` | Consume failure events, email the affected user                       |

## Tech stack

Go · Gin · RabbitMQ · PostgreSQL · Redis · MinIO/S3 · Docker Compose · Kubernetes ·
Prometheus + Grafana · GitHub Actions.

## Project layout

```
cmd/            service entrypoints (gateway, worker, notifier)
internal/       private application code
  config/       env-based configuration
  platform/     cross-cutting helpers (logging, signals, metrics)
migrations/     SQL schema (added in Phase 2)
deploy/         docker-compose + k8s manifests (added later)
legacy/         the original base project, for reference
docs/           challenge PDF + architecture docs
```

## Running locally (Phase 1)

Each service reads config from environment variables (sensible dev defaults are
built in, so it boots without a `.env`):

```bash
go run ./cmd/gateway    # HTTP on :8080, GET /healthz
go run ./cmd/worker     # boots and waits for jobs
go run ./cmd/notifier   # boots and waits for events
```

Build all binaries:

```bash
go build ./...
```

> Infrastructure (Postgres, Redis, RabbitMQ, MinIO) and the real endpoints are
> added in subsequent phases.
