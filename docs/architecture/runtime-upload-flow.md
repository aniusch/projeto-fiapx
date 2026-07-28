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
    G->>Q: publish job {video_id, source_key}
    G-->>U: 202 Accepted {video_id}

    Q->>W: deliver job
    W->>DB: UPDATE status=PROCESSING
    W->>S: get source object
    W->>W: ffmpeg extract frames + zip

    alt processing succeeds
        W->>S: put result zip
        W->>DB: UPDATE status=DONE, zip_key, frame_count
        W-->>Q: ack
    else processing fails
        W->>DB: UPDATE status=FAILED, error_message
        W->>Q: publish failure event {video_id, user_id, reason}
        W-->>Q: ack (job removed; failure recorded)
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
- Delivery is at-least-once, so the worker must be idempotent: reprocessing the
  same `video_id` must converge to the same result.
