# Troubleshooting

## `/ready` returns not_ready

| Check | Meaning |
|-------|---------|
| `postgres: unavailable` | Check `MGL_PUSH_DATABASE_URL`; confirm migrations applied |
| `providers: none` | Registry empty (unexpected) |
| `worker: stopped` | Worker not started or already exited |

Missing vendor credentials still register **noop**, so `providers` is usually not `none`.

## Migration / `type` column errors

If an old volume only has `001`, run:

```bash
psql "$MGL_PUSH_DATABASE_URL" -f mgl-push-server/migrations/002_v21_events.sql
```

Or recreate the Compose volume.

## Device registration 401

`Authorization: Bearer …` must be listed in `MGL_PUSH_SERVICE_TOKENS`, and `app_id` must match the token mapping.

## Device registration 403 app_id mismatch

Request body `app_id` must match the app bound to the service token.

## Send succeeds but no notification arrives

1. Confirm the provider is **not noop** (startup logs; `/ready` providers list alone is insufficient — check credentials)
2. Device `status` should be `active` (sending stops after invalid token)
3. Android: vendor channel may be killed by the OS; OPPO/vivo need a notification channel
4. iOS: real device + Push capability; sandbox vs production must match `MGL_PUSH_APNS_PRODUCTION`

## Token change not synced

Client must listen for `token_changed` (auto after `initialize`) and `PUT /v1/devices/{installation_id}/token`.

## Incoming Call missing / disappears immediately

- Check whether `expires_at` has passed (client drops expired events)
- iOS: VoIP token / PushKit present? CallKit UI belongs to **mgl-call**
- Android: cold start must call `consumePendingEvents` (`initialize` does this automatically)
- Multi-device: after one answers, send `call-cancelled` to the others

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

OK to log: `message_id` · `event_id` · `call_id` · `device_id` · `provider` · `status`  
**Do not** log: push tokens, APNs/FCM/vendor secrets, sensitive payloads.
