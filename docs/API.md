# API

Base: `/v1`  
Auth: `Authorization: Bearer <service-token>` (maps to `app_id`)

Optional: `Idempotency-Key` (message endpoints, spec §86). Duplicate requests return the same `message_id` with `duplicate: true`.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness |
| GET | `/ready` | Postgres + providers + worker |
| GET | `/metrics` | Prometheus |
| POST | `/v1/devices` | Register / upsert device |
| PUT | `/v1/devices/{installation_id}/token` | Update token |
| PUT | `/v1/devices/{installation_id}/user` | Bind user |
| DELETE | `/v1/devices/{installation_id}/user` | Unbind user |
| DELETE | `/v1/devices/{installation_id}` | Unregister (`status=disabled`) |
| POST | `/v1/messages` | Notification / silent / background |
| POST | `/v1/messages/incoming-call` | Incoming-call Push |
| POST | `/v1/messages/call-cancelled` | Cancel ringing |
| POST | `/v1/messages/call-ended` | Call ended |

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

Optional `reason`: `user-ended` · `remote-ended` · `rejected` · `timeout` · `cancelled` · `failed` · `busy`

## Message types

| `type` | Purpose |
|--------|---------|
| `notification` | Visible notification |
| `silent` | Data-only / sync |
| `background` | Background wake |
| `incoming_call` | Incoming-call wake |
| `call_cancelled` | Caller cancelled |
| `call_ended` | Call ended |

## Error format

```json
{ "error": { "code": "DEVICE_NOT_FOUND", "message": "Device not found" } }
```

Common codes: `INVALID_REQUEST` · `FORBIDDEN` · `DEVICE_NOT_FOUND` · `UNAUTHORIZED`

## Data payload

`data` is `string → string` only. Fetch complex business data via your business API.
