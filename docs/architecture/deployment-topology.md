# Deployment topology

Two deployment targets ([ADR-0007](../adr/0007-deployment-compose-and-kubernetes.md)):
Docker Compose for local development/demo, Kubernetes for the scalable production
story.

## Development — Docker Compose

Infrastructure runs in containers; the Go services run on the host via `go run`
during development (they connect to the containers on `localhost`).

```mermaid
flowchart TB
    subgraph host["Developer host (go run)"]
        gw["gateway :8080"]
        wk["worker"]
        nt["notifier"]
    end
    subgraph net["docker compose network"]
        pg[("Postgres :5432")]
        rd[("Redis :6379")]
        mq{{"RabbitMQ :5672 / UI :15672"}}
        s3[("MinIO :9000 / console :9001")]
        mp["Mailpit SMTP :1025 / UI :8025"]
        mig["migrate (one-shot)"]
    end

    gw --> pg
    gw --> rd
    gw --> mq
    gw --> s3
    wk --> pg
    wk --> mq
    wk --> s3
    nt --> mq
    nt --> mp
    mig -. applies migrations .-> pg
```

## Production — Kubernetes

Each service is a Deployment. The worker carries a HorizontalPodAutoscaler so the
CPU-heavy tier scales on load independently of the API tier. Managed AWS services
(RDS, ElastiCache, S3, SES) can replace the in-cluster backing services.

```mermaid
flowchart TB
    ing["Ingress / Load Balancer"] --> gsvc["Service: gateway"]
    gsvc --> gw1["gateway pod"]
    gsvc --> gw2["gateway pod"]

    subgraph wdep["worker Deployment + HPA"]
        wk1["worker pod"]
        wk2["worker pod"]
        wk3["worker pod (autoscaled)"]
    end
    ndep["notifier Deployment"]

    gw1 --> mq{{"RabbitMQ"}}
    gw2 --> mq
    gw1 --> pg[("PostgreSQL / RDS")]
    gw2 --> pg
    gw1 --> rd[("Redis / ElastiCache")]
    gw2 --> rd
    gw1 --> s3[("S3")]
    gw2 --> s3

    mq --> wk1
    mq --> wk2
    mq --> wk3
    wk1 --> pg
    wk2 --> pg
    wk3 --> pg
    wk1 --> s3
    wk2 --> s3
    wk3 --> s3
    wk1 -. failure events .-> mq
    wk2 -. failure events .-> mq
    wk3 -. failure events .-> mq

    mq --> ndep
    ndep --> smtp["SMTP / SES"]
```
