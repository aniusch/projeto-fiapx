# Requirements traceability

Each requirement from the challenge brief mapped to where it is implemented and
how it was verified.

## Functional requirements

| Requirement | Where it's met | How it's verified |
| ----------- | -------------- | ----------------- |
| Process more than one video at the same time | Stateless `worker` scaled by replicas; prefetch-bounded concurrent consumers ([`internal/worker/consumer.go`](../internal/worker/consumer.go)); `--scale worker=N` / [HPA](../deploy/k8s/worker.yaml) | Compose + k8s deploys; HPA observed at `cpu:70%` target |
| Must not lose a request during spikes | Gateway only enqueues and returns **202**; durable RabbitMQ queue with persistent messages + DLQ ([`internal/messaging`](../internal/messaging)) | e2e: upload lands a message in `videos.jobs`; worker drains it |
| Protected by user + password | bcrypt + JWT ([`internal/auth`](../internal/auth)); auth middleware on `/videos/*` ([`internal/gateway/middleware.go`](../internal/gateway/middleware.go)) | Unit tests (auth, gateway); e2e register/login/401 |
| Per-user status listing | `GET /videos` scoped to the caller ([`internal/gateway/video_handler.go`](../internal/gateway/video_handler.go)); indexed `videos(user_id, created_at)` | Gateway tests; e2e listing |
| Notify the user on error | `worker` publishes `VideoFailedEvent`; `notifier` emails via SMTP ([`internal/notifier`](../internal/notifier), [`internal/mail`](../internal/mail)) | e2e: failed video → email visible in Mailpit; unit + SMTP integration test |

## Technical requirements

| Requirement | Where it's met | How it's verified |
| ----------- | -------------- | ----------------- |
| Persist data | Postgres (`users`, `videos`) + MinIO/S3 blobs; [migrations](../migrations/0001_init.up.sql) | Repository integration test; e2e status survives restarts |
| Scalable architecture | Stateless services, shared queue/stores, worker HPA | k8s deploy on minikube; HPA reads CPU |
| Versioned on GitHub | Git repository, one commit per phase | `git log` |
| Tests that ensure quality | Unit tests (in-memory fakes) + integration tests (tag `integration`) | `go test ./...` and `-tags=integration ./...` green |
| CI/CD | [GitHub Actions](../.github/workflows/ci.yml): lint, test, build & push images | Validated with actionlint |

## Recommended stack

| Area | Recommended | Chosen |
| ---- | ----------- | ------ |
| Containers | Docker + Kubernetes **or** Compose | **Both** — [`docker-compose.yml`](../docker-compose.yml) and [`deploy/k8s`](../deploy/k8s) |
| Messaging | RabbitMQ / Kafka / similar | **RabbitMQ** |
| Database | PostgreSQL + Redis | **PostgreSQL + Redis** |
| Monitoring | Prometheus + Grafana / ELK | **Prometheus + Grafana** |
| CI/CD | GitHub Actions | **GitHub Actions** |

## Deliverables

| Deliverable | Location |
| ----------- | -------- |
| Architecture documentation | [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md), [diagrams](./architecture/), [ADRs](./adr/) |
| Database creation script | [`migrations/0001_init.up.sql`](../migrations/0001_init.up.sql) |
| GitHub link | this repository (`github.com/aniusch/projeto-fiapx`) |
| Demo video (≤10 min) | script in [`docs/demo-script.md`](./demo-script.md) |
