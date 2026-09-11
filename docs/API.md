# API

Base paths:

- Push (compat): `/v1/...`
- Spec alias: `/api/v1/...` (devices / messages same as `/v1`)
- Realtime Call / Presence: `/api/v1/calls`, `/api/v1/presence`
- WebSocket: `GET /ws`

Auth:

| Use | Header |
|-----|--------|
| Business backend Push / Call | `Authorization: Bearer <service-token>` → `app_id` |
| Client Call / Presence | `Authorization: Bearer <user-JWT>` (claims: `sub`, `app`, optional `device_id`) |
| WebSocket | Upgrade unauthenticated; first message `authenticate` carries JWT |

Optional: `Idempotency-Key` (message endpoints). Duplicate requests return the same `message_id` with `duplicate: true`.

Realtime details: [REALTIME.md](REALTIME.md).

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness |
| GET | `/ready` | Postgres + providers + worker |
| GET | `/metrics` | Prometheus |
| GET | `/ws` | WebSocket Signaling |
| POST | `/v1/devices` · `/api/v1/devices` | Register / upsert device |
| PUT | `…/devices/{installation_id}/token` | Update token |
| PUT | `…/devices/{installation_id}/user` | Bind user |
| DELETE | `…/devices/{installation_id}/user` | Unbind user |
| DELETE | `…/devices/{installation_id}` | Unregister (`status=disabled`) |
| POST | `…/messages` | Notification / silent / background |
| POST | `…/messages/incoming-call` | Incoming-call Push |
| POST | `…/messages/call-cancelled` | Cancel ringing |
| POST | `…/messages/call-ended` | Call ended |
| POST | `/api/v1/calls` | Create call runtime |
| GET | `/api/v1/calls/{id}` | Get call |
| POST | `/api/v1/calls/{id}/accept` | Accept (multi-device stop ringing) |
| POST | `/api/v1/calls/{id}/reject` | Reject |
| POST | `/api/v1/calls/{id}/cancel` | Cancel |
| POST | `/api/v1/calls/{id}/join` | Join |
| POST | `/api/v1/calls/{id}/leave` | Leave |
| POST | `/api/v1/calls/{id}/hangup` | Hangup |
| POST | `/api/v1/calls/{id}/token` | Call / SFU token + ICE |
| POST | `/api/v1/calls/{id}/resume` | Recovery snapshot |
| GET | `/api/v1/presence/{user_id}` | User presence |

## Device registration

```http
POST /v1/devices
```

```json
{
  "installation_id": "ins_123",
  "user_id": "user_123",
  "platform": "android",
  "provider": "xiaomi",
  "token": "…",
  "app_id": "net.amjil.nomio",
  "app_version": "1.0.0",
  "os_version": "15",
  "device_model": "…",
  "locale": "mn",
  "timezone": "Asia/Ulaanbaatar",
  "providers": ["xiaomi", "fcm"],
  "capabilities": ["notification", "silent", "background", "incoming_call"]
}
```

Response:

```json
{ "device_id": "01J…", "installation_id": "ins_123", "status": "active" }
```

`provider` values: `fcm` · `apns` · `apns_voip` · `huawei` · `xiaomi` · `oppo` · `vivo`

## Send message

```http
POST /v1/messages
Idempotency-Key: optional-key
```

```json
{
  "user_ids": ["user_123"],
  "installation_ids": [],
  "type": "notification",
  "notification": { "title": "Nomio", "body": "Someone replied to you" },
  "data": { "type": "comment", "article_id": "123" },
  "priority": "normal",
  "ttl_seconds": 3600,
  "collapse_key": "comment-123",
  "deep_link": "/article/123"
}
```

When `type` is omitted: title/body present → `notification`; data only → `silent`.

You can also send directly by `provider` + `token` (creates a temporary device record).

Response `202`:

```json
{ "message_id": "01J…", "accepted": true, "duplicate": false }
```

Outbound data always includes: `mgl_message_id`, `mgl_event_id`, `mgl_event_type`, `mgl_timestamp`.

## Incoming call

