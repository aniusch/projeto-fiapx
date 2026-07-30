# C4 — Containers

The deployable units inside FIAP X and how they collaborate. The **worker** is
the only CPU-heavy tier and scales out horizontally; the **gateway** only
validates, stores, and enqueues, so uploads return quickly.

To keep the services as independent architectural quanta, **only the gateway
owns a database**. The worker is stateless: it reports each transition
(processing / done / failed) as an event, and the gateway — the sole writer of
the `videos` table — applies them (single-writer + event-carried state transfer).
See [ADR-0011](../adr/0011-database-per-service-single-writer.md).

```mermaid
C4Container
    title Container Diagram — FIAP X

    Person(user, "User", "Uploads and monitors videos")
    System_Ext(mail, "Email provider", "SMTP")

    Container_Boundary(fiapx, "FIAP X") {
        Container(gateway, "Gateway", "Go, Gin", "Auth (JWT), uploads, status listing, downloads. Publishes jobs; does no heavy processing.")
        Container(worker, "Worker", "Go, ffmpeg", "Stateless: consumes jobs, extracts frames, zips, uploads results, and emits status events. Owns no database. Scales horizontally.")
        Container(notifier, "Notifier", "Go", "Consumes failure events and emails the affected user.")
        ContainerDb(pg, "PostgreSQL", "Postgres 16", "Users and video job state (written only by the gateway)")
        ContainerDb(redis, "Redis", "Redis 7", "Token cache and rate-limit counters")
        ContainerQueue(mq, "RabbitMQ", "AMQP 0-9-1", "Durable job queue and failure events, with a dead-letter queue")
        ContainerDb(s3, "Object storage", "S3 / MinIO", "Source videos and result ZIP archives")
    }

    Rel(user, gateway, "Uses", "HTTPS / JSON")

    Rel(gateway, pg, "Reads/writes users & videos", "SQL")
    Rel(gateway, redis, "Caches tokens, counts requests", "RESP")
    Rel(gateway, s3, "Stores source, streams downloads", "S3 API")
    Rel(gateway, mq, "Publishes processing jobs", "AMQP")

    Rel(mq, worker, "Delivers jobs", "AMQP")
    Rel(worker, s3, "Reads source, writes ZIP", "S3 API")
    Rel(worker, mq, "Publishes status & failure events", "AMQP")

    Rel(mq, gateway, "Delivers status events", "AMQP")
    Rel(mq, notifier, "Delivers failure events", "AMQP")
    Rel(notifier, mail, "Sends email", "SMTP")

    UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
```
