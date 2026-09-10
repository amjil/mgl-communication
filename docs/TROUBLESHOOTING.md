# Troubleshooting

## `/ready` returns not_ready

- `postgres: unavailable` — check `MGL_PUSH_DATABASE_URL` and migrations
- `providers: none` — provider registry has no providers
- `worker: stopped` — worker is not running

## Device registration 401

Check that `Authorization: Bearer ...` is listed in `MGL_PUSH_SERVICE_TOKENS` and that `app_id` matches the token mapping.

## Device registration 403 app_id mismatch

Request body `app_id` must match the app bound to the service token.

## Send succeeds but no notification (Phase 1)

Phase 1 uses the noop provider: it only updates delivery status and does not call a real vendor. Wait for Phase 2/3.

## Token change not synced

Confirm the client listens for `token_changed` and calls `PUT /v1/devices/{id}/token`.

## APNs: no token / 403

- Debug on a physical device with the Push capability enabled
- Check that `MGL_PUSH_APNS_*` matches the Apple key
- Use sandbox in development (`MGL_PUSH_APNS_PRODUCTION=false`)

## Huawei: no token / 80300007

- Confirm HMS Core is installed and `agconnect-services.json` is correct
- Server `MGL_PUSH_HUAWEI_APP_ID/SECRET` must be the same app as the client
- Invalid tokens are marked `devices.status=invalid`

## Xiaomi registration failure

- Confirm the host includes the MiPush SDK and correct meta-data AppID/AppKey
- Implement and register `PushMessageReceiver`, forwarding `XiaomiBridge.onRegister`
- Server uses `MGL_PUSH_XIAOMI_APP_SECRET`; `app_id` must be the Android package name

## OPPO auth failure / no notification

- Check `MGL_PUSH_OPPO_APP_KEY` / `MASTER_SECRET`
- Create the `mgl_default` (or Category) notification channel on the client
- Invalid registration_id values are marked invalid

## vivo auth / 10302

- `MGL_PUSH_VIVO_APP_ID` must be numeric
- sign = md5(appId+appKey+timestamp+appSecret) lowercase
- `10302` means invalid regId → device marked invalid

## App fails to start because of Push

This should not happen. Client `initialize` catches errors; if it still crashes, open an issue with platform logs.
