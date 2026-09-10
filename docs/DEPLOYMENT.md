# Deployment

## Docker Compose (recommended for development / small scale)

```bash
docker compose up --build -d
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `MGL_PUSH_ENV` | `development` / `production` |
| `MGL_PUSH_HTTP_ADDR` | Default `:8080` |
| `MGL_PUSH_DATABASE_URL` | Postgres DSN |
| `MGL_PUSH_SERVICE_TOKENS` | `token:app_id,token2:app_id2` |
| `MGL_PUSH_MAX_ATTEMPTS` | Default 5 |
| `MGL_PUSH_WORKER_POLL_MS` | Default 1000 |
| `MGL_PUSH_FCM_CREDENTIALS_FILE` | Firebase Admin SDK JSON (Phase 2) |
| `MGL_PUSH_APNS_TEAM_ID` | Apple Team ID |
| `MGL_PUSH_APNS_KEY_ID` | APNs Key ID |
| `MGL_PUSH_APNS_BUNDLE_ID` | App Bundle ID (apns-topic) |
| `MGL_PUSH_APNS_PRIVATE_KEY` | Path to `.p8` or PEM contents |
| `MGL_PUSH_APNS_PRODUCTION` | `true`/`false` (defaults follow ENV) |
| `MGL_PUSH_HUAWEI_APP_ID` | AppGallery Connect App ID |
| `MGL_PUSH_HUAWEI_APP_SECRET` | App Secret |
| `MGL_PUSH_XIAOMI_APP_SECRET` | MiPush AppSecret |
| `MGL_PUSH_XIAOMI_PACKAGE_NAME` | Optional package-name fallback |
| `MGL_PUSH_OPPO_APP_KEY` | OPPO AppKey |
| `MGL_PUSH_OPPO_MASTER_SECRET` | OPPO MasterSecret |
| `MGL_PUSH_VIVO_APP_ID` | vivo numeric appId |
| `MGL_PUSH_VIVO_APP_KEY` | vivo appKey |
| `MGL_PUSH_VIVO_APP_SECRET` | vivo appSecret |

Never commit secrets to Git, Flutter, ClojureDart, the database, or plain logs.

## Production recommendations

```text
Internet → TLS (Caddy/Nginx) → mgl-push-server → PostgreSQL
```

Require HTTPS; give each business service its own service token.
