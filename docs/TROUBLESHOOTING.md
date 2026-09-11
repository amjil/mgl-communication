# Troubleshooting

## `/ready` returns not_ready

| Check | Meaning |
|-------|---------|
| `postgres: unavailable` | Check `MGL_PUSH_DATABASE_URL`; confirm migrations applied |
| `providers: none` | Registry empty (unexpected) |
| `worker: stopped` | Worker not started or already exited |
| `websocket: disabled` | Hub not wired (unexpected) |

Missing vendor credentials still register **noop**, so `providers` is usually not `none`.

## Migration / `type` column errors

If an old volume only has `001`, run:

```bash
psql "$MGL_PUSH_DATABASE_URL" -f mgl-realtime-server/migrations/002_v21_events.sql
```

Or recreate the Compose volume.

## Device registration 401

`Authorization: Bearer …` must be listed in `MGL_PUSH_SERVICE_TOKENS`, and `app_id` must match the token mapping.  
Call / Presence may also use a user JWT (`sub` + `app`); secret is `MGL_PUSH_JWT_SECRET`.

## Device registration 403 app_id mismatch

Request body `app_id` must match the app bound to the service token.

## WebSocket `authenticate` → UNAUTHORIZED

- JWT must be HS256 with the same secret as the server
- `sub` / `app` required; `exp` must not be expired
- Connect to `/ws` first, then send `{ "type":"authenticate", "token":"…" }` (do not expect Upgrade Header JWT alone)

## WebSocket `RATE_LIMITED`

- Signaling: ~30 msg/s per connection by default (`MGL_PUSH_WS_MSG_PER_SEC`)
- Call create: ~20/min per user (`MGL_PUSH_CALLS_PER_MINUTE`)

## Call create 403 not allowed

When `MGL_PUSH_PHOENIX_BASE_URL` is set, Phoenix `/v1/calls/authorize` returned `allowed:false` or is unavailable.  
For development, clear `PHOENIX_BASE_URL` (allow-all).

## Call `CALL_NOT_FOUND` after restart

Call Runtime is **in-memory**; sessions are lost on process restart. Clients should `call.create` again or treat the call as ended. Multi-node / persistence is Phase 5 (Redis).

## Other devices keep ringing after accept

- Use `/api/v1/calls/{id}/accept` or WS `call.accept` (sends `call.stop_ringing` + Push)
- Manual `/v1/messages/incoming-call` alone does not coordinate multi-device; business must send `call-cancelled`
- Clients should handle `call.stop_ringing` / `call-cancelled` (`accepted_elsewhere`)

## Resume missing ICE / SFU token

- P2P: check `MGL_PUSH_ICE_SERVERS`
- Group: check LiveKit URL/key/secret; without them only stub tokens are issued (cannot join real media)

## Send succeeds but no notification arrives

1. Confirm the provider is **not noop** (startup logs; `/ready` providers list alone is insufficient — check credentials)
2. Device `status` should be `active` (sending stops after invalid token)
3. Android: vendor channel may be killed by the OS; OPPO/vivo need a notification channel
4. iOS: real device + Push capability; sandbox vs production must match `MGL_PUSH_APNS_PRODUCTION`

## Token change not synced

Client must listen for `token_changed` (auto after `initialize`) and `PUT /v1/devices/{installation_id}/token`.

## Incoming Call missing / disappears immediately

- Check whether `expires_at` has passed (client drops expired events)
- iOS: VoIP token / PushKit present? Has CallKit been reported? Did `endSystemCall` / `call-cancelled` dismiss the system UI?
- Android: full-screen Incoming Call UI / notification channel `mgl_incoming_call`; cold start needs `USE_FULL_SCREEN_INTENT`
- Multi-device: after one answers, server sends stop-ringing / `call-cancelled`

## Duplicate incoming-call UI

Push is at-least-once. Clients should dedupe on `event_id` + `call_id` (`EventDeduper` already does).  
Server should use `Idempotency-Key` for Phoenix retries.

## APNs: no token / 403

- Debug on a real device; enable Push (and VoIP) capabilities
- `MGL_PUSH_APNS_*` must match the Apple Key
- Dev sandbox: `MGL_PUSH_APNS_PRODUCTION=false`

## Huawei: no token / 80300007

- HMS Core + `agconnect-services.json`
- Server App ID/Secret must be the same app as the client
- Invalid token → `devices.status=invalid`

## Xiaomi register / send failure

- Host includes MiPush SDK + correct meta-data
- `PushMessageReceiver` registered and forwards to `XiaomiBridge`
- Server sets **`MGL_PUSH_XIAOMI_PACKAGE_NAME`** to the Android package name (not business `app_id`)

## OPPO auth / no notification

- Check `MGL_PUSH_OPPO_APP_KEY` / `MASTER_SECRET`
- Client creates `mgl_default` (or Category) channel
- Invalid registration_id → mark invalid

## vivo auth / 10302

- `MGL_PUSH_VIVO_APP_ID` must be numeric
- sign = md5(appId+appKey+timestamp+appSecret) lowercase
- `10302` = invalid regId → mark invalid

## Lost events on cold start

- Android: confirm message path reaches `*Bridge` / `PendingNativeStore`
- iOS: `PendingEventStore` + `consumePendingEvents`
- Flutter: `initialize` must finish to consume pending

## App fails to start because of Push

Should not happen. `initialize` catches errors; if it still crashes, file an issue with platform logs.

## Log safety

OK to log: `message_id` · `event_id` · `call_id` · `device_id` · `provider` · `status` · `user_id`  
**Do not** log: push tokens, JWT, APNs/FCM/vendor secrets, full SDP, sensitive payloads.
