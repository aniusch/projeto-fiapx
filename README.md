# FIAP X — Sistema de Processamento de Vídeos

[![CI](https://github.com/aniusch/projeto-fiapx/actions/workflows/ci.yml/badge.svg)](https://github.com/aniusch/projeto-fiapx/actions/workflows/ci.yml)

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
cmd/                  service entrypoints (gateway, worker, notifier)
internal/             private application code
  config/             env-based configuration
  platform/           cross-cutting helpers (logging, signals, Postgres pool)
  domain/             core business types (User, Video) — no infra dependencies
  repository/postgres/ Postgres-backed repositories
migrations/           versioned SQL schema (also the DB creation script)
docs/
  adr/                Architecture Decision Records
  architecture/       C4, ERD, runtime & deployment diagrams (Mermaid)
legacy/               the original base project, for reference
docker-compose.yml    local infrastructure (Postgres, Redis, RabbitMQ, MinIO, Mailpit)
```

## Documentation

- [**Architecture**](./docs/ARCHITECTURE.md) — the reviewer-facing overview (start here).
- [Diagrams](./docs/architecture/) — C4 context/container, data model, runtime flow, deployment topology.
- [Decision records](./docs/adr/) — why each major choice was made.
- [Requirements traceability](./docs/requirements-traceability.md) — each requirement → where it's met.
- [Demo script](./docs/demo-script.md) — run sheet for the ≤10-minute video.

## Running locally

Common tasks are wrapped in a [`Makefile`](./Makefile) — run `make help` for the
list (`make up`, `make check`, `make cover`, `make test-integration`, …).

Bring up the whole system (backing services, the three app services, monitoring,
and the DB migrations) with one command:

```bash
docker compose up -d
```

| Component  | URL |
| ---------- | --- |
| Gateway API | [localhost:8080](http://localhost:8080) |
| Postgres   | `localhost:5432` (fiapx/fiapx) |
| Redis      | `localhost:6379` |
| RabbitMQ   | AMQP `localhost:5672` · UI [localhost:15672](http://localhost:15672) (guest/guest) |
| MinIO      | API `localhost:9000` · console [localhost:9001](http://localhost:9001) (minioadmin/minioadmin) |
| Mailpit    | SMTP `localhost:1025` · UI [localhost:8025](http://localhost:8025) |
| Prometheus | [localhost:9090](http://localhost:9090) |
| Grafana    | [localhost:3000](http://localhost:3000) (admin/admin) — "FIAP X — Overview" dashboard |

Scale the CPU-heavy worker horizontally:

```bash
docker compose up -d --scale worker=3
```

### Running a service on the host

For fast iteration you can run a single service on the host instead of its
container (dev defaults match the compose stack, so no `.env` is needed — copy
[`.env.example`](./.env.example) to `.env` to customize). Stop the container
first to free its port, e.g. `docker compose stop gateway`, then:

```bash
go run ./cmd/gateway    # :8080 — GET /healthz, /readyz, /metrics
go run ./cmd/notifier   # consumes failure events
go run ./cmd/worker     # requires ffmpeg on the host (or set FFMPEG_PATH)
```

## Kubernetes

Manifests for the whole system live in [`infra/k8s/`](./infra/k8s/) (Kustomize):
Deployments for the three services, in-cluster backing services, a migration Job,
liveness/readiness probes, an Ingress, and a **HorizontalPodAutoscaler on the
worker** (2→6 replicas on CPU) — the concrete "scale the CPU-heavy tier"
demonstration. See [`infra/k8s/README.md`](./infra/k8s/README.md) for the deploy
steps; the short version:

```bash
minikube image load fiapx/gateway:latest fiapx/worker:latest fiapx/notifier:latest
kubectl kustomize --load-restrictor LoadRestrictionsNone infra/k8s | kubectl apply -f -
```

## CI/CD

[GitHub Actions](./.github/workflows/ci.yml) runs on every push and pull request:

1. **Lint, build & test** — `gofmt` check, `go vet`, build, unit tests (`-race`),
   then integration tests against Postgres/Redis/Mailpit service containers with
   ffmpeg installed and migrations applied.
2. **Build & push images** — on pushes to `main`/tags, builds the three service
   images and pushes them to the GitHub Container Registry
   (`ghcr.io/aniusch/projeto-fiapx-{gateway,worker,notifier}`), with build caching.

To deploy those published images to Kubernetes, point the manifests at GHCR by
editing the `images:` block in [`infra/k8s/kustomization.yaml`](./infra/k8s/kustomization.yaml).

## Monitoring

Every service exposes Prometheus metrics (`fiapx_<service>_<name>`): gateway HTTP
rate/latency, worker jobs-processed-by-outcome, job-duration histogram and
in-flight gauge, and notifier deliveries. Prometheus scrapes them and Grafana
renders the provisioned dashboard out of the box at
[localhost:3000](http://localhost:3000).

## API

All video routes require a `Authorization: Bearer <token>` header. Auth routes are
rate-limited per client IP (60 req/min).

| Method | Path | Auth | Description |
| ------ | ---- | ---- | ----------- |
| `POST` | `/auth/register` | — | Create account `{email, password}` → `{token}` (201) |
| `POST` | `/auth/login` | — | `{email, password}` → `{token}` (200) |
| `POST` | `/videos` | ✓ | Multipart upload (`video` field) → stores source, enqueues job (202, `PENDING`) |
| `GET` | `/videos` | ✓ | List the caller's videos with status + download URL |
| `GET` | `/videos/:id` | ✓ | Single video status |
| `GET` | `/videos/:id/download` | ✓ | Redirect to a presigned URL for the result zip (409 until `DONE`) |
| `GET` | `/healthz` `/readyz` | — | Liveness / readiness (readiness pings Postgres + Redis) |

Example:

```bash
TOKEN=$(curl -s -X POST localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"password123"}' | jq -r .token)

curl -X POST localhost:8080/videos \
  -H "Authorization: Bearer $TOKEN" -F "video=@myclip.mp4"

curl localhost:8080/videos -H "Authorization: Bearer $TOKEN"
```

## Tests

```bash
go build ./...                                       # compile everything
go vet ./...                                          # static checks
go test ./...                                         # unit tests (no external deps)
go test -tags=integration ./...                       # integration tests (needs compose up)
go test -cover ./...                                  # with coverage
```

Unit tests use in-memory fakes for the databases, broker, and object store, so
they run anywhere. Integration tests (build tag `integration`) exercise the real
Postgres, Redis, and SMTP from the compose stack; the ffmpeg pipeline test
generates its own clip and skips automatically if ffmpeg isn't installed.

> The real endpoints, queue consumers, and object-storage wiring arrive in
> subsequent phases.
