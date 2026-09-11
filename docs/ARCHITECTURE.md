# Architecture

## High-level

```text
                    Application
                         │
              ┌──────────┴──────────┐
              │                     │
          mgl-push               mgl-call
              │                     │
       Push / Events          Call / WebRTC
              │                     │
       Native Push SDK        CallKit / LCK / Telecom
              │                     │
     APNs / FCM / Vendors     flutter_webrtc
```

```text
Business (Phoenix / etc)
        │ HTTP Bearer + Idempotency-Key
        ▼
 mgl-push-server (Go)
   ├── API (/v1/devices, /v1/messages, …)
   ├── DeviceService / MessageService
   ├── Repository (PostgreSQL)
   ├── Queue Worker + Retry
   └── Provider Registry
         ├── fcm · apns · apns_voip
         ├── huawei · xiaomi
         ├── oppo · vivo
         └── noop (fallback when credentials are missing; does not block startup)
                │
                ▼
         Mobile OS Push
                │
                ▼
     Flutter / ClojureDart client
         ├── DomainPushEvent stream
         ├── event_id / call_id dedupe
         └── PendingNativeStore (cold start)
```

## Layers

```text
API → Service → Repository / Provider
```

The business layer only sees: `Message`, `Device`, `Delivery`, and provider name strings.  
Vendor payload types must not appear in Service / API.

## Message types

| Type | Transport intent |
|------|------------------|
| `notification` | Visible notification |
| `silent` / `background` | Data-only / background wake (execution not guaranteed) |
| `incoming_call` | High-priority / VoIP incoming-call wake |
| `call_cancelled` / `call_ended` | Cancel / end signaling-style Push |

On iOS, Incoming Call prefers `apns_voip` when a VoIP token exists; otherwise high-priority APNs.  
Android uses the device’s current vendor / FCM with data-only + high priority.

## Client

- The native SDK owns the long-lived connection and system notifications
- Flutter: `MethodChannel` / `EventChannel`
- `EventBuffer` + `EventDeduper`: cold-start buffering and dedupe
- `PendingNativeStore` (Android) / `PendingEventStore` (iOS): persist when Flutter is not running yet; TTL ~30–120s
- Incoming-call events past `expires_at` are dropped on the client

## Delivery semantics

| Status | Meaning |
|--------|---------|
| `accepted` | Provider accepted the request (≠ user saw / answered) |
| `delivered` / `opened` | Reserved |
| `failed` | Permanent failure (including invalid token) |

Delivery model: **at-least-once**. Clients must dedupe with `mgl_event_id` (and `call_id` for call scenarios).

## Boundary with mgl-call

```text
mgl-push  →  PushEvent(incoming-call)  →  Application  →  mgl-call
```

- mgl-push does **not** `require` mgl-call
- mgl-call does **not** hard-depend on mgl-push (foreground can use WebSocket)
- CallKit / LCK / Telecom / WebRTC all belong to mgl-call
