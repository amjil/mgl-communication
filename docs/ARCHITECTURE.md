# Architecture

```text
Business (Phoenix / etc)
        │ HTTP Bearer
        ▼
 mgl-push-server (Go)
   ├── API
   ├── DeviceService / MessageService
   ├── Repository (PostgreSQL)
   ├── Queue Worker
   ├── Retry
   └── Provider Registry
         ├── noop (Phase 1)
         ├── fcm (Phase 2)
         ├── apns (Phase 3)
         └── huawei / xiaomi / oppo / vivo
                │
                ▼
         Mobile OS Push
                │
                ▼
     Flutter / ClojureDart client
```

## Layers

```text
API → Service → Repository / Provider
```

The business domain may only use: `Message`, `Device`, `Delivery`, and provider name strings.

Vendor payload types must not appear in Service or API layers.

## Client

- Native SDKs own the long-lived connection and system notifications
- Flutter talks via MethodChannel / EventChannel
- EventBuffer keeps events when cold start or listeners are not ready yet

## Delivery semantics

- `accepted`: the provider accepted the request
- `delivered` / `opened`: reserved for later phases
- Delivery model: at-least-once; clients dedupe with `mgl_message_id`
