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

- [Architecture](./docs/architecture/) — C4 context/container, data model, runtime flow, deployment topology.
- [Decision records](./docs/adr/) — why each major choice was made.

## Running locally

Start the backing infrastructure (also runs the DB migrations):

```bash
docker compose up -d
```

| Service   | URL |
| --------- | --- |
| Postgres  | `localhost:5432` (fiapx/fiapx) |
| Redis     | `localhost:6379` |
| RabbitMQ  | AMQP `localhost:5672` · UI [localhost:15672](http://localhost:15672) (guest/guest) |
| MinIO     | API `localhost:9000` · console [localhost:9001](http://localhost:9001) (minioadmin/minioadmin) |
| Mailpit   | SMTP `localhost:1025` · UI [localhost:8025](http://localhost:8025) |

Run the services on the host (dev defaults match the compose stack, so no `.env`
is needed — copy [`.env.example`](./.env.example) to `.env` to customize):

```bash
go run ./cmd/gateway    # :8080 — GET /healthz (liveness), GET /readyz (pings Postgres)
go run ./cmd/worker     # boots and waits for jobs
go run ./cmd/notifier   # boots and waits for events
```

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
go test ./...                                         # unit tests
go test -tags=integration ./...                       # integration tests (needs compose up)
```

> The real endpoints, queue consumers, and object-storage wiring arrive in
> subsequent phases.
