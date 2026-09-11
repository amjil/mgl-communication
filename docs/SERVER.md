# Server

Go Realtime Communication Server: `mgl-realtime-server/`.

Details: [`REALTIME.md`](REALTIME.md)

## Running

Environment variables: [DEPLOYMENT.md](DEPLOYMENT.md).

```bash
cd mgl-realtime-server
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
| `cmd/mgl-push` | Entrypoint; provider / realtime wiring |
| `internal/api` | HTTP: devices, messages, calls, presence, `/ws` |
| `internal/auth` | Service token + user JWT + Call Token |
| `internal/service` | Device / Message (Push) |
| `internal/repository` | PostgreSQL |
| `internal/queue` | Push Worker |
| `internal/retry` | Backoff (1s → 5s → 30s → 2m → 10m) |
| `internal/provider` | Push Provider Registry + Named (e.g. `apns_voip`) |
| `internal/provider/{fcm,apns,huawei,xiaomi,oppo,vivo,noop}` | Vendor adapters |
| `internal/events` | In-process Event Bus |
| `internal/presence` | Online presence (memory) |
| `internal/call` | Call Runtime + Orchestrator (Phoenix / SFU / ICE) |
| `internal/incomingcall` | Call → Push bridge (ring / stop-ring / cancel) |
| `internal/signaling/protocol` | WS message types and Envelope |
| `internal/signaling/connection` | Single connection |
| `internal/signaling/websocket` | Hub: auth, relay, rate limit, multi-device |
| `internal/sfu` + `sfu/livekit` | Room / Join Token (stub or LiveKit JWT) |
| `internal/phoenix` | `CanUserCall` HTTP client (empty config = allow-all) |
| `internal/ratelimit` | Fixed-window rate limiter |
| `internal/metrics` | Prometheus |
| `internal/idgen` | ULID |
| `internal/domain` | Message / Device / Delivery / errors |
| `internal/config` | Env |

## Push message pipeline

```text
API
 → MessageService (validate, Idempotency, write message + deliveries)
 → status=queued
 → Worker claim (FOR UPDATE SKIP LOCKED)
 → Provider.Send
 → accepted | retrying | failed (+ invalidate token)
 → message completed / failed when all deliveries terminal
```

Incoming Call / Call signal use the same pipeline (`type` + high priority + short TTL).

## Call / Signaling pipeline

```text
WS authenticate (JWT) | HTTP Bearer (service|JWT)
 → Call Orchestrator
      → Phoenix CanUserCall (optional)
      → Call Service (memory)
      → group: SFU EnsureRoom
 → Incoming Call Push (ringing)
 → accept → stop_ringing (WS + Push)
 → webrtc.* relay (P2P) | SFU token (group)
 → hangup / timeout / fail
```

## Migrations

| File | Content |
|------|---------|
| `migrations/001_init.sql` | devices / push_messages / push_deliveries / idempotency_keys |
| `migrations/002_v21_events.sql` | `type`, idempotency, delivery accepted_at/failed_at |

Docker Compose mounts into `docker-entrypoint-initdb.d/` (**empty volume, first create only**). Existing volumes must run `002` manually.

Call / Presence have **no dedicated tables** (single-node memory).

## Idempotency

Message endpoints read `Idempotency-Key`.  
Incoming Call default: `incoming:{call_id}:{user_or_installation}`.  
Stop-ringing Push: `stop_ring:{call_id}:{user_id}`.

## Graceful shutdown

SIGTERM/SIGINT → stop HTTP → stop worker → close providers → close DB.

## Health

- `GET /health` — liveness
- `GET /ready` — Postgres + providers + worker (+ websocket marker)
- `GET /metrics` — Prometheus (includes `websocket_*`, `call_*`, `push_*`)

## Implementation status

| Phase (spec §42) | Scope | Status |
|------------------|-------|--------|
| 1 | JWT, Device, APNs/FCM, WS, Presence | ✓ |
| 2 | Call, Incoming Call, 1:1, ICE | ✓ |
| 3 | SFU integration, Group | ✓ (LiveKit token / stub) |
| 4 | Reconnect, Recovery, Multi-device, rate limits | ✓ (baseline) |
| 5 | Redis, horizontal scaling | ○ not started |
