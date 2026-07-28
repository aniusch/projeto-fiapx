# ADR-0002: Decompose into gateway/worker/notifier services

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

The base project is a monolith that accepts an upload and runs ffmpeg
**synchronously** inside the HTTP handler. This couples request handling to heavy
CPU work: one large video ties up a request, spikes cannot be absorbed, and the
web tier and processing tier cannot scale independently. The requirements demand
processing many videos at once, surviving spikes without losing requests, and a
horizontally scalable architecture.

## Decision

We will split the system into three independently deployable services:

- **gateway** — public HTTP API: authentication, uploads, status listing,
  downloads. Does no heavy processing; it validates, stores, and enqueues.
- **worker** — consumes jobs from the broker, runs ffmpeg, zips frames, uploads
  results, and updates job state. This is the only CPU-heavy tier and scales out
  by running more replicas.
- **notifier** — consumes failure events and notifies the affected user.

The services share nothing but the datastores and the broker; they communicate
asynchronously (see [ADR-0004](./0004-asynchronous-processing-rabbitmq.md)).

## Consequences

- The processing tier scales independently of the API tier — the key to
  "process many videos at once" and absorbing spikes.
- A slow or crashing worker no longer blocks or crashes the API.
- Separation of concerns keeps each codebase small and testable.
- Cost: more operational surface (three deployables), and the need for
  cross-service contracts (message schemas) and end-to-end tracing.

## Alternatives considered

- **Modular monolith with internal goroutine workers.** Simpler to deploy, but
  the web and processing tiers still scale together and share a failure domain;
  weaker fit for the "microservices" requirement.
- **Two services (merge notifier into the gateway).** Fewer moving parts, but
  notifications would then be coupled to the API's deployment and scaling, and
  the failure-handling path would be less clearly isolated.
