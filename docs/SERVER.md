# Server

## Running

See [DEPLOYMENT.md](DEPLOYMENT.md) for environment variables.

Binary:

```bash
cd mgl-push-server
go build -o bin/mgl-push ./cmd/mgl-push
./bin/mgl-push
```

## Modules

| Package | Responsibility |
|---------|----------------|
| `internal/api` | HTTP handlers |
| `internal/service` | Device / Message business logic |
| `internal/repository` | PostgreSQL |
| `internal/queue` | Worker |
| `internal/retry` | Backoff strategy |
| `internal/provider` | Provider interface + registry |
| `internal/auth` | Bearer service token → app_id |

## Worker

```text
Claim pending/retrying deliveries
 → Resolve device + message
 → Provider.Send
 → accepted | retrying | failed + invalidate token
 → Complete message when all deliveries terminal
```

## Graceful shutdown

SIGTERM/SIGINT → stop HTTP → stop worker → close providers → close DB.
