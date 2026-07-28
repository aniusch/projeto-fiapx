# ADR-0008: Observability with Prometheus and Grafana

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

A distributed system needs to be observable: we must be able to see request
rates and latencies, how many videos are processed and how many fail, how long
processing takes, and whether notifications go out. The requirements call for a
monitoring stack, suggesting Prometheus + Grafana or the ELK stack.

## Decision

We will instrument every service with **Prometheus** metrics and visualize them
in **Grafana**.

- Each service exposes a `/metrics` endpoint. The gateway serves it on its HTTP
  port; the worker and notifier run a small dedicated metrics HTTP server.
- Metrics use a `fiapx_<service>_<name>` naming convention: gateway HTTP request
  count/latency, worker jobs-processed (by outcome), job-duration histogram and
  in-flight gauge, and notifier notifications (by outcome).
- Prometheus scrapes all services on the compose network; a provisioned Grafana
  datasource and starter dashboard ship in the repo under `infra/`.

To make in-network scraping straightforward, the gateway and notifier are now
containerized alongside the worker.

## Consequences

- Golden-signal visibility (rate, errors, duration) across all services, plus
  domain metrics (videos processed/failed) that map directly to the product.
- The dashboard and datasource are provisioned as code, so `docker compose up`
  yields a working Grafana with no manual setup — good for the demo.
- Containerizing the gateway surfaced the internal-vs-public storage endpoint
  problem, solved with `S3_PUBLIC_ENDPOINT` (download URLs are signed for the
  host clients actually use). This is a genuinely useful production concern to
  have addressed early.
- Cost: two more components to run, and the discipline of instrumenting new code
  paths as they are added.

## Alternatives considered

- **ELK stack (Elasticsearch/Logstash/Kibana).** Strong for log search and
  aggregation, but heavier to operate; Prometheus's pull-based metrics model is a
  better fit for the numeric golden signals we care about here. Structured JSON
  logs (already emitted via slog) keep the door open to adding ELK later.
- **No metrics, logs only.** Simplest, but blind to latency distributions,
  throughput, and error rates — exactly what a scalable system must watch.
