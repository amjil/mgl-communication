# Deployment

## Docker Compose (dev / small scale)

Repo root:

```bash
cp .env.example .env   # fill in as needed
docker compose up --build -d
```

Services:

- `postgres:16` — init mounts `migrations/001_init.sql` + `002_v21_events.sql`
- `mgl-push` — default `http://localhost:8080`, `dev-token` → `net.amjil.demo`

> **Note:** init SQL runs only on **first create of an empty volume**. For existing data, run `002_v21_events.sql` manually.

## Binary

```bash
cd mgl-push-server
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
| `MGL_PUSH_MAX_ATTEMPTS` | Default 5 |
| `MGL_PUSH_WORKER_POLL_MS` | Default 1000 |
| `MGL_PUSH_WORKER_BATCH` | Default 50 |
| `MGL_PUSH_RATE_LIMIT_SERVICE` | Per-service rate limit (default 1000/min) |
| `MGL_PUSH_RATE_LIMIT_USER` | Per user (default 60/min) |
| `MGL_PUSH_RATE_LIMIT_INSTALLATION` | Per installation (default 30/min) |
| **FCM (Phase 1)** | |
| `MGL_PUSH_FCM_CREDENTIALS_FILE` | Firebase Admin JSON |
| **APNs (Phase 1)** | |
| `MGL_PUSH_APNS_TEAM_ID` | Apple Team ID |
| `MGL_PUSH_APNS_KEY_ID` | Key ID |
| `MGL_PUSH_APNS_BUNDLE_ID` | Bundle ID (apns-topic) |
| `MGL_PUSH_APNS_PRIVATE_KEY` | `.p8` path or PEM contents |
| `MGL_PUSH_APNS_PRODUCTION` | `true`/`false` (defaults follow ENV) |
| **Huawei (Phase 2)** | |
| `MGL_PUSH_HUAWEI_APP_ID` | AGC App ID |
| `MGL_PUSH_HUAWEI_APP_SECRET` | App Secret |
| **Xiaomi (Phase 2)** | |
| `MGL_PUSH_XIAOMI_APP_SECRET` | AppSecret |
| `MGL_PUSH_XIAOMI_PACKAGE_NAME` | **Android package name** (strongly recommended) |
| `MGL_PUSH_XIAOMI_APP_ID` | Optional (for client meta) |
| **OPPO (Phase 3)** | |
| `MGL_PUSH_OPPO_APP_KEY` | AppKey |
| `MGL_PUSH_OPPO_MASTER_SECRET` | MasterSecret |
| **vivo (Phase 3)** | |
| `MGL_PUSH_VIVO_APP_ID` | Numeric appId |
| `MGL_PUSH_VIVO_APP_KEY` | appKey |
| `MGL_PUSH_VIVO_APP_SECRET` | appSecret |

Do not commit secrets to Git, Flutter, ClojureDart, the database, or plaintext logs.

## Production

```text
Internet → TLS (Caddy/Nginx) → mgl-push-server → PostgreSQL
```

- Enforce HTTPS
- Separate `service-token` per business service
- Monitor `/metrics` and `/ready`
- Configure credentials per vendor; unconfigured channels use noop (acceptable for development only)

## Phoenix integration

Phoenix should **not** call APNs/FCM/vendor APIs directly. Use:

```text
Phoenix Push Context → mgl-push HTTP API
```

For incoming-call scenarios, always send `Idempotency-Key: incoming:{call_id}:{user_id}`.
