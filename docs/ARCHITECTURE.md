# FIAP X — Architecture

This is the reviewer-facing architecture document for the FIAP X video-processing
system. It links to the detailed [diagrams](./architecture/) and the
[decision records](./adr/) rather than duplicating them.

## 1. Problem & goals

The base project ([`legacy/base-project/`](../legacy/base-project/)) is a single
Go file that accepts a video upload and runs ffmpeg **synchronously** inside the
HTTP handler, writing frames to local disk. It has no authentication, no
persistence, no way to scale, and loses work under load.

The goal is to re-architect it into a system that:

- processes **many videos in parallel**,
- **never loses a request** during traffic spikes,
- is **protected by user + password**,
- offers a **per-user status listing**,
- **notifies the user on failure**,
- **persists data**, is **horizontally scalable**, **tested**, and shipped via
  **CI/CD**.

See the [requirements traceability matrix](./requirements-traceability.md) for a
requirement-by-requirement map to the implementation.

## 2. High-level architecture

Three independently deployable services communicate asynchronously through
RabbitMQ, backed by Postgres (state), Redis (cache), and S3/MinIO (blobs).

```mermaid
C4Container
    title Container Diagram — FIAP X

    Person(user, "User", "Uploads and monitors videos")
    System_Ext(mail, "Email provider", "SMTP")

    Container_Boundary(fiapx, "FIAP X") {
        Container(gateway, "Gateway", "Go, Gin", "Auth (JWT), uploads, status, downloads. Publishes jobs.")
        Container(worker, "Worker", "Go, ffmpeg", "Stateless: consumes jobs, extracts frames, zips, uploads, emits status events. Scales horizontally.")
        Container(notifier, "Notifier", "Go", "Consumes failure events and emails the user.")
        ContainerDb(pg, "PostgreSQL", "Postgres 16", "Users and video job state (gateway-owned)")
        ContainerDb(redis, "Redis", "Redis 7", "Token cache and rate-limit counters")
        ContainerQueue(mq, "RabbitMQ", "AMQP 0-9-1", "Durable job queue + failure events, with a DLQ")
        ContainerDb(s3, "Object storage", "S3 / MinIO", "Source videos and result ZIPs")
    }

    Rel(user, gateway, "Uses", "HTTPS / JSON")
    Rel(gateway, pg, "Reads/writes", "SQL")
    Rel(gateway, redis, "Caches / rate-limits", "RESP")
    Rel(gateway, s3, "Stores source, signs downloads", "S3 API")
    Rel(gateway, mq, "Publishes jobs", "AMQP")
    Rel(mq, worker, "Delivers jobs", "AMQP")
    Rel(worker, s3, "Reads source, writes ZIP", "S3 API")
    Rel(worker, mq, "Publishes status & failures", "AMQP")
    Rel(mq, gateway, "Delivers status events", "AMQP")
    Rel(mq, notifier, "Delivers failures", "AMQP")
    Rel(notifier, mail, "Sends email", "SMTP")

    UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
```

More views: [system context](./architecture/c4-context.md) ·
[runtime flow](./architecture/runtime-upload-flow.md) ·
[data model](./architecture/data-model.md) ·
[deployment topology](./architecture/deployment-topology.md).

## 3. Services

| Service | Package | Responsibility |
| ------- | ------- | -------------- |
| **gateway** | [`cmd/gateway`](../cmd/gateway), [`internal/gateway`](../internal/gateway) | Public HTTP API: register/login (JWT), upload (→ S3 + enqueue), per-user status, download. Does no heavy processing. |
| **worker** | [`cmd/worker`](../cmd/worker), [`internal/worker`](../internal/worker), [`internal/processing`](../internal/processing) | **Stateless** — owns no database. Consumes jobs, runs ffmpeg, zips frames, uploads result, and emits `processing`/`done`/`failed` events. The only CPU-heavy tier. |
| **notifier** | [`cmd/notifier`](../cmd/notifier), [`internal/notifier`](../internal/notifier) | Consumes failure events and emails the affected user. |

Shared building blocks live in `internal/`: `config` (env config), `domain`
(core types), `platform` (DB/Redis/Rabbit connectors, logging, signals),
`repository/postgres`, `storage` (S3), `messaging` (queue topology + contracts),
`auth` (bcrypt + JWT), `mail` (SMTP), and `observability` (metrics).

## 4. Data & storage

- **Postgres** is the system of record: `users` and `videos`
  ([schema / DB creation script](../migrations/0001_init.up.sql)). `status` is a
  Postgres ENUM; a trigger maintains `updated_at`; the video→user FK cascades.
  **Only the gateway connects to it** — the worker and notifier are stateless, so
  each service is its own quantum ([ADR-0011](./adr/0011-database-per-service-single-writer.md)).
- **Redis** caches nothing critical — it holds rate-limit counters (and is the
  place to cache token/session lookups).
- **S3 / MinIO** stores the large blobs (source videos under `sources/…`, result
  archives under `results/<video_id>.zip`). The database stores only object keys.
  A separate public endpoint is used to **sign download URLs** so they are
  reachable by browser clients ([ADR-0008](./adr/0008-observability-prometheus-grafana.md)).

## 5. Messaging & the async pipeline

RabbitMQ decouples accepting work from doing it. Topology
([`internal/messaging`](../internal/messaging)):

