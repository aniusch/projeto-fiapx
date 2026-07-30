# Architecture Decision Records

This log captures the significant architectural decisions on the FIAP X project,
each as an immutable, numbered record. The format follows
[Michael Nygard's ADR style](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions).

**Why ADRs?** They document *why* a decision was made — the context and the
alternatives rejected — not just *what* the code does. Six months later, that
"why" is the part nobody remembers.

**Rules**
- Records are immutable once accepted. To change a decision, add a new ADR that
  supersedes the old one (and mark the old one `Superseded by ADR-NNNN`).
- One decision per record. Number sequentially.
- Copy [`template.md`](./template.md) to start a new one.

## Index

| ADR | Title | Status |
| --- | ----- | ------ |
| [0001](./0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](./0002-microservice-decomposition.md) | Decompose into gateway/worker/notifier services | Accepted |
| [0003](./0003-language-and-runtime-go.md) | Implement services in Go | Accepted |
| [0004](./0004-asynchronous-processing-rabbitmq.md) | Asynchronous processing via RabbitMQ | Accepted |
| [0005](./0005-object-storage-s3-minio.md) | Store media in S3-compatible object storage | Accepted |
| [0006](./0006-datastores-postgres-redis.md) | Postgres for state, Redis for cache | Accepted |
| [0007](./0007-deployment-compose-and-kubernetes.md) | Ship both Docker Compose and Kubernetes | Accepted |
| [0008](./0008-observability-prometheus-grafana.md) | Observability with Prometheus and Grafana | Accepted |
| [0009](./0009-secrets-external-secrets-operator.md) | Secrets via External Secrets Operator (node-role auth) | Accepted |
| [0010](./0010-continuous-deployment-keel-and-manual-apply.md) | Continuous deployment via Keel + a manual apply workflow | Accepted |
