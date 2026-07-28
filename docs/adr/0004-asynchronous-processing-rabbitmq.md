# ADR-0004: Asynchronous processing via RabbitMQ

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

Two hard requirements drive this decision: the system must **not lose a request
during spikes**, and it must **process many videos concurrently**. Processing is
slow and bursty, while uploads arrive unpredictably. We need to decouple accepting
work from doing work, buffer bursts durably, distribute jobs across workers, and
retry failures.

## Decision

We will use **RabbitMQ** as the message broker. The gateway publishes a job
message per upload to a durable queue; workers consume competitively (one job per
worker at a time via prefetch). We will use:

- a **durable** job queue with **persistent** messages, and manual acks so a job
  is only removed after it succeeds;
- a **dead-letter queue (DLQ)** for messages that exhaust their retries, so
  nothing is silently lost;
- a separate **events** exchange the worker publishes failures to, which the
  notifier consumes.

## Consequences

- Uploads return immediately (HTTP 202) after enqueuing; the broker absorbs
  spikes durably, satisfying "never lose a request".
- Throughput scales by adding worker replicas — competing consumers spread the
  load automatically.
- Manual acks + DLQ give at-least-once delivery and a safety net for poison
  messages. Workers must therefore be **idempotent** (a redelivered job must not
  corrupt state) — accepted as a design constraint.
- Cost: operating a broker, and reasoning about delivery semantics and retries.

## Alternatives considered

- **Apache Kafka.** Excellent for very high-throughput event streaming and
  replay, but heavier to operate and overkill for a work-queue of this size;
  RabbitMQ's per-message ack/DLQ model fits task distribution more directly.
- **Database-backed queue (poll a `jobs` table).** Fewer moving parts, but
  re-implements broker features (visibility, fair dispatch, DLQ) poorly and
  couples throughput to database load.
- **Synchronous processing (status quo).** Fails both the spike and concurrency
  requirements outright.
