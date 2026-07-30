# Architecture

Living architecture documentation for FIAP X. Diagrams are written in
[Mermaid](https://mermaid.js.org/) so they are diffable, reviewable, and render
directly on GitHub. They evolve alongside the code, phase by phase.

| Document | What it shows |
| -------- | ------------- |
| [C4 — System Context](./c4-context.md) | The system as a black box: who uses it and what it talks to. |
| [C4 — Containers](./c4-container.md) | The deployable services/stores inside the system and how they interact. |
| [Data model (ERD)](./data-model.md) | The Postgres schema: entities, columns, relationships. |
| [Runtime flow](./runtime-upload-flow.md) | Sequence of an upload from request to processed result and notification. |
| [Deployment topology](./deployment-topology.md) | How it runs under Docker Compose (dev) and Kubernetes (prod). |
| [AWS/EKS cloud topology](./aws-topology.md) | Detailed AWS topology: VPC, EKS namespaces, RDS/S3/Secrets Manager, GHCR, node-role auth. |
| [Workflow (runtime + CI/CD)](./aws-workflow.md) | The runtime request/processing flow and the CI/CD + secrets-sync flow. |

See also the [Architecture Decision Records](../adr/) for the *why* behind these.

## C4 model, briefly

The [C4 model](https://c4model.com/) describes software at four zoom levels:
**Context** (systems and people), **Container** (deployable units), **Component**
(major parts inside a container), and **Code**. We maintain the top two levels
here; component-level detail lives near the code it describes.
