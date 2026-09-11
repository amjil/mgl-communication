# mgl-communication

Unified Device Push, Incoming Call Events, and Realtime Communication Infrastructure for Flutter / ClojureDart.

## Scope

| Library | Responsibility |
|---------|----------------|
| **mgl-push** | Notify / Wake / Deliver Event / Incoming Call（LCK · CallKit · Android Call UI） |
| **mgl-call** | Client WebRTC / Media / Room / Participant |
| **mgl-realtime-server** | Go Realtime Server: Push + Presence + Call Runtime + Signaling + SFU |

Push and Call stay loosely coupled: cold-start wake uses **PushEvent** (`incoming-call` / `call-cancelled` / `call-ended`); foreground uses **WebSocket**.

> Go owns realtime; Phoenix owns business; SFU owns media.

## Repository layout

```text
mgl-communication/
├── mgl-push-client/           # Flutter plugin (Dart + Android/iOS native)
├── mgl-push-cljd/             # ClojureDart API (thin wrapper)
├── mgl-realtime-server/       # Go Realtime Communication Server
├── mgl-call/                  # WebRTC / media / call lifecycle (client)
├── mgl-push-protocol/         # JSON Schema (PushEvent / Device / etc.)
├── docs/
├── docker-compose.yml
└── .env.example
```

## Status

| Area | Scope | Status |
|------|-------|--------|
| **Push Phase 1–3** | Device, APNs/FCM, Huawei/Xiaomi/OPPO/vivo, Incoming Call Push | ✓ |
| **Incoming Call UI** | iOS LCK (when available) · CallKit · Android Telecom / full-screen Call UI · Accept/Reject | ✓ |
| **Realtime Phase 1–4** | JWT, WS, Presence, Call Runtime, 1:1/Group, Multi-device, Resume, rate limits | ✓ (single-node in-memory) |
| **Realtime Phase 5** | Redis / horizontal scaling | ○ |
| **mgl-call** | Client Signaling / WebRTC / Media | In-repo (`mgl-call/`) |


## Quick start

```bash
docker compose up --build
```

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

> First startup runs `migrations/001_init.sql`, `002_v21_events.sql`, and `003_dual_provider.sql`.  
> If you reuse an existing Postgres volume, apply newer migrations manually.

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

### Create a call (Runtime + Incoming Call Push)

```bash
curl -X POST http://localhost:8080/api/v1/calls \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "caller_id": "user_123",
    "callee_id": "user_456",
    "type": "video",
    "mode": "direct",
    "caller_name": "Alice"
  }'
```

### Incoming Call Push (low-level; usually triggered by Call Runtime)

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

### WebSocket

```text
ws://localhost:8080/ws
→ { "type": "authenticate", "token": "<user-JWT>", "device_id": "…" }
→ call.create / accept / webrtc.offer / …
```

See [docs/REALTIME.md](docs/REALTIME.md).

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
    if (e.isAccepted) {
      // → mgl-call accept / join
    } else if (e.isRinging) {
      // System Call UI already shown by native mgl-push
    } else if (e.isRejected) {
      // user rejected
    }
  }
});
```

### ClojureDart

```clojure
(require '[mgl.push.api :as push])

(push/init! {:server-url "https://push.example.com"
             :service-token "…"
             :app-id "net.amjil.nomio"})

;; Unique key → hot-reload safe (same key overwrites). Returns cleanup fn.
(push/on-event ::mgl-call
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
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System boundaries, Push / Realtime layers |
| [REALTIME.md](docs/REALTIME.md) | **WS / Call / Presence / SFU / multi-device** |
| [CLIENT.md](docs/CLIENT.md) | Flutter / CLJD / platform |
| [SERVER.md](docs/SERVER.md) | Go modules, pipelines, implementation phases |
| [API.md](docs/API.md) | HTTP + WS endpoint reference |
| [PROVIDERS.md](docs/PROVIDERS.md) | Vendor configuration |
| [DEPLOYMENT.md](docs/DEPLOYMENT.md) | Env vars and deployment |
| [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common issues |

Protocol schemas: [`mgl-push-protocol/`](mgl-push-protocol/)

## Design principles

1. Provider logic stays inside adapters — never exposed to the business layer
2. `installation_id` ≠ vendor token
3. Provider credentials exist only on the Go server
4. At-least-once delivery; clients dedupe with `event_id` + `call_id`
5. No dependency on `flutter_webrtc` / `mgl-call` inside Push
6. Keep the PushEvent contract stable and backward-compatible
7. Keep Incoming Call payloads small (no SDP / ICE / TURN)
8. One Go binary, modular packages — no premature microservice split
9. Call API hides P2P vs SFU; runtime chooses transport
