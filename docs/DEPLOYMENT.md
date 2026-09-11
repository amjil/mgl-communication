# Deployment

## Docker Compose (dev / small scale)

Repo root:

```bash
cp .env.example .env   # fill in as needed
docker compose up --build -d
```

Services:

- `postgres:16` — init mounts `migrations/001_init.sql` + `002_v21_events.sql` + `003_dual_provider.sql`
- `mgl-communication` — default `http://localhost:8080`, `dev-token` → `net.amjil.demo`

> **Note:** init SQL runs only on **first create of an empty volume**. For existing data, run newer migrations manually (`002_v21_events.sql`, `003_dual_provider.sql`).

## Binary

```bash
cd mgl-realtime-server
go build -o bin/mgl-push ./cmd/mgl-push
export MGL_PUSH_DATABASE_URL=postgres://…
./bin/mgl-push
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `MGL_PUSH_ENV` | `development` / `production` |
| `MGL_PUSH_HTTP_ADDR` | Default `:8080` |
| `MGL_PUSH_DATABASE_URL` | Postgres DSN |
| `MGL_PUSH_SERVICE_TOKENS` | `token:app_id,token2:app_id2` |
| **Realtime / JWT** | |
| `MGL_PUSH_JWT_SECRET` | HS256 secret (must change in production) |
| `MGL_PUSH_JWT_ISSUER` | Optional issuer check (default `mgl-realtime`) |
| `MGL_PUSH_PHOENIX_BASE_URL` | Authz API; empty = allow-all |
| `MGL_PUSH_PHOENIX_TOKEN` | Phoenix Bearer |
| `MGL_PUSH_PHOENIX_TIMEOUT_MS` | Default 3000 |
| `MGL_PUSH_ICE_SERVERS` | `stun:…` or `turn:host:3478\|user\|pass`, comma-separated |
| `MGL_PUSH_LIVEKIT_URL` | LiveKit URL (optional) |
| `MGL_PUSH_LIVEKIT_API_KEY` / `MGL_PUSH_LIVEKIT_API_SECRET` | SFU token |
| `MGL_PUSH_CALL_RING_TIMEOUT_SEC` | Default 45 |
| `MGL_PUSH_CALL_TOKEN_TTL_SEC` | Default 300 |
| `MGL_PUSH_WS_PING_SEC` | Default 25 |
| `MGL_PUSH_WS_READ_TIMEOUT_SEC` | Default 60 |
| `MGL_PUSH_WS_MSG_PER_SEC` | Signaling rate limit / connection (default 30) |
| `MGL_PUSH_CALLS_PER_MINUTE` | Call create limit / user (default 20) |
| **Push worker** | |
| `MGL_PUSH_MAX_ATTEMPTS` | Default 5 |
| `MGL_PUSH_WORKER_POLL_MS` | Default 1000 |
| `MGL_PUSH_WORKER_BATCH` | Default 50 |
| `MGL_PUSH_RATE_LIMIT_SERVICE` | Per-service (default 1000/min) |
| `MGL_PUSH_RATE_LIMIT_USER` | Per user (default 60/min) |
| `MGL_PUSH_RATE_LIMIT_INSTALLATION` | Per installation (default 30/min) |
| **FCM** | |
| `MGL_PUSH_FCM_CREDENTIALS_FILE` | Firebase Admin JSON |
| **APNs** | |
| `MGL_PUSH_APNS_TEAM_ID` | Apple Team ID |
| `MGL_PUSH_APNS_KEY_ID` | Key ID |
| `MGL_PUSH_APNS_BUNDLE_ID` | Bundle ID (apns-topic) |
| `MGL_PUSH_APNS_PRIVATE_KEY` | `.p8` path or PEM contents |
| `MGL_PUSH_APNS_PRODUCTION` | `true`/`false` (defaults follow ENV) |
| **Huawei** | |
| `MGL_PUSH_HUAWEI_APP_ID` / `MGL_PUSH_HUAWEI_APP_SECRET` | AGC |
| **Xiaomi** | |
| `MGL_PUSH_XIAOMI_APP_SECRET` | AppSecret |
| `MGL_PUSH_XIAOMI_PACKAGE_NAME` | **Android package name** (strongly recommended) |
| `MGL_PUSH_XIAOMI_APP_ID` | Optional |
| **OPPO** | |
| `MGL_PUSH_OPPO_APP_KEY` / `MGL_PUSH_OPPO_MASTER_SECRET` | |
| **vivo** | |
| `MGL_PUSH_VIVO_APP_ID` / `APP_KEY` / `APP_SECRET` | `APP_ID` must be numeric |

Do not commit secrets to Git, Flutter, ClojureDart, the database, or plaintext logs.  
Do not log JWT, Push Token, or full SDP.

## Production

```text
Internet → TLS (Caddy/Nginx) → mgl-realtime-server → PostgreSQL
                              ↘ WSS /ws
                              ↘ LiveKit / coturn (sidecar)
```

- Enforce HTTPS / WSS
- Separate `service-token` per business service; rotate `JWT_SECRET`
- Monitor `/metrics` and `/ready`
- Unconfigured Push vendors use noop (dev only)
- Single node: Call / Presence in memory; do not horizontally scale without shared state
- Production group calls need LiveKit; TURN via coturn, issued through `MGL_PUSH_ICE_SERVERS`

## Phoenix integration

Phoenix must **not** call APNs/FCM directly. Recommended:

```text
Phoenix
  ├── Push Context → /v1/messages* (or let Go Call send Incoming Call)
  ├── Authorize    ← Go POST {PHOENIX}/v1/calls/authorize
  └── Call History ← business persistence after call.ended
```

Incoming Call retries must use `Idempotency-Key: incoming:{call_id}:{user_id}`.

Issue client JWTs (HS256) with claims: `sub` (user_id), `app` (app_id), `exp`, optional `device_id`.