```http
POST /v1/messages/incoming-call
Idempotency-Key: incoming:call_123:user_456
```

```json
{
  "user_ids": ["user_456"],
  "call": {
    "call_id": "call_123",
    "caller_id": "user_123",
    "callee_id": "user_456",
    "media_type": "video",
    "expires_at": 1757500032,
    "caller_display_name": "Alice"
  }
}
```

- Default TTL **30 seconds** (`expires_at` relative to now)
- Expired requests return 400
- Payload must **not** include SDP / ICE / TURN / long-lived credentials
- Incoming Call does **not** use ordinary collapse

> Prefer creating calls via `/api/v1/calls` or WebSocket `call.create`; the Incoming Call module sends Push automatically.

## Call cancelled / ended

```http
POST /v1/messages/call-cancelled
```

```json
{ "call_id": "call_123", "user_ids": ["user_456"] }
```

```http
POST /v1/messages/call-ended
```

```json
{
  "call_id": "call_123",
  "user_ids": ["user_456"],
  "reason": "remote-ended"
}
```

Optional `reason`: `user-ended` · `remote-ended` · `rejected` · `timeout` · `cancelled` · `failed` · `busy` · `accepted_elsewhere`

## Call Runtime

```http
POST /api/v1/calls
Authorization: Bearer <service-token|user-JWT>
```

```json
{
  "caller_id": "user_a",
  "callee_id": "user_b",
  "type": "video",
  "mode": "direct",
  "device_id": "device_a",
  "caller_name": "Alice"
}
```

- `callee_ids` may replace `callee_id` (group)
- With user JWT, `caller_id` may be omitted (taken from `sub`)
- On success, Incoming Call Push is sent automatically; `201` returns the full Call object

```http
POST /api/v1/calls/{id}/accept
{ "user_id": "user_b", "device_id": "iphone" }
```

After accept: other devices of the same user receive stop-ringing Push (`accepted_elsewhere`).

```http
POST /api/v1/calls/{id}/token
{ "user_id": "user_b" }
```

```json
{
  "call_id": "call_…",
  "room_id": "room_…",
  "token": "…",
  "transport": "p2p",
  "ice_servers": [{ "urls": ["stun:stun.l.google.com:19302"] }],
  "role": "callee",
  "expires_in": 300
}
```

Group/`sfu` responses also include `sfu_url`.

```http
POST /api/v1/calls/{id}/resume
{ "user_id": "user_b", "device_id": "iphone" }
```

Returns `call` + `participants` + `room` + `token` + `ice_servers` (see REALTIME.md).

## Presence

```http
GET /api/v1/presence/{user_id}
```

```json
{
  "user_id": "user_b",
  "app_id": "nomio",
  "status": "online",
  "devices": [
    { "device_id": "iphone", "status": "online", "updated_at": "…" }
  ],
  "updated_at": "…"
}
```

## WebSocket

```http
GET /ws
```

Message Envelope:

```json
{
  "id": "msg_…",
  "type": "call.create",
  "timestamp": "2026-01-01T12:00:00Z",
  "call_id": "",
  "sender_id": "",
  "data": { "callee_id": "user_b", "type": "audio" }
}
```

Authenticate:

```json
{ "type": "authenticate", "token": "<JWT>", "device_id": "…", "reconnect": false }
```

Full event table: [REALTIME.md](REALTIME.md).

## Message types

| `type` | Purpose |
|--------|---------|
| `notification` | Visible notification |
| `silent` | Data-only / sync |
| `background` | Background wake |
| `incoming_call` | Incoming-call wake |
| `call_cancelled` | Caller cancelled / stop ringing |
| `call_ended` | Call ended |

## Error format

```json
{ "error": { "code": "DEVICE_NOT_FOUND", "message": "Device not found" } }
```

Common codes: `INVALID_REQUEST` · `FORBIDDEN` · `DEVICE_NOT_FOUND` · `CALL_NOT_FOUND` · `UNAUTHORIZED`

WebSocket: `{ "type": "error", "data": { "code": "RATE_LIMITED", "message": "…" } }`

## Data payload

`data` is `string → string` only. Fetch complex business data via your business API.
