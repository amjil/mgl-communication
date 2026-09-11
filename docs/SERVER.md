# Server

Go Push Gateway: `mgl-push-server/`.

## Running

See [DEPLOYMENT.md](DEPLOYMENT.md) for environment variables.

```bash
cd mgl-push-server
go build -o bin/mgl-push ./cmd/mgl-push
./bin/mgl-push
```

Development:

```bash
# repo root
docker compose up --build
```

## Modules

| Package | Responsibility |
|---------|----------------|
| `cmd/mgl-push` | Entrypoint, provider registration |
| `internal/api` | HTTP handlers |
| `internal/service` | Device / Message / Incoming Call |
| `internal/repository` | PostgreSQL |
| `internal/queue` | Worker |
| `internal/retry` | Backoff (1s → 5s → 30s → 2m → 10m) |
| `internal/provider` | Interface + Registry + Named (e.g. `apns_voip`) |
| `internal/provider/{fcm,apns,huawei,xiaomi,oppo,vivo,noop}` | Vendor adapters |
| `internal/auth` | Bearer → app_id |
| `internal/metrics` | Prometheus |
| `internal/idgen` | ULID |
| `internal/domain` | Message / Device / Delivery / MessageType |
| `internal/config` | Env |

## Message pipeline

```text
API
 → MessageService (validate, Idempotency, write message + deliveries)
 → status=queued
 → Worker claim (FOR UPDATE SKIP LOCKED)
 → Provider.Send
 → accepted | retrying | failed (+ invalidate token)
 → message completed / failed when all deliveries terminal
```

Incoming Call / Call signal use the same pipeline with `type` + high priority + short TTL.

## Migrations

| File | Content |
|------|---------|
| `migrations/001_init.sql` | devices / push_messages / push_deliveries / idempotency_keys |
| `migrations/002_v21_events.sql` | Upgrade existing DBs: `type` column, idempotency, delivery accepted_at/failed_at |

Docker Compose mounts both into `docker-entrypoint-initdb.d/` (**first empty volume only**). For an existing volume, run `002` manually.

## Idempotency

Message endpoints read `Idempotency-Key`.  
If Incoming Call omits the key, the default is `incoming:{call_id}:{user_or_installation}`.

## Graceful shutdown

SIGTERM/SIGINT → stop HTTP → stop worker → close providers → close DB.

## Health

- `GET /health` — process liveness
- `GET /ready` — Postgres ping + at least one provider + worker alive
- `GET /metrics` — Prometheus (includes `incoming_call_push_total`, etc.)
