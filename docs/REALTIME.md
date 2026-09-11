# mgl-realtime-server

Go **Realtime Communication Server**: Push, Device, Presence, Incoming Call, Call Runtime, WebSocket Signaling, SFU Integration.

Core principle:

> **Go owns realtime; Phoenix owns business; SFU owns media.**

## Responsibility boundary

| Component | Owns |
|-----------|------|
| **Go Server** | WebSocket, Signaling, Presence, Device Session, Push, Incoming Call, Call Runtime, SFU Token, Reconnect |
| **Phoenix** | User, AuthZ business rules, Friendship, Groups, Call History |
| **SFU (LiveKit, etc.)** | Media routing, A/V forwarding |
| **coturn** | TURN Relay (Go only issues ICE config) |
| **mgl-push Client** | Push Token, VoIP Push, Incoming Call UI（LCK / CallKit / Android Call UI） |
| **mgl-call Client** | WebRTC, A/V, Room, Participant, Media State |

## Architecture

```text
         mgl-push Client              mgl-call Client
                │                            │
                └────────────┬───────────────┘
                             ▼
                   mgl-realtime-server (Go)
                   ├── Push / Device / Queue
                   ├── Presence (memory)
                   ├── Call Runtime (memory)
                   ├── Incoming Call → Push
                   ├── WebSocket Signaling
                   └── SFU Integration
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
          Phoenix        APNs/FCM        LiveKit
                         + vendors       (+ coturn)
```

## Runtime state (single node)

Call / Presence / WebSocket connections are kept in **process memory**.  
Push delivery state is still persisted in PostgreSQL.

Do not assume Call Sessions are shared across instances until multi-node support lands.

## Authentication

| Scenario | Method |
|----------|--------|
| Business backend (Phoenix → Push / Call HTTP) | `Authorization: Bearer <service-token>` → `app_id` |
| Client WebSocket / Call HTTP | `Authorization: Bearer <user-JWT>` (HS256) |
| WebSocket upgrade | `/ws` skips middleware; first message `authenticate` carries JWT |

JWT must at least include:

```json
{ "sub": "user_xxx", "app": "nomio", "exp": 1234567890, "device_id": "optional" }
```

Dev default secret: `MGL_PUSH_JWT_SECRET` (must change in production).

## HTTP: Call / Presence

Prefix: `/api/v1` (Push devices/messages still support `/v1` and `/api/v1`).

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/calls` | Create call (after Phoenix authz) |
| GET | `/api/v1/calls/{id}` | Get call |
| POST | `/api/v1/calls/{id}/accept` | Accept (triggers multi-device stop ringing) |
| POST | `/api/v1/calls/{id}/reject` | Reject |
| POST | `/api/v1/calls/{id}/cancel` | Caller cancel |
| POST | `/api/v1/calls/{id}/join` | Join |
| POST | `/api/v1/calls/{id}/leave` | Leave |
| POST | `/api/v1/calls/{id}/hangup` | Hangup |
| POST | `/api/v1/calls/{id}/token` | Call / SFU Token + ICE |
| POST | `/api/v1/calls/{id}/resume` | Reconnect recovery snapshot |
| GET | `/api/v1/presence/{user_id}` | Online presence |

Create-call example (service token + body `caller_id`, or client JWT):

```bash
curl -X POST http://localhost:8080/api/v1/calls \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "caller_id": "user_a",
    "callee_id": "user_b",
    "type": "video",
    "mode": "direct",
    "device_id": "device_a",
    "caller_name": "Alice"
  }'
