# Postman collection

End-to-end API tests for the gateway: 25 requests, 38 assertions.

| Folder | What it proves |
|---|---|
| 0. Health | `/healthz` liveness, `/readyz` checks Postgres + Redis |
| 1. Auth | register (201), duplicate (409), weak password (400), login (200), wrong password (401, generic message) |
| 2. Validation & security | no/bogus token (401), missing file / bad extension (400), bad id (400), unknown id (404) |
| 3. Happy path | upload → 202 `PENDING` → polls until `DONE` → listing → 302 presigned URL → real zip |
| 4. Failure path | corrupt upload → `FAILED` with friendly message → download 409 → e-mail in Mailpit |
| 5. Isolation | a second user can't see the first user's video (404) |

## Run

```bash
make up        # start the stack
make postman   # runs with newman (via npx)
```

In the Postman app: import both JSON files, select the **FIAP X — local** environment,
set *Settings → General → Working directory* to this `postman/` folder (so uploads
find `fixtures/`), then run the collection with the Collection Runner, in order.

Each run registers a fresh user, so the collection can be re-run any number of times.
