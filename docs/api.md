# API Reference

All endpoints are under `/api/v1` and accept JSON unless noted. Two clients
share the same surface: the embedded Vue SPA (cookie auth) and the `basepod`
CLI (bearer-token auth).

## Auth

Send **either**:

```
Cookie: basepod_session=<sid>
```

…or:

```
Authorization: Bearer bp_<token>
```

`/api/v1/health` and `/api/v1/auth/login` are unauthenticated.

## Endpoints

### Health

```
GET /api/v1/health
→ { status, version, time, podman?, caddy? }
```

### Auth

```
POST /api/v1/auth/login           { username, password }   → sets cookie
POST /api/v1/auth/logout
GET  /api/v1/auth/me              → { user }
POST /api/v1/auth/tokens          { name }                 → { token, name }
GET  /api/v1/auth/tokens          → { tokens: [...] }
DELETE /api/v1/auth/tokens/:id
```

### Apps

```
GET    /api/v1/apps
POST   /api/v1/apps               { name, image_repo?, instances?, ports?, volumes?, ... }
GET    /api/v1/apps/:name
PATCH  /api/v1/apps/:name         { instances?, deploy_strategy?, healthcheck_path?, ... }
DELETE /api/v1/apps/:name

POST   /api/v1/apps/:name/deploy
   multipart: tar=@build.tar  appfile=@basepod.yaml(optional)
   or application/json: { "image": "ghcr.io/.../foo:tag" }
POST   /api/v1/apps/:name/restart
POST   /api/v1/apps/:name/rollback { version }

GET    /api/v1/apps/:name/logs    (SSE — event-stream of stdout/stderr lines)
GET    /api/v1/apps/:name/versions
GET    /api/v1/apps/:name/env
PUT    /api/v1/apps/:name/env     { env: { K: V, ... } }

POST   /api/v1/apps/:name/domains  { domain }
DELETE /api/v1/apps/:name/domains/:domain
```

### Templates

```
GET    /api/v1/templates                                    → { templates: [...] }
POST   /api/v1/templates/install   { template_id, app_name, fields }
```

### Settings

```
GET    /api/v1/settings                                     → { settings: { ... } }
PUT    /api/v1/settings            { root_domain?, acme_email?, dns_provider?, dns_token?, admin_subdomain? }
```

Allowed keys: `root_domain`, `acme_email`, `dns_provider`, `dns_token`,
`admin_subdomain`. `dns_token` is masked as `***` on read.

### Events (SSE)

```
GET    /api/v1/events                  → server-sent events
```

Event types include:

- `started`, `built`, `deployed`, `failed` (topic `app:<name>:deploy`)
- `log` (build output)
- `rollback_started`

### Backup

```
POST   /api/v1/backup              → streams Content-Type: application/gzip
POST   /api/v1/restore             → 501 (manual: stop server, swap state.db, restart)
```

## Error shape

4xx and 5xx return:

```json
{ "error": "human message", "code": "machine_key", "status": 400 }
```

Common codes: `bad_request`, `not_found`, `name_in_use`, `invalid_credentials`,
`podman_unavailable`, `template_not_found`, `version_not_found`,
`not_implemented`.

## Versioning

`/api/v1` is the only version. A future breaking change would add `/api/v2`
side-by-side.
