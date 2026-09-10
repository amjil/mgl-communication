# mgl-push

Unified cross-platform push infrastructure.

## Current status (Phase 7 complete)

All providers in the spec are implemented:

**FCM · APNs · Huawei · Xiaomi · OPPO · vivo**

Providers without credentials fall back to noop automatically and do not block startup.

## Quick start

```bash
docker compose up --build
```

Health checks:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

Register a device:

```bash
curl -X POST http://localhost:8080/v1/devices \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "installation_id": "01TESTINSTALL0001",
    "platform": "android",
    "provider": "fcm",
    "token": "phase1-token",
    "app_id": "net.amjil.demo",
    "app_version": "0.1.0"
  }'
```

Send a message:

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "installation_ids": ["01TESTINSTALL0001"],
    "notification": { "title": "Hello", "body": "Phase 1" },
    "data": { "type": "test" }
  }'
```

## Repository layout

```text
mgl-push/
├── mgl-push-client/      # Flutter plugin
├── mgl-push-cljd/        # ClojureDart wrapper
├── mgl-push-server/      # Go server
├── docs/                 # Documentation
├── docker-compose.yml
└── mgl-push-spec.md
```

## Documentation

- [ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [CLIENT.md](docs/CLIENT.md)
- [SERVER.md](docs/SERVER.md)
- [PROVIDERS.md](docs/PROVIDERS.md)
- [API.md](docs/API.md)
- [DEPLOYMENT.md](docs/DEPLOYMENT.md)
- [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)

## Design principles (summary)

1. Flutter does **not** implement the push long-lived connection
2. Push is separate from realtime
3. Provider logic lives only under `internal/provider/`
4. `installation_id` is separate from vendor tokens
5. Provider secrets exist only on the server
6. at-least-once + delivery tracking + retry + token invalidation

## Roadmap

1. ~~Phase 1 — Foundation~~
2. ~~Phase 2 — FCM~~
3. ~~Phase 3 — APNs~~
4. ~~Phase 4 — Huawei~~
5. ~~Phase 5 — Xiaomi~~
6. ~~Phase 6 — OPPO~~
7. ~~Phase 7 — vivo~~

All phases are complete. See [PROVIDERS.md](docs/PROVIDERS.md) and `.env.example`.

## Local development (without building a Docker image)

```bash
# Postgres
docker compose up -d postgres

# Server
cd mgl-push-server
export MGL_PUSH_DATABASE_URL='postgres://mglpush:mglpush@localhost:5432/mglpush?sslmode=disable'
export MGL_PUSH_SERVICE_TOKENS='dev-token:net.amjil.demo'
go run ./cmd/mgl-push
```

Apply `migrations/001_init.sql` on first run (Compose applies it automatically).
