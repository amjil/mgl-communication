# API

Base: `/v1`  
Auth: `Authorization: Bearer <service-token>`

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness |
| GET | `/ready` | Postgres + providers + worker |
| POST | `/v1/devices` | Register device |
| PUT | `/v1/devices/{installation_id}/token` | Update token |
| PUT | `/v1/devices/{installation_id}/user` | Bind user |
| DELETE | `/v1/devices/{installation_id}/user` | Unbind user |
| DELETE | `/v1/devices/{installation_id}` | Deregister (`status=disabled`) |
| POST | `/v1/messages` | Send (`user_ids` / `installation_ids` / `provider`+`token`) |

## Error format

```json
{ "error": { "code": "DEVICE_NOT_FOUND", "message": "Device not found" } }
```

## Data payload

`string → string` only. Clients should fetch complex data from business APIs in a follow-up request.
