# Runtime flow — upload to result

The end-to-end path of one video, showing why the upload returns immediately
(HTTP 202) while processing happens asynchronously on a worker.

```mermaid
sequenceDiagram
    actor U as User
    participant G as Gateway
    participant S as Object storage (S3/MinIO)
    participant DB as Postgres
    participant Q as RabbitMQ
    participant W as Worker
    participant N as Notifier
    participant M as Email

    U->>G: POST /videos (JWT, video file)
    G->>S: put source object
    G->>DB: INSERT video (status=PENDING)
    G->>Q: publish job {video_id, source_key, email}
    G-->>U: 202 Accepted {video_id}

    Note over W,DB: The worker owns no DB. It only emits events;<br/>the gateway (single writer) applies them.

    Q->>W: deliver job
    W->>Q: publish video.processing {video_id}
    Q->>G: deliver status event
    G->>DB: UPDATE status=PROCESSING (only if PENDING)
    W->>S: get source object
    W->>W: ffmpeg extract frames + zip

    alt processing succeeds
        W->>S: put result zip
        W->>Q: publish video.done {video_id, zip_key, frame_count}
        W-->>Q: ack
        Q->>G: deliver status event
        G->>DB: UPDATE status=DONE, zip_key, frame_count
    else processing fails
        W->>Q: publish video.failed {video_id, email, reason}
        W-->>Q: ack (job removed; failure reported)
        Q->>G: deliver status event
        G->>DB: UPDATE status=FAILED, error_message
        Q->>N: deliver failure event
        N->>M: send failure email
        M-->>U: "your video could not be processed"
    end

    U->>G: GET /videos (JWT)
    G->>DB: SELECT videos WHERE user_id ORDER BY created_at DESC
    G-->>U: 200 [{video_id, status, download_url?}]
```

Notes:
- The gateway never blocks on ffmpeg — it enqueues and returns. Spikes are
  absorbed by the durable queue ([ADR-0004](../adr/0004-asynchronous-processing-rabbitmq.md)).
- Delivery is at-least-once, so the flow is idempotent: reprocessing the same
  `video_id` overwrites the deterministic result key, and the gateway applies
  status events idempotently (`MarkProcessing` only advances a still-`PENDING`
  row, so a redelivered event can't regress a terminal state).
- Only the **gateway** touches Postgres. The worker is stateless and reports
  status purely through events — see
  [ADR-0011](../adr/0011-database-per-service-single-writer.md).
