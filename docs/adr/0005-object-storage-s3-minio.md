# ADR-0005: Store media in S3-compatible object storage

- **Status:** Accepted
- **Date:** 2026-07-28

## Context

The base project writes uploads and result zips to the **local filesystem**
(`uploads/`, `outputs/`). That breaks the moment there is more than one replica:
the worker that processed a video and the gateway that serves its download may be
different pods on different nodes with different disks. We need durable,
shared, horizontally-scalable storage for large binary blobs (source videos and
frame archives), and we intend to deploy to AWS.

## Decision

We will store all media in **S3-compatible object storage**, addressed by object
key. In development we run **MinIO** (a self-hosted S3 implementation) via Docker
Compose; in production we use **Amazon S3**. The same S3 client and code path
serve both — only endpoint and credentials change.

The database stores only the object *keys* (`source_key`, `zip_key`), never the
bytes.

## Consequences

- Any gateway or worker replica can read/write the same objects — a prerequisite
  for scaling either tier.
- Local disk becomes disposable; pods are stateless and can be rescheduled freely.
- Dev/prod parity: developing against MinIO exercises the exact S3 API used in
  production.
- Large files stream to/from object storage instead of occupying pod disk.
- Cost: an S3 client dependency and managing credentials/bucket policy.

## Alternatives considered

- **Local filesystem (status quo).** Incompatible with multiple replicas.
- **A shared network volume (NFS/EFS/RWX PVC).** Works, but is operationally
  clunky, less cloud-native, and does not match the intended S3 deployment.
- **Storing blobs in Postgres (bytea/large objects).** Bloats the database,
  hurts backup/restore, and couples storage scaling to the database.
