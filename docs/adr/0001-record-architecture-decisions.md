# ADR-0001: Record architecture decisions

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

This project rebuilds the FIAP X video processor from a single-file prototype
into a distributed system. Many architectural choices (how to split services,
which broker, which datastores) will be made early and will constrain everything
built afterwards. The hackathon also requires architecture documentation as a
deliverable. We need a lightweight, durable way to record *why* each choice was
made so reviewers — and our future selves — can follow the reasoning.

## Decision

We will keep a log of Architecture Decision Records under `docs/adr/`, one file
per decision, using Michael Nygard's format. Records are immutable once accepted;
a decision is changed by adding a new record that supersedes the old one.

## Consequences

- Every significant decision has a traceable rationale and a list of rejected
  alternatives.
- Reviewers can read the decision log to understand the system's shape without
  reverse-engineering it from code.
- There is a small, ongoing discipline cost: a decision is not "done" until its
  ADR is written.

## Alternatives considered

- **A single design document.** Tends to rot: it is edited in place, so the
  history of *why* a decision changed is lost. ADRs are append-only by design.
- **No formal record (tribal knowledge).** Fails the documentation deliverable
  and loses context the moment a contributor moves on.
