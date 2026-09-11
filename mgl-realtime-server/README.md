# mgl-realtime-server

MGL Realtime Communication Server (Go).

Docs: [`../docs/REALTIME.md`](../docs/REALTIME.md) · [`../docs/SERVER.md`](../docs/SERVER.md) · [`../docs/API.md`](../docs/API.md)

## Build & run

```bash
go build -o bin/mgl-push ./cmd/mgl-push
./bin/mgl-push
```

Or from the repo root:

```bash
docker compose up --build
```

## Capabilities

- Push: APNs / FCM / China vendors + Incoming Call
- WebSocket signaling: `/ws`
- Presence, Call Runtime (1:1 P2P / group SFU)
- Multi-device stop-ringing, Call Resume, rate limiting
- Phoenix authorization hook, LiveKit token / ICE issuance

See [`.env.example`](../.env.example) and [`docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md) for environment variables.
