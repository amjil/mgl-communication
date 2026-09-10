# Providers

Canonical names: `huawei` | `xiaomi` | `oppo` | `vivo` | `fcm` | `apns`

## Phase status

| Provider | Client | Server |
|----------|--------|--------|
| FCM | ✅ | ✅ |
| APNs | ✅ | ✅ |
| Huawei | ✅ | ✅ |
| Xiaomi | ✅ | ✅ |
| OPPO | ✅ | ✅ |
| vivo | Phase 7 ✅ | Phase 7 ✅ |

## vivo (Phase 7)

### Server

```bash
export MGL_PUSH_VIVO_APP_ID=<numeric appId>
export MGL_PUSH_VIVO_APP_KEY=<appKey>
export MGL_PUSH_VIVO_APP_SECRET=<appSecret>
```

- Auth: `POST /message/auth`, `sign = md5(appId+appKey+timestamp+appSecret)` (lowercase)
- Single push: `POST /message/send`, Header `authToken`, body includes `regId`
- Prefer RegId (no alias/tags server-side routing in the first stage)

### Android Client

1. Integrate the vivo Push SDK
2. meta-data: `VIVO_APP_ID` / `VIVO_APP_KEY` (or official `com.vivo.push.app_id` / `com.vivo.push.api_key`)
3. `OpenClientPushMessageReceiver` → `VivoBridge` (see `MglVivoPushDocs.kt`)

## Other

Full environment variables are in `.env.example`.
