# Kubernetes deployment

Manifests for running FIAP X on Kubernetes, wired together with
[Kustomize](https://kustomize.io/). Everything lands in the `fiapx` namespace.

The backing services (Postgres, Redis, RabbitMQ, MinIO, Mailpit) are deployed
in-cluster for a self-contained demo. In a real environment you would point the
services at managed offerings (RDS, ElastiCache, Amazon MQ, S3, SES) by editing
`config.yaml` and removing the corresponding in-cluster manifests.

## What's here

| File | Resources |
| ---- | --------- |
| `namespace.yaml` | the `fiapx` namespace |
| `config.yaml` | `ConfigMap` (non-secret env) + `Secret` (credentials) |
| `postgres.yaml` `redis.yaml` `rabbitmq.yaml` `minio.yaml` `mailpit.yaml` | backing services + `Service`s (+ PVCs) |
| `migrate-job.yaml` | one-shot `Job` that applies the SQL migrations |
| `gateway.yaml` | `Deployment` (2 replicas) + `Service` + `Ingress` |
| `worker.yaml` | `Deployment` (2 replicas) + `Service` + **HorizontalPodAutoscaler** (2→6 on CPU) |
| `notifier.yaml` | `Deployment` + `Service` |
| `kustomization.yaml` | ties it together; generates the migrations ConfigMap; pins image tags |

Probes: the gateway uses `/healthz` (liveness) and `/readyz` (readiness — pings
Postgres + Redis); the worker and notifier use `/healthz` on their metrics port.

## Deploy (local minikube)

The app images are built by the compose Dockerfiles. Tag them under the names the
manifests expect and load them into the cluster:

```bash
docker tag projeto-fiapx-gateway:latest  fiapx/gateway:latest
docker tag projeto-fiapx-worker:latest   fiapx/worker:latest
docker tag projeto-fiapx-notifier:latest fiapx/notifier:latest
minikube image load fiapx/gateway:latest fiapx/worker:latest fiapx/notifier:latest

minikube addons enable metrics-server   # required for the worker HPA
```

Apply. Kustomize reads the migration SQL from `../../migrations`, which needs the
relaxed load restrictor, so build-and-pipe rather than `apply -k`:

```bash
kubectl kustomize --load-restrictor LoadRestrictionsNone deploy/k8s | kubectl apply -f -
kubectl -n fiapx get pods -w
```

> The gateway/worker/notifier fail fast if a dependency isn't up yet, so they may
> restart a few times on first boot until Postgres and RabbitMQ are ready — the
> Kubernetes restart policy recovers them automatically.

## Try it

```bash
kubectl -n fiapx port-forward svc/gateway 8080:8080
# then, in another shell, use the API as usual (see the top-level README)

kubectl -n fiapx get hpa worker          # watch autoscaling
kubectl -n fiapx top pods                # live CPU/memory
```

## Using published (GHCR) images

CI publishes images to `ghcr.io/aniusch/projeto-fiapx-{gateway,worker,notifier}`.
To pull those instead of loading local images, update the `images:` block in
`kustomization.yaml`, e.g.:

```yaml
images:
  - name: fiapx/gateway
    newName: ghcr.io/aniusch/projeto-fiapx-gateway
    newTag: latest
  # ...worker, notifier likewise
```

## Tear down

```bash
kubectl delete namespace fiapx
```
