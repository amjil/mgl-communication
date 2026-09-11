# Providers

Canonical names: `fcm` · `apns` · `apns_voip` · `huawei` · `xiaomi` · `oppo` · `vivo`

When credentials are missing, **noop** is registered (fake success) and startup is not blocked. Production must use real credentials.

## Phase matrix (spec 2.1)

| Provider | Spec phase | Client | Server |
|----------|------------|--------|--------|
| FCM | Phase 1 | ✅ | ✅ |
| APNs / `apns_voip` | Phase 1 | ✅ PushKit | ✅ |
| Huawei | Phase 2 | ✅ HMS SDK | ✅ |
| Xiaomi | Phase 2 | ✅ reflection + host Receiver | ✅ |
| OPPO | Phase 3 | ✅ reflection Bridge | ✅ |
| vivo | Phase 3 | ✅ reflection Bridge | ✅ |

## FCM

```bash
export MGL_PUSH_FCM_CREDENTIALS_FILE=/path/to/firebase-adminsdk.json
```

- HTTP v1 Admin SDK
- data-only / notification decided by `Message.Type`
- Incoming Call: `priority=high`, no collapse

## APNs / VoIP

```bash
export MGL_PUSH_APNS_TEAM_ID=…
export MGL_PUSH_APNS_KEY_ID=…
export MGL_PUSH_APNS_BUNDLE_ID=com.example.app
export MGL_PUSH_APNS_PRIVATE_KEY=/path/to/AuthKey.p8
export MGL_PUSH_APNS_PRODUCTION=false
```

| Provider name | Topic | push-type |
|---------------|-------|-----------|
| `apns` | Bundle ID | `alert` / `background` |
| `apns_voip` | Bundle ID + `.voip` | `voip` |

VoIP tokens from client PushKit should register as `apns_voip` (or update via `token_changed`).

## Huawei (Phase 2)

```bash
export MGL_PUSH_HUAWEI_APP_ID=…
export MGL_PUSH_HUAWEI_APP_SECRET=…
```

- OAuth client_credentials → `messages:send`
- Data-only / incoming-call still set `android.urgency=HIGH` and `ttl`
- Client: `agconnect-services.json` + HMS Core

## Xiaomi (Phase 2)

```bash
export MGL_PUSH_XIAOMI_APP_SECRET=…
export MGL_PUSH_XIAOMI_PACKAGE_NAME=net.amjil.nomio   # required: Android package name
```

- `restricted_package_name` **prefers** `MGL_PUSH_XIAOMI_PACKAGE_NAME`
- Fall back only when device `app_id` looks like an Android package
- pass_through=1 for silent / incoming-call
- Host: MiPush AAR + meta-data + [`AppMiPushReceiver` example](../mgl-push-client/android/examples/AppMiPushReceiver.kt)

## OPPO (Phase 3)

```bash
export MGL_PUSH_OPPO_APP_KEY=…
export MGL_PUSH_OPPO_MASTER_SECRET=…
```

- Unicast notification API; pure data gets a synthesized notification shell with business fields in `action_parameters`
- Default channel: `mgl_default`
- Incoming-call title/body prefer `caller_display_name` / `caller_id`

## vivo (Phase 3)

```bash
export MGL_PUSH_VIVO_APP_ID=<numeric>
export MGL_PUSH_VIVO_APP_KEY=…
export MGL_PUSH_VIVO_APP_SECRET=…
```

- Auth: `sign = md5(appId+appKey+timestamp+appSecret)` (lowercase)
- Unicast by RegId; incoming call likewise synthesizes a notification shell + `clientCustomMap`

## Capability notes

| Capability | Notes |
|------------|-------|
| `notification` | All vendors |
| `silent` / `background` | Subject to OS policy; execution not guaranteed |
| `incoming_call` | iOS uses VoIP; Android high-priority data / vendor channel; **not** the same as CallKit availability |

Full env vars: [DEPLOYMENT.md](DEPLOYMENT.md) and [`.env.example`](../.env.example).
