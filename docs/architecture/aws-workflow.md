# Workflow — runtime and CI/CD

The two flows that move through the [AWS topology](./aws-topology.md): the runtime
request/processing path, and the CI/CD + secrets-sync path.

## Runtime flow

Numbered from the upload; the failure branch (12) is dashed. Uploads return 202
immediately; processing happens asynchronously on the worker.

```mermaid
flowchart LR
  user["User"]
  gw["gateway"]
  s3[("S3")]
  rds[("RDS")]
  mq{{"RabbitMQ"}}
  wk["worker<br/>ffmpeg"]
  nt["notifier"]
  mp["Mailpit"]

  user -->|"1 POST /videos (JWT)"| gw
  gw -->|"2 put source"| s3
  gw -->|"3 INSERT PENDING"| rds
  gw -->|"4 publish job"| mq
  gw -->|"5 202 Accepted"| user
  mq -->|"6 deliver"| wk
  wk -->|"7 PROCESSING"| rds
  wk -->|"8 get source"| s3
  wk -->|"9 ffmpeg + zip"| wk
  wk -->|"10 put zip"| s3
  wk -->|"11 DONE"| rds
  wk -.->|"12 failure event"| mq
  mq -.->|"12 deliver"| nt
  nt -.->|"12 email"| mp
  user -->|"13 GET status / download (presigned S3)"| gw
```

## CI/CD & secrets flow

Routine pushes deploy themselves: CI publishes images and Keel rolls them out —
no AWS credentials in the loop. Secrets are synced from AWS Secrets Manager by the
External Secrets Operator.

```mermaid
flowchart LR
  dev["Developer"]
  gh["GitHub + Actions<br/>test, build, Sonar"]
  ghcr[("GHCR :latest")]
  keel["Keel<br/>in-cluster"]
  pods["gateway / worker / notifier"]
  tf["Terraform"]
  sm[("Secrets Manager<br/>fiapx/app")]
  eso["External Secrets Operator"]
  sec["k8s Secret<br/>fiapx-secrets"]

  dev -->|"A push to main"| gh
  gh -->|"B build & push images"| ghcr
  keel -->|"C poll :latest"| ghcr
  keel -->|"D rolling update"| pods
  tf -->|"E write secret"| sm
  eso -->|"F read (node role via IMDS)"| sm
  eso -->|"G create / sync"| sec
  sec -->|"H envFrom"| pods
```
