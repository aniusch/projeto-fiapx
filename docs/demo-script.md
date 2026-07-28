# Demo video script (≤ 10 minutes)

A timeboxed run sheet for the presentation. Rehearse once so the timings hold.
Have the stack **already built** before recording (`docker compose build`) so you
never wait on image builds on camera.

Pre-flight (before recording):

```bash
docker compose build
docker compose up -d
# wait until healthy, then confirm:
curl -s localhost:8080/readyz
```

---

## 0:00–1:30 — The problem & the architecture (slides / docs)

- Show the base monolith ([`legacy/base-project/main.go`](../legacy/base-project/main.go)):
  ffmpeg runs **synchronously in the HTTP handler**, no auth, no persistence,
  local disk. State the goals (parallel, spike-safe, auth, status, notify).
- Show the **container diagram** ([`docs/architecture/c4-container.md`](./architecture/c4-container.md))
  and name the three services + queue + stores.
- One line on the ADRs: "every major decision is recorded in `docs/adr/`."

## 1:30–2:15 — The stack is running

```bash
docker compose ps
```

- Point out the three services + Postgres, Redis, RabbitMQ, MinIO, Mailpit,
  Prometheus, Grafana — all from one `docker compose up`.

## 2:15–4:30 — Happy path: upload → process → download

```bash
# Register and capture a token
TOKEN=$(curl -s -X POST localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@fiapx.local","password":"password123"}' | jq -r .token)

# Upload a video (returns 202 + PENDING)
curl -s -X POST localhost:8080/videos \
  -H "Authorization: Bearer $TOKEN" -F "video=@sample.mp4" | jq

# Poll the per-user status listing until DONE
curl -s localhost:8080/videos -H "Authorization: Bearer $TOKEN" | jq

# Download the frames zip (follows the presigned URL)
curl -sL "localhost:8080/videos/<ID>/download" -H "Authorization: Bearer $TOKEN" -o frames.zip
unzip -l frames.zip
```

- Narrate: upload returns **202** immediately (async); the worker picked it off
  the queue, ran ffmpeg, zipped the frames, and marked it `DONE`.

## 4:30–5:45 — Failure path: notification

```bash
# Upload an invalid file to force a failure
echo "not a video" > bad.mp4
curl -s -X POST localhost:8080/videos -H "Authorization: Bearer $TOKEN" -F "video=@bad.mp4" | jq
```

- Show the video turns `FAILED` in the listing.
- Open **Mailpit** at <http://localhost:8025> and show the failure email.

## 5:45–7:00 — Monitoring

- Open **Grafana** at <http://localhost:3000> (admin/admin) → "FIAP X — Overview".
- Point at: gateway request rate, worker jobs by outcome (done/failed), job
  duration, notifications sent. Optionally show **RabbitMQ UI** at
  <http://localhost:15672> (guest/guest) with the queues.

## 7:00–8:45 — Scalability

Either (compose):

```bash
docker compose up -d --scale worker=3
docker compose ps | grep worker
```

Or (Kubernetes — if a cluster is handy):

```bash
kubectl -n fiapx get pods
kubectl -n fiapx get hpa worker      # 2→6 replicas on CPU
```

- Narrate: the CPU-heavy worker scales independently; competing consumers on the
  durable queue spread the load, so bursts are absorbed without losing requests.

## 8:45–10:00 — Quality: tests & CI/CD

```bash
go test ./...                          # fast unit tests, no external deps
```

- Show [`.github/workflows/ci.yml`](../.github/workflows/ci.yml): lint + unit +
  integration (service containers) + build/push images to GHCR.
- Close on the [requirements traceability matrix](./requirements-traceability.md):
  every requirement met, and where.

---

### Teardown

```bash
docker compose down          # add -v to wipe data volumes
```
