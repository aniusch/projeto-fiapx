# Deployment topology

Two deployment targets ([ADR-0007](../adr/0007-deployment-compose-and-kubernetes.md)):
Docker Compose for local development/demo, Kubernetes for the scalable production
story.

## Development — Docker Compose

The whole system runs in the compose network: the three services are
containerized (the worker's image includes ffmpeg), and Prometheus + Grafana
provide monitoring. Only published ports are reachable from the host. Services
can still be run individually on the host via `go run` for fast iteration.

```mermaid
flowchart TB
    subgraph svc["services (containers)"]
        gw["gateway :8080"]
        wk["worker (ffmpeg)"]
        nt["notifier"]
    end
    subgraph infra["backing services"]
        pg[("Postgres :5432")]
        rd[("Redis :6379")]
        mq{{"RabbitMQ :5672 / UI :15672"}}
        s3[("MinIO :9000 / console :9001")]
        mp["Mailpit SMTP :1025 / UI :8025"]
        mig["migrate (one-shot)"]
    end
    subgraph mon["monitoring"]
        prom["Prometheus :9090"]
        graf["Grafana :3000"]
    end

    gw --> pg & rd & mq & s3
    wk --> pg & mq & s3
    nt --> mq & mp
    mig -. applies migrations .-> pg
    prom -. scrapes /metrics .-> gw & wk & nt
    graf --> prom
```

## Production — AWS / EKS

Deployed from the [`infra/k8s/overlays/aws`](../../infra/k8s/overlays/aws) overlay
onto EKS (provisioned by [`infra/terraform`](../../infra/terraform)). Postgres is
managed **RDS** and video storage is **S3**; Redis, RabbitMQ, and Mailpit run
in-cluster. Images are pulled from **public GHCR**. Application secrets are synced
from **AWS Secrets Manager** by the **External Secrets Operator (ESO)** into a
Kubernetes Secret. The worker's HorizontalPodAutoscaler scales the CPU-heavy tier
(2→6 on CPU).

Because AWS Academy Learner Lab forbids IRSA, both the app and ESO authenticate to
AWS with the **EKS node role (`LabRole`) via IMDS** — no static credentials in the
cluster ([ADR-0009](../adr/0009-secrets-external-secrets-operator.md)).

```mermaid
flowchart TB
    ghcr["ghcr.io (public images)"]

    subgraph eks["EKS — nodes assume LabRole via IMDS"]
        gw["gateway pods"]
        subgraph wdep["worker Deployment + HPA (2→6)"]
            wk["worker pods"]
        end
        nt["notifier pod"]
        rd[("Redis")]
        mq{{"RabbitMQ"}}
        mp["Mailpit"]
        eso["External Secrets Operator"]
        sec["Secret: fiapx-secrets"]
    end

    rds[("RDS PostgreSQL")]
    s3[("S3")]
    sm[("AWS Secrets Manager")]

    ghcr -. images .-> gw & wk & nt
    gw --> rd
    gw --> mq
    gw --> rds
    gw --> s3
    mq --> wk
    wk --> rds
    wk --> s3
    wk -. failure events .-> mq
    mq --> nt
    nt --> mp
    eso -. reads .-> sm
    eso -. creates .-> sec
    sec -. envFrom .-> gw & wk & nt
```