```

When `mode` is omitted: one callee → `direct` (P2P); multiple → `group` (SFU).

Resume response (spec §26):

```json
{
  "call_id": "call_…",
  "state": "connected",
  "participants": [],
  "room": { "room_id": "room_…", "transport": "p2p" },
  "call": { },
  "token": { "token": "…", "transport": "p2p", "ice_servers": [] },
  "ice_servers": [{ "urls": ["stun:…"] }]
}
```

## WebSocket Signaling

```text
GET /ws
```

### Authentication

Client:

```json
{ "type": "authenticate", "token": "<JWT>", "device_id": "device_xxx", "reconnect": true }
```

Server:

```json
{
  "type": "authenticated",
  "sender_id": "user_xxx",
  "data": {
    "user_id": "user_xxx",
    "app_id": "nomio",
    "device_id": "device_xxx",
    "reconnect": true,
    "active_calls": []
  }
}
```

### Unified message format

```json
{
  "id": "msg_…",
  "type": "call.accepted",
  "timestamp": "2026-01-01T12:00:00Z",
  "call_id": "call_…",
  "sender_id": "user_…",
  "data": {}
}
```

WebSocket **ping/pong** is supported (server sends periodic Ping).

### Client → Server

| type | Description |
|------|-------------|
| `call.create` | Create (data: `callee_id` / `callee_ids`, `type`, `mode`, `caller_name`) |
| `call.accept` / `reject` / `cancel` / `join` / `leave` / `hangup` | Lifecycle |
| `call.resume` / `session.resume` | Resume; reply `call.state` |
| `webrtc.offer` / `answer` / `ice_candidate` | P2P signaling relay |
| `media.mute` / `unmute` / `camera_on` / `camera_off` | Media state relay |
| `ping` | App-level heartbeat → `pong` |

### Server → Client

| type | Description |
|------|-------------|
| `call.created` / `ringing` / `accepted` / `rejected` / `cancelled` | State |
| `call.participant_joined` / `participant_left` | Participants |
| `call.connected` / `ended` / `failed` / `state` | Connection & recovery |
| `call.stop_ringing` | **Multi-device**: stop ringing on other devices |
| `presence.updated` | Presence |
| `error` | `{ "code", "message" }` |

## Call state machine

Call: `created → ringing → accepted → connecting → connected → ended`  
Also: `rejected` / `cancelled` / `timeout` / `failed`.

Participant: `invited` / `ringing` / `accepted` / `connecting` / `connected` / `left` / `reconnecting` / …

Transport:

- `direct` → `p2p` (STUN/TURN via ICE config)
- `group` → `sfu` (LiveKit token; stub when credentials unset)

## Incoming Call and multi-device

```text
call.create
  → Call Runtime (ringing)
  → Push incoming_call to all callee devices
  → one device accept
      → WS call.accepted (all participants)
      → WS call.stop_ringing (other devices of same user)
      → Push call_cancelled (same user, reason=accepted_elsewhere)
```

Ring timeout (default 45s) → Call `timeout` + cancel Push.

## Presence

Statuses: `offline` · `online` · `idle` · `busy` · `in_call`  
User-level status aggregates devices (`in_call` > `busy` > `online` > …).

WebSocket auth success → device online; disconnect → device offline.

## Rate limiting

| Dimension | Default | Env |
|-----------|---------|-----|
| Signaling messages / connection | 30 / sec | `MGL_PUSH_WS_MSG_PER_SEC` |
| Call creates / user | 20 / min | `MGL_PUSH_CALLS_PER_MINUTE` |

Over limit returns `error.code = RATE_LIMITED`.

## Phoenix authorization

When `MGL_PUSH_PHOENIX_BASE_URL` is set, create-call requests:

```http
POST {base}/v1/calls/authorize
{ "app_id", "caller_id", "callee_ids" }
→ { "allowed": true|false, "reason": "…" }
```

Unset → **allow-all** in development. When Phoenix is unavailable, reject new calls (keep established runtimes when possible).

## SFU / ICE

| Variable | Description |
|----------|-------------|
| `MGL_PUSH_ICE_SERVERS` | `stun:…` or `turn:host:3478\|user\|pass` (comma-separated) |
| `MGL_PUSH_LIVEKIT_URL` | LiveKit URL |
| `MGL_PUSH_LIVEKIT_API_KEY` / `SECRET` | Issue room tokens |

Without LiveKit, group calls use stub tokens (dev only; not production media).

## Metrics (selected)

- `websocket_connections` / `websocket_reconnects`
- `call_created_total` / `call_connected_total` / `call_ended_total` / `call_failed_total`
- Existing `push_*` / `incoming_call_push_*`

## Related docs

- [API.md](API.md) — Full HTTP reference
- [SERVER.md](SERVER.md) — Modules and pipelines
- [DEPLOYMENT.md](DEPLOYMENT.md) — Environment variables
- [ARCHITECTURE.md](ARCHITECTURE.md) — System boundaries
