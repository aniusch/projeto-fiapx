# ADR-0003: Implement services in Go

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

We need a language for all three services. The base project is already written in
Go. The workload is I/O- and CPU-bound (shelling out to ffmpeg, streaming large
files, consuming queues concurrently), and we want small, fast-starting container
images suitable for horizontal scaling.

## Decision

We will implement all services in Go, as a single module with one binary per
service under `cmd/`. Shared code lives in `internal/`.

## Consequences

- The base project's ffmpeg + zip logic ports over directly into the worker.
- Goroutines and channels make concurrent queue consumption and graceful
  shutdown straightforward.
- Statically linked binaries yield tiny, fast-starting images — good for
  scale-out and quick pod startup on Kubernetes.
- A single module keeps shared types (domain, config) trivially reusable and the
  build simple; the trade-off is that services are versioned together rather than
  independently. Acceptable at this scale.
- Cost: the team is more familiar with TypeScript; there is a learning curve on
  Go idioms (explicit error handling, pointers vs values, interfaces). This is a
  deliberate, accepted cost — skill growth is a goal of the project.

## Alternatives considered

- **Polyglot (e.g. Node/TypeScript gateway + Go worker).** Demonstrates service
  independence but doubles the toolchain, CI, and container-base maintenance for
  no functional gain here.
- **Separate Go module per service.** True independent versioning, but adds
  `go.work`/replace-directive complexity and harder shared-code and Docker builds
  with no benefit at this size.
