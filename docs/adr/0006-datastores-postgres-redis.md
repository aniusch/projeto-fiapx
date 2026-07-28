# ADR-0006: Postgres for state, Redis for cache

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

We must persist users and the status of every video job (the base project has no
database at all — it globs a directory). We also need auth support (accounts,
credentials) and fast, ephemeral state such as token/session lookups and
rate-limit counters to protect the API during spikes.

## Decision

We will use **PostgreSQL** as the system of record for durable, relational state
(`users`, `videos`) and **Redis** as an in-memory cache for ephemeral state
(JWT/session validation cache, rate-limit counters). Schema changes are managed as
versioned SQL migrations under `migrations/`.

## Consequences

- Strong consistency and relational integrity for core data: foreign keys tie a
  video to its owner, an ENUM constrains status to valid values, and a trigger
  keeps `updated_at` honest.
- Redis keeps hot-path lookups off Postgres and provides cheap counters for rate
  limiting, helping absorb spikes.
- Versioned migrations give a reproducible schema and satisfy the "database
  creation script" deliverable.
- Cost: two datastores to run and reason about; care needed not to treat Redis as
  a source of truth (it is a cache).

## Alternatives considered

- **Postgres only.** Simpler, but session/rate-limit churn on the primary
  database adds avoidable load on the hot path.
- **A NoSQL document store (e.g. MongoDB).** The data is naturally relational
  (users own videos, status is a small closed set); we would lose foreign keys
  and typed enums for no benefit.
