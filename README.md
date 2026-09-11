# mgl-push

Unified Device Push & Incoming Communication Event Infrastructure for Flutter / ClojureDart.

**Spec:** [mgl-push-spec-2.1.md](mgl-push-spec-2.1.md)

## Scope

| Library | Responsibility |
|---------|----------------|
| **mgl-push** | Notify / Wake / Deliver Event (this repo) |
| **mgl-call** | CallKit / LCK / Telecom / Signaling / WebRTC (separate repo) |

This repository does **not** implement CallKit, LiveCommunicationKit, Android Telecom, or WebRTC.  
The only stable contract with `mgl-call` is **PushEvent** (`incoming-call` / `call-cancelled` / `call-ended`).

## Repository layout

```text
mgl-push/
├── mgl-push-client/      # Flutter plugin (Dart + Android/iOS native)
├── mgl-push-cljd/        # ClojureDart API (thin wrapper, no vendor logic)
├── mgl-push-server/      # Go Push Gateway
├── mgl-push-protocol/    # JSON Schema (PushEvent / Device / etc.)
├── docs/
├── docker-compose.yml
├── .env.example
└── mgl-push-spec-2.1.md
```

## MVP status (spec §148–150)

| Phase | Scope | Status |
| --- | --- | --- |
| **Phase 1** | Core, Installation ID, Token, APNs/FCM, PushEvent, Incoming Call, dedupe, Pending | ✓ |
| **Phase 2** | Huawei + Xiaomi | ✓ (Xiaomi requires host SDK + Receiver) |
| **Phase 3** | OPPO + vivo | ✓ (host SDK; reflection Bridge) |
| **mgl-call** | System Call + WebRTC | Separate repo |

## Quick start

```bash
docker compose up --build
```

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

> First startup runs `migrations/001_init.sql` and `002_v21_events.sql`.  
> If you reuse an existing Postgres volume, apply `002_v21_events.sql` manually.

### Register a device

```bash
curl -X POST http://localhost:8080/v1/devices \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "installation_id": "01TESTINSTALL0001",
    "platform": "android",
    "provider": "fcm",
    "token": "demo-token",
    "app_id": "net.amjil.demo",
    "app_version": "0.1.0",
    "capabilities": ["notification", "silent", "background", "incoming_call"]
  }'
```

### Notification

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "user_ids": ["user_123"],
    "type": "notification",
    "notification": { "title": "Nomio", "body": "Hello" },
    "data": { "type": "comment", "article_id": "123" }
  }'
```

### Incoming Call

```bash
curl -X POST http://localhost:8080/v1/messages/incoming-call \
  -H "Authorization: Bearer dev-token" \
  -H "Idempotency-Key: incoming:call_123:user_456" \
  -H "Content-Type: application/json" \
  -d '{
    "user_ids": ["user_456"],
    "call": {
      "call_id": "call_123",
      "caller_id": "user_123",
      "callee_id": "user_456",
      "media_type": "video",
      "expires_at": 9999999999
    }
  }'
```

### Call cancelled / ended

```bash
curl -X POST http://localhost:8080/v1/messages/call-cancelled \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{ "call_id": "call_123", "user_ids": ["user_456"] }'
```

## Client usage

### Flutter

```dart
final push = createMglPush(MglPushConfig(
  serverUrl: 'https://push.example.com',
  serviceToken: 'your-service-token',
  appId: 'net.amjil.nomio',
));

await push.initialize();
await push.requestPermission();
final caps = await push.getCapabilities();

push.events.listen((e) {
  if (e is DomainPushEvent && e.isIncomingCall) {
    // → mgl-call
  }
});
```

### ClojureDart

```clojure
(require '[mgl.push.api :as push])

(push/init! {:server-url "https://push.example.com"
             :service-token "…"
             :app-id "net.amjil.nomio"})

(push/on-event
  (fn [event]
    (when (push/incoming-call? event)
      ;; Hand off to mgl-call; never require mgl-call inside mgl-push
      (call/incoming! (push/event-data event)))))
```

## PushEvent contract

Unified client event shape (see `mgl-push-protocol/event.schema.json`):

```json
{
  "version": 1,
  "id": "evt_…",
  "type": "incoming-call",
  "timestamp": 1757500000,
  "data": {
    "call_id": "call_123",
    "caller_id": "user_a",
    "callee_id": "user_b",
    "media_type": "video",
    "expires_at": "1757500030"
  }
}
```

`type` values: `notification` · `silent` · `background` · `incoming-call` · `call-cancelled` · `call-ended`  
Lifecycle events: `token_changed` · `notification_open` · `error`.

## Documentation

| Doc | Content |
|-----|---------|
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Layers, boundaries, delivery semantics |
| [CLIENT.md](docs/CLIENT.md) | Flutter / CLJD / platform integration |
| [SERVER.md](docs/SERVER.md) | Go modules, worker, migrations |
| [API.md](docs/API.md) | Full HTTP API reference |
| [PROVIDERS.md](docs/PROVIDERS.md) | Vendor config and capabilities |
| [DEPLOYMENT.md](docs/DEPLOYMENT.md) | Env vars and deployment |
| [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common issues |

Protocol schemas: [`mgl-push-protocol/`](mgl-push-protocol/)

## Design principles

1. Provider logic stays inside adapters — never exposed to the business layer
2. `installation_id` ≠ vendor token
3. Provider credentials exist only on the Go server
4. At-least-once delivery; clients dedupe with `event_id` + `call_id`
5. No dependency on `flutter_webrtc` / `mgl-call`
6. Keep the PushEvent contract stable and backward-compatible
7. Keep Incoming Call payloads small (no SDP / ICE / TURN)
