# Data model

The Postgres schema (see [`migrations/0001_init.up.sql`](../../migrations/0001_init.up.sql)).
A user owns many videos; deleting a user cascades to their videos. `status` is a
Postgres ENUM, and `updated_at` is maintained by a trigger.

```mermaid
erDiagram
    users ||--o{ videos : owns

    users {
        uuid        id           PK "gen_random_uuid()"
        citext      email        UK "case-insensitive, unique"
        text        password_hash   "bcrypt hash"
        timestamptz created_at
    }

    videos {
        uuid         id            PK "gen_random_uuid()"
        uuid         user_id       FK "-> users.id, ON DELETE CASCADE"
        text         original_name
        video_status status           "PENDING|PROCESSING|DONE|FAILED"
        text         source_key       "object key of uploaded video"
        text         zip_key          "object key of result archive"
        int          frame_count
        text         error_message    "set when status = FAILED"
        timestamptz  created_at
        timestamptz  updated_at       "maintained by trigger"
    }
```

## Status lifecycle

```mermaid
stateDiagram-v2
    [*] --> PENDING: gateway creates job
    PENDING --> PROCESSING: worker picks up
    PROCESSING --> DONE: frames zipped & uploaded
    PROCESSING --> FAILED: error (worker publishes event)
    DONE --> [*]
    FAILED --> [*]
```
