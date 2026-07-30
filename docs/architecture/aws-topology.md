# AWS / EKS cloud topology

The deployed topology on Amazon EKS (as provisioned by
[`infra/terraform`](../../infra/terraform) and
[`infra/k8s/overlays/aws`](../../infra/k8s/overlays/aws)). Dashed edges are image
pulls / secret injection; everything else is a runtime dependency.

> **Auth:** there is no IRSA in AWS Academy Learner Lab, so pods (and the External
> Secrets Operator) authenticate to S3 and Secrets Manager via the **EKS node role
> (`LabRole`) over IMDS** (node launch template hop-limit 2). No NAT gateway —
> nodes sit in public subnets.

```mermaid
flowchart TB
  client["Client / browser"]
  gha["GitHub Actions<br/>CI: test, build, push, Sonar"]
  ghcr[("GHCR ghcr.io<br/>public images")]

  subgraph aws["AWS account · us-east-1"]
    subgraph vpc["VPC 10.0.0.0/16 · public subnets · 2 AZs · no NAT"]
      subgraph eks["EKS cluster: fiapx · k8s 1.30<br/>nodes 2x t3.medium · IMDSv2 hop-limit 2"]
        subgraph nsapp["namespace: fiapx"]
          gw["gateway x2<br/>Service + Ingress"]
          wk["worker x2 + HPA 2-6<br/>ffmpeg"]
          nt["notifier"]
          rd["redis"]
          mq{{"rabbitmq"}}
          mp["mailpit"]
        end
        subgraph nseso["namespace: external-secrets"]
          eso["External Secrets Operator"]
          sec["k8s Secret: fiapx-secrets"]
        end
        subgraph nskeel["namespace: keel"]
          keel["Keel"]
        end
      end
    end
    rds[("RDS PostgreSQL 16.9<br/>private")]
    s3[("S3: fiapx-videos<br/>encrypted, presigned URLs")]
    sm[("Secrets Manager<br/>fiapx/app")]
  end

  client -->|HTTPS| gw
  gw -->|"SQL 5432"| rds
  gw -->|"S3 API (node role)"| s3
  gw -->|"cache / rate-limit"| rd
  gw -->|"publish jobs, AMQP"| mq
  mq -->|"deliver jobs"| wk
  wk -->|"get source / put zip (node role)"| s3
  wk -->|"status & failure events"| mq
  mq -->|"deliver status events"| gw
  mq -->|"deliver events"| nt
  nt -->|SMTP| mp
  eso -->|"read fiapx/app (node role via IMDS)"| sm
  eso -->|sync| sec
  sec -.->|envFrom| gw
  sec -.->|envFrom| wk
  sec -.->|envFrom| nt
  keel -->|"poll :latest"| ghcr
  gha -->|"build & push"| ghcr
  ghcr -.->|"image pull"| gw
  ghcr -.->|"image pull"| wk
  ghcr -.->|"image pull"| nt
```
