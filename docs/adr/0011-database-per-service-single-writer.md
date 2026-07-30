# ADR-0011: One database per service — gateway as the single writer

- **Status:** Accepted
- **Date:** 2026-07-30

## Context

The system is decomposed into three services (gateway, worker, notifier). An
architectural quantum — an independently deployable unit with high functional
cohesion — must include the data it operates on; a **shared database** couples
services statically, so two services reading and writing the same tables are, in
practice, a single quantum no matter how they are deployed.

The earlier design had the **gateway and the worker share one Postgres
database**: the gateway inserted the `videos` row and listed status, while the
worker reached into the *same* table to flip status (`PROCESSING`/`DONE`/
`FAILED`) and read the owner's email from `users`. That is exactly the static
coupling above — a change to the `videos` schema forced both services to change
and redeploy together — so gateway and worker were not really separate quanta.

The two services genuinely operate on **one bounded context** (the video
lifecycle): the gateway is the producer, the worker is the consumer of the work.

## Decision

We keep **exactly one owner per database, and only the gateway owns one.** The
worker and notifier become **stateless** — they hold no database handle at all.

- The gateway is the **single writer** of the `users` and `videos` tables.
- The worker reports every transition as an event on the `videos.events` topic
  exchange: `video.processing` when it picks up a job, `video.done` (with the
  result key + frame count) on success, `video.failed` (with the friendly
  message + the owner's email) on failure.
- The gateway runs a small **status consumer** that binds to those routing keys
  and applies each event to the `videos` table. This is **event-carried state
  transfer** with a **single writer**.
- The `video.failed` event fans out on the same exchange to *both* the gateway
  (mark the row `FAILED`) and the notifier (email the user).
- The owner's email travels **inside the job message** (published by the
  gateway, which owns `users`), so the worker never needs to read `users`.

Idempotency, since delivery is at-least-once: `MarkProcessing` only advances a
row that is still `PENDING`, so a redelivered "processing" event can't regress a
terminal row; the result object uses a deterministic key so re-processing
overwrites rather than duplicates; and the terminal `done`/`failed` update is the
authoritative record, so the worker requeues if it cannot publish it.

On AWS this maps onto the **single RDS instance** with a logical database; the
point is ownership (only the gateway connects), not a separate server per service.

## Consequences

- **Three independent quanta.** The worker and notifier own no data, so a
  `videos` schema change touches only the gateway. Each service can be scaled,
  deployed, and reasoned about on its own.
- **One source of truth for status.** Status lives in exactly one table, written
  by exactly one service — no dual-write and no cross-service SQL.
- **The worker got simpler**: no Postgres pool, no migrations dependency, no
  startup coupling to the database (reflected in compose `depends_on`).
- **Trade-off — eventual consistency.** A status change is visible only after the
  event is delivered and applied, a small window versus the old synchronous
  write. For a "listing of video status" this is entirely acceptable.
- **Trade-off — a lost `processing` event** leaves a row on `PENDING` a bit
  longer; the authoritative terminal event still corrects it. Only the terminal
  event is treated as must-deliver (it requeues on publish failure).
- Object storage (S3/MinIO) remains shared between gateway and worker. A blob
  store keyed by opaque IDs has no schema to couple on, so we treat it as a
  shared resource rather than owned data.

## Alternatives considered

- **Physical database-per-service (worker owns `videos`).** Give the worker its
  own database and have the gateway keep a status *projection* synced via events.
  This shows multiple databases but duplicates the video record across two stores
  and adds CQRS-style sync for no real benefit here — the gateway already needs
  the status to serve the listing, so making it the owner is simpler.
- **Keep the shared database (status quo).** Pragmatic and consistent, but it
  fuses gateway + worker into one quantum — the precise coupling this project is
  meant to demonstrate removing.
- **Worker keeps writing, but to its own DB, gateway reads it.** Reintroduces
  cross-service reads (the gateway reaching into the worker's DB for the
  listing), i.e. the same shared-database coupling in the other direction.
