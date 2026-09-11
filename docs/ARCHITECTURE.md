# Architecture

## High-level

```text
                    MGL Applications
                          │
             ┌────────────┴────────────┐
             │                         │
       mgl-push Client          mgl-call Client
       Push / Device            WebRTC / Media
             │                         │
             └────────────┬────────────┘
                          ▼
                mgl-realtime-server (Go)
                ├── Push · Device · Queue
                ├── Presence · Call Runtime
                ├── Incoming Call
                ├── WebSocket Signaling
                └── SFU Integration
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
         Phoenix       Push vendors   SFU / coturn
```

Design principle: **Go owns realtime; Phoenix owns business; SFU owns media.**  
See also: [`docs/REALTIME.md`](REALTIME.md).

## Push pipeline (persisted)

```text
Business (Phoenix / etc)
        │ HTTP Bearer + Idempotency-Key
        ▼
 mgl-realtime-server
   ├── API (/v1|/api/v1 devices, messages)
   ├── DeviceService / MessageService
   ├── Repository (PostgreSQL)
   ├── Queue Worker + Retry
   └── Provider Registry
         ├── fcm · apns · apns_voip
         ├── huawei · xiaomi · oppo · vivo
         └── noop
                │
                ▼
         Mobile OS Push → mgl-push Client
```

## Realtime runtime (in-memory, single node)

```text
Client JWT
   │
   ▼
GET /ws  → authenticate → Presence online
   │
   ├── call.*  → Call Runtime → (optional) Incoming Call Push
   ├── webrtc.* / media.* → relay to other participants
   └── disconnect → Presence offline

HTTP /api/v1/calls|presence  (service token or user JWT)
```

Runtime state (WebSocket, Presence, Call Session) lives in-process; Push delivery is stored in PostgreSQL.

## Layers

```text
API / WebSocket Hub
  → Call Orchestrator (Phoenix authz + SFU token + ICE)
  → Call Service / Presence / Incoming Call
  → MessageService → Repository / Provider
```

The business layer only sees: `Message`, `Device`, `Delivery`, `Call`, and provider name strings.  
Vendor payload types must not appear in Service / API.

## Message types (Push)

| Type | Transport intent |
|------|------------------|
| `notification` | Visible notification |
| `silent` / `background` | Data-only / background wake |
| `incoming_call` | High-priority / VoIP wake |
| `call_cancelled` / `call_ended` | Cancel / end signaling-style Push |

On iOS, Incoming Call prefers `apns_voip`; Android uses the current vendor / FCM high-priority data-only path.

## Call transport

| Mode | Transport | Media |
|------|-----------|--------|
| `direct` | P2P | Client WebRTC + STUN/TURN |
| `group` | SFU | LiveKit (or stub) |

Client APIs do not expose P2P/SFU differences: unified `create` / `join` / `leave`.

## Multi-device Incoming Call

```text
ring all devices → accept on one
  → call.stop_ringing (WS to other devices)
  → call_cancelled Push (same user)
```

## Client

- Native SDK: long-lived connection and system notifications (Push)
- Flutter: `MethodChannel` / `EventChannel`
- `EventBuffer` + `EventDeduper`; Pending store (cold start)
- Incoming-call events past `expires_at` are dropped on the client
- Foreground calls: mgl-call uses WebSocket Signaling

## Delivery semantics (Push)

| Status | Meaning |
|--------|---------|
| `accepted` | Provider accepted (≠ user saw / answered) |
| `delivered` / `opened` | Reserved |
| `failed` | Permanent failure (including invalid token) |

**at-least-once**; clients dedupe with `mgl_event_id` (and `call_id`).

## Boundary with mgl-call

```text
mgl-push  →  PushEvent(incoming-call)  →  Application  →  mgl-call
mgl-call  ↔  WebSocket Signaling / Call Runtime  ↔  Go Server
```

- mgl-push does not `require` mgl-call
- mgl-call does not hard-depend on mgl-push (foreground can use WS alone)
- CallKit / LCK / Telecom / WebRTC belong to mgl-call
- Go does not implement SFU / RTP / Codec