- **jobs**: `videos.jobs` (durable) — the gateway publishes one persistent
  message per upload; workers consume competitively. Rejected messages
  dead-letter to `videos.jobs.dlq`.
- **events**: `videos.events` (topic) — the worker publishes lifecycle events
  (`video.processing` / `video.done` / `video.failed`). Two queues bind to it:
  `videos.status` (the gateway applies each event to the `videos` table it alone
  writes) and `videos.notifications` (the notifier emails the user on failure). A
  failure fans out to both. This is single-writer **event-carried state transfer**
  ([ADR-0011](./adr/0011-database-per-service-single-writer.md)).

The [runtime flow](./architecture/runtime-upload-flow.md) shows the full
sequence. Uploads return **202 Accepted** immediately after enqueuing.

## 6. Scalability & resilience

- **Parallelism / scale-out**: the worker is stateless; run N replicas
  (`--scale worker=N` in compose, or the **HorizontalPodAutoscaler** in
  Kubernetes, 2→6 on CPU). Competing consumers spread jobs automatically.
- **Spike safety**: the durable queue buffers bursts; the gateway only enqueues,
  so it stays responsive. Nothing is processed inline.
- **At-least-once + idempotency**: manual acks mean a crash mid-job redelivers
  the message. Reprocessing is safe — the result uses a deterministic key
  (overwrite, not duplicate), and the gateway applies status events idempotently
  (`MarkProcessing` only advances a still-`PENDING` row, so a redelivered event
  can't regress a terminal state). Infra errors retry once then dead-letter;
  processing failures are reported and acked.
- **Rate limiting**: a Redis fixed-window limiter on auth endpoints, enforced
  across all gateway replicas.
- **Graceful shutdown**: every service drains in-flight work on SIGTERM.

## 7. Security

- Passwords hashed with **bcrypt**; never stored or logged in plaintext.
- Stateless **JWT** (HS256) auth; the token middleware rejects the wrong
  algorithm (guards against alg-confusion).
- **Ownership checks**: a user can only see/download their own videos; others
  get 404 (existence is not revealed). Login returns a uniform 401 to avoid
  email enumeration.
- Secrets are injected via environment, never baked into the image or the repo.
  Locally they come from compose env / a Kubernetes `Secret`; on AWS the **External
  Secrets Operator** syncs them from **AWS Secrets Manager**
  ([ADR-0009](./adr/0009-secrets-external-secrets-operator.md)).

## 8. Observability

Every service exposes Prometheus metrics (`fiapx_<service>_<name>`): gateway HTTP
rate/latency, worker jobs-by-outcome + duration histogram + in-flight gauge,
notifier deliveries. Prometheus scrapes them; a provisioned **Grafana** dashboard
("FIAP X — Overview") ships in [`infra/grafana`](../infra/grafana). Structured
JSON logs (via `slog`) are emitted in production. See
[ADR-0008](./adr/0008-observability-prometheus-grafana.md).

## 9. Deployment

- **Docker Compose** ([`docker-compose.yml`](../docker-compose.yml)) runs the
  whole system — services, backing stores, and monitoring — with one command.
- **Kubernetes** ([`infra/k8s`](../infra/k8s)) provides Deployments, Services,
  an Ingress, probes wired to `/healthz` + `/readyz`, ConfigMap/Secret, a
  migration Job, and the worker HPA. Verified on minikube.
- **AWS / EKS** ([`infra/terraform`](../infra/terraform) +
  [`infra/k8s/overlays/aws`](../infra/k8s/overlays/aws)): EKS with managed RDS and
  S3, images from GHCR, and secrets via the External Secrets Operator; Redis,
  RabbitMQ, and Mailpit stay in-cluster.

See [ADR-0007](./adr/0007-deployment-compose-and-kubernetes.md) and the
[deployment topology](./architecture/deployment-topology.md).

## 10. Testing & CI/CD

- **Unit tests** use in-memory fakes (no external services) — auth, gateway
  handlers, worker/notifier logic, config, zip, SMTP formatting.
- **Integration tests** (build tag `integration`) exercise real Postgres, Redis,
  Mailpit, and the ffmpeg pipeline.
- **CI/CD** ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml)): lint,
  build, unit + integration tests against service containers, then build and push
  the three images to GHCR on `main`.
- **Code quality** ([`.github/workflows/sonar.yml`](../.github/workflows/sonar.yml)):
  a SonarCloud scan with Go coverage runs on every push and pull request.

## 11. Technology choices

| Concern | Choice | Rationale (ADR) |
| ------- | ------ | --------------- |
| Language | Go | [0003](./adr/0003-language-and-runtime-go.md) |
| Decomposition | 3 microservices | [0002](./adr/0002-microservice-decomposition.md) |
| Messaging | RabbitMQ (queue + DLQ) | [0004](./adr/0004-asynchronous-processing-rabbitmq.md) |
| Object storage | S3 / MinIO | [0005](./adr/0005-object-storage-s3-minio.md) |
| Datastores | Postgres + Redis | [0006](./adr/0006-datastores-postgres-redis.md) |
| Deployment | Compose + Kubernetes | [0007](./adr/0007-deployment-compose-and-kubernetes.md) |
| Observability | Prometheus + Grafana | [0008](./adr/0008-observability-prometheus-grafana.md) |

The full decision log is in [`docs/adr/`](./adr/).
