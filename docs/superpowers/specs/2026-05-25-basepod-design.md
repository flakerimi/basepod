# BasePod — Design Spec

**Date:** 2026-05-25
**Status:** Draft (awaiting review)

## 1. Purpose

BasePod is a CapRover-like Platform-as-a-Service for **macOS**. It manages
container-based services (web apps, databases, workers) using **Podman**
and routes traffic via **Caddy**, with a Go backend and Vue 3 frontend.

CapRover does not run on macOS; BasePod fills that gap for developers and
homelab users who want a one-click deploy experience on a Mac.

## 2. Scope

### In scope (v1)

- Single macOS host (no clustering).
- Deploy from: tarball upload, Dockerfile-in-repo, pre-built OCI image,
  Git webhook, one-click templates.
- Internal apps (private) and public apps with auto-HTTPS via Caddy.
- Public root domain (e.g. `example.com` with `*.example.com` → host IP).
  Apps get `<app>.<root>` automatically; custom domains attachable.
- Blue/green deploy (default) and stop/start (configurable per app).
- Persistent storage via bind mount to `~/BasePodData/<app>/` by default;
  named podman volumes opt-in for perf-sensitive workloads (DBs).
- Web UI (Vue 3 + Nuxt UI, non-green palette) + CLI (`basepod`) sharing
  the same REST/SSE API.
- Single admin user with password login (UI) and API tokens (CLI/CI).
- One-click app templates: bundled set + optional remote source merge.
- Backup/restore of state, data, and Caddy config.

### Out of scope (v1)

- Multi-host clustering, Swarm-equivalent orchestration.
- Internal image registry (rely on podman local store; rollback via tag
  retention — last 5 versions per app).
- Persistent log storage / search (live tail only, via `podman logs -f`).
- Metrics dashboards.
- CI/webhook integrations beyond the CLI surface.
- Marketplace UI for templates.

### Future opt-ins (not v1, but design door open)

- Multi-host (add remote podman hosts via SSH or REST).
- Vue Vapor mode (blocked on Nuxt UI Vapor support).
- DNS-01 wildcard certs (already supported in design; enable when user
  provides DNS provider creds).

## 3. Architecture

```
+-------------------- macOS host -----------------------+
|                                                       |
|  basepod-server (Go binary, single artifact)          |
|   - HTTP :8080  (admin UI + REST + SSE)               |
|   - embed.FS:   Vue SPA dist                          |
|   - SQLite:     ~/BasePodData/_basepod/state.db       |
|   - AES key:    macOS Keychain (service "basepod")    |
|   - Talks to -> podman REST socket                    |
|                          |                            |
|                          v                            |
|  +----- podman machine (Linux VM) -----------------+  |
|  |  podman.sock (REST API)                          | |
|  |                                                  | |
|  |  network: basepod (bridge, DNS by container name)| |
|  |   - caddy        :80, :443 (published to host)   | |
|  |   - app1         (internal only)                 | |
|  |   - app2         (internal only)                 | |
|  |   - postgres-x   (internal only)                 | |
|  |                                                  | |
|  |  volumes: bind ~/BasePodData/<app> -> VM         | |
|  |  (via `podman machine init --volume`)            | |
|  +--------------------------------------------------+  |
|                                                       |
+-------------------------------------------------------+

Browser/CLI ---REST/SSE---> :8080
Internet    ---80/443----> macOS -> podman machine -> caddy -> app
```

### Key flows (summary)

- **Deploy**: client POSTs tar -> server saves -> `podman build` via
  socket -> tags `basepod/<app>:<sha>` -> creates new container on
  `basepod` net -> healthcheck -> regenerates Caddy config -> reloads
  via admin API -> stops old container.
- **TLS**: Caddy ACME (HTTP-01 default; DNS-01 if user configures a DNS
  provider in settings).
- **Logs**: SSE endpoint streams `podman logs -f <container>`.

## 4. Components (Go server)

```
basepod-server/
+- cmd/basepod-server/main.go
+- internal/
|  +- api/           # HTTP handlers (chi router)
|  |  +- auth.go     # login, token issue/revoke
|  |  +- apps.go     # CRUD, scale, restart, env
|  |  +- deploy.go   # tar upload, build, swap
|  |  +- logs.go     # SSE stream
|  |  +- domains.go  # attach/detach, TLS state
|  |  +- templates.go# list, install one-click
|  |  +- system.go   # health, version, settings
|  +- podman/        # REST client wrapper
|  +- caddy/         # config render + admin API client (:2019)
|  +- store/         # sqlc-generated queries, goose migrations
|  +- crypto/        # AES-GCM env vars, keychain via go-keyring
|  +- builder/       # tar extract -> build -> tag -> version mgmt (keep N=5)
|  +- deploy/        # orchestrator: blue/green, healthcheck, rollback
|  +- templates/     # bundled YAML (embed) + remote fetch + render
|  +- auth/          # session + token + middleware
|  +- events/        # SSE hub (logs, build output, state changes)
|  +- bootstrap/     # first-run: podman machine, network, Caddy
+- web/              # Vue SPA source (built -> dist/ -> go:embed)
```

Cross-cutting: structured logging (`slog`), context propagation,
request ID middleware, graceful shutdown.

## 5. API surface

Single REST API serves both Vue SPA and `basepod` CLI. Auth middleware
accepts **either** a session cookie (UI) or `Authorization: Bearer
<token>` (CLI).

```
GET  /                                # Vue SPA (embedded)
GET  /assets/*                        # SPA assets

GET  /api/v1/health
POST /api/v1/auth/login               # sets cookie
POST /api/v1/auth/logout
POST /api/v1/auth/tokens              # issue API token
GET  /api/v1/auth/tokens
DELETE /api/v1/auth/tokens/:id

GET  /api/v1/apps
POST /api/v1/apps
GET  /api/v1/apps/:name
PATCH /api/v1/apps/:name              # scale, strategy, healthcheck, etc.
DELETE /api/v1/apps/:name
POST /api/v1/apps/:name/deploy        # multipart: tar + basepod.yaml
POST /api/v1/apps/:name/restart
POST /api/v1/apps/:name/rollback      # body: { version }
GET  /api/v1/apps/:name/logs          # SSE
GET  /api/v1/apps/:name/env
PUT  /api/v1/apps/:name/env           # full replace
GET  /api/v1/apps/:name/versions

POST /api/v1/apps/:name/domains       # attach
DELETE /api/v1/apps/:name/domains/:domain

GET  /api/v1/templates                # bundled + remote merged
POST /api/v1/templates/install        # body: { template_id, app_name, fields }

GET  /api/v1/settings
PUT  /api/v1/settings                 # root_domain, acme_email, dns_*

GET  /api/v1/events                   # SSE: build, deploy, app state
POST /api/v1/backup                   # produces tar
POST /api/v1/restore                  # multipart tar
```

## 6. Data model (SQLite via sqlc + goose)

```sql
users(id, username, password_hash, created_at, updated_at)
tokens(id, user_id, name, hash, last_used_at, created_at, revoked_at)
sessions(id, user_id, expires_at, created_at)

apps(
  id, name UNIQUE, image_repo, current_version,
  instances INT DEFAULT 1,
  deploy_strategy TEXT DEFAULT 'blue_green',   -- 'stop_start' | 'blue_green'
  healthcheck_path TEXT, healthcheck_cmd TEXT,
  internal_only BOOLEAN DEFAULT 0,
  created_at, updated_at
)
app_versions(id, app_id, version, image_tag, status, deployed_at, log_excerpt)
app_env(id, app_id, key, value_encrypted)        -- AES-GCM
app_volumes(id, app_id, container_path, host_path, named_volume)
app_ports(id, app_id, container_port, protocol)
app_domains(id, app_id, domain UNIQUE, is_primary, tls_state)
app_resource_limits(app_id, memory_mb, cpu_pct)

templates_installed(id, template_id, app_name, params_json, installed_at)
settings(key UNIQUE, value)
   -- keys: root_domain, acme_email, dns_provider, dns_token_enc,
   --       template_sources_json, image_retention_count
audit_log(id, user_id, action, target, payload_json, created_at)
```

Migrations live in `internal/store/migrations/*.sql`, applied on boot.

## 7. App definition (`basepod.yaml`)

Lives in repo root or tarball root. Schema (v1):

```yaml
name: myapp                   # optional; CLI/UI can override
build:
  type: dockerfile            # dockerfile | image | tarball
  dockerfile: ./Dockerfile    # only for type=dockerfile
  image: ghcr.io/me/app:tag   # only for type=image
env:
  NODE_ENV: production
ports:
  - 3000
volumes:
  - container: /data
    host: ~/BasePodData/myapp/data    # default if omitted
healthcheck:
  path: /healthz
  interval: 10s
  timeout: 3s
  retries: 3
instances: 1
deploy_strategy: blue_green
resources:
  memory_mb: 512
  cpu_pct: 50
```

Server-side overrides (env, domains) take precedence over file values.

## 8. Deploy flow

```
client                    server                       podman             caddy
  |                         |                            |                   |
  |  POST /deploy ---->     |                            |                   |
  |   (tar + basepod.yaml)  |                            |                   |
  |                         | save tar -> workdir        |                   |
  |                         | parse basepod.yaml         |                   |
  |  <-- SSE: build started |                            |                   |
  |                         |                            |                   |
  |                         | POST /libpod/build ------> |                   |
  |                         |   tag basepod/<n>:<sha>    |                   |
  |  <-- SSE: build logs    | <-- stream logs ---------- |                   |
  |                         |                            |                   |
  |                         | create container <n>-new ->|                   |
  |                         |   net=basepod              |                   |
  |                         |   env (decrypt+inject)     |                   |
  |                         |   volumes bind             |                   |
  |                         | start <n>-new ------------>|                   |
  |                         |                            |                   |
  |                         | poll healthcheck (60s)     |                   |
  |                         |   fail -> rollback         |                   |
  |                         |                            |                   |
  |                         | render Caddy JSON (upstream=<n>-new)           |
  |                         | POST :2019/load ---------------------------->  |
  |                         |                            |                   |
  |                         | rename <n>   -> <n>-old    |                   |
  |                         | rename <n>-new -> <n>      |                   |
  |                         | stop+rm <n>-old ---------->|                   |
  |                         | record app_versions row    |                   |
  |  <-- SSE: deployed      |                            |                   |
```

- **Rollback**: healthcheck or Caddy reload failure -> stop `<n>-new`,
  keep `<n>` running, revert Caddy JSON, return error to client.
- **Stop_start strategy**: stop old, start new with original name, reload
  Caddy. Brief downtime.
- **Image retention**: keep last 5 image tags per app. Older tags pruned
  via `podman rmi`. Rollback re-deploys an existing tag.

## 9. Routing & TLS (Caddy)

Caddy receives config via admin API (`:2019/load`, bound to host
loopback). Config is JSON, generated by Go `text/template`. Atomic;
revert on failure.

Example rendered Caddyfile-equivalent (shown as Caddyfile for clarity):

```
{
    email {$ACME_EMAIL}
    admin 0.0.0.0:2019
}

*.example.com {
    tls {
        dns {$DNS_PROVIDER} {$DNS_TOKEN}    # only if configured
    }
    @app1 host app1.example.com
    handle @app1 { reverse_proxy app1:3000 }
}

shop.acme.com {
    reverse_proxy app1:3000                  # HTTP-01 ACME
}
```

DNS-01 wildcard is opt-in via settings; HTTP-01 per-app is the default.

## 10. One-click app templates

Bundled YAML files embedded in the binary; remote sources fetched and
merged at runtime. Example:

```yaml
id: postgres
name: PostgreSQL
version: "16"
description: Relational database
fields:
  - key: POSTGRES_PASSWORD
    label: Password
    type: password
    required: true
  - key: POSTGRES_DB
    label: Database name
    default: app
deploy:
  image: docker.io/library/postgres:16
  env:
    POSTGRES_PASSWORD: "{{.POSTGRES_PASSWORD}}"
    POSTGRES_DB: "{{.POSTGRES_DB}}"
  volumes:
    - container: /var/lib/postgresql/data
      host: ~/BasePodData/{{.app_name}}/pgdata
  ports:
    - 5432
  internal_only: true
  healthcheck:
    cmd: ["pg_isready", "-U", "postgres"]
```

Remote sources are URLs to YAML index files; configurable in settings.

## 11. Bootstrap (first run)

Idempotent. Re-running is safe.

1. Ensure `podman` binary exists. Otherwise emit install hint.
2. Ensure `podman machine` exists. Otherwise run
   `podman machine init --volume ~/BasePodData:/BasePodData`.
3. Ensure machine running. Otherwise `podman machine start`.
4. Ensure podman REST socket reachable.
5. Ensure network `basepod` exists. Otherwise create.
6. Ensure caddy container exists and running. Otherwise pull
   `docker.io/library/caddy:2-alpine` and create with:
   - `--network basepod`
   - publish `80:80`, `443:443`, `127.0.0.1:2019:2019`
   - volume `basepod_caddy_data` (certs persist)
7. Ensure admin user exists. Otherwise prompt password on stdin or
   auto-generate (printed once, then forgotten).
8. Start HTTP server on `:8080`.

## 12. Frontend stack

- Vue 3 + Vite + TypeScript + Pinia + Vue Router.
- Nuxt UI as the component library.
- **Brand color: `#00C0E8`** (cyan). Nuxt UI primary set to a custom
  palette derived from this hex (configured via `app.config.ts`
  `ui.colors.primary`).
- Logo: `assets/logo.svg` (wordmark, single-color, fill `#00C0E8`).
- REST via `fetch` wrapper with token/cookie injection.
- SSE via `EventSource` for logs and live events.
- Built artifact -> `web/dist/` -> `go:embed` into binary.

## 13. CLI (`basepod`)

```
basepod login [--server URL]                # interactive password -> token
basepod logout
basepod tokens list | create <name> | revoke <id>

basepod app list
basepod app create <name> [--image ...]
basepod app show <name>
basepod app deploy <path>                   # path = dir or .tar.gz
basepod app logs <name> [-f]
basepod app restart <name>
basepod app scale <name> --instances N
basepod app env set <name> KEY=VAL [...]
basepod app env unset <name> KEY [...]
basepod app rollback <name> --version SHA
basepod app destroy <name>

basepod domain add <app> <fqdn>
basepod domain rm <app> <fqdn>

basepod template list
basepod template install <id> <app-name> [--field K=V ...]

basepod backup [--out path]
basepod restore <path>
```

Same binary works against any BasePod server via `--server` flag.

## 14. Error handling

- 4xx for user errors (with `{error, code, hint}` JSON).
- 5xx for system errors (logged with request ID).
- Deploy failures -> SSE event + `app_versions.status='failed'` with
  trimmed stderr.
- Caddy reload failure -> revert in-memory prior JSON and re-POST.
- Podman socket disconnect -> reconnect with backoff; mark system
  unhealthy in `/health`.
- Panic-recovery middleware on all handlers -> 500 + audit log entry.

## 15. Testing strategy

- **Unit**: store repos, Caddy config renderer, `basepod.yaml` parser,
  env crypto, template renderer.
- **Integration**: real `podman` on CI macOS runner. End-to-end deploy
  a tiny `hello` image, hit Caddy, assert 200.
- **API**: `httptest` server + sqlite `:memory:`. Auth, CRUD, deploy
  path with mocked podman client interface.
- **Frontend**: Vitest for stores; Playwright for one smoke flow
  (login -> create app -> deploy mock -> see logs).

## 16. Distribution & target

- **Distribution (v1)**: `curl https://get.basepod.dev | sh` install
  script. Downloads the latest darwin/arm64 binary, places it at
  `/usr/local/bin/basepod-server` and `/usr/local/bin/basepod`, then
  installs a launchd plist at
  `~/Library/LaunchAgents/dev.basepod.server.plist` to run as a user
  agent and restart on crash. Brew formula and `.pkg` deferred.
- **Architecture (v1)**: **darwin/arm64 only** (Apple Silicon). Intel
  Mac support deferred. CI builds a single arm64 binary; `go build`
  with `GOOS=darwin GOARCH=arm64`.

## 17. Open questions

- Exact launchd plist contents (RunAtLoad, KeepAlive, StandardOutPath,
  EnvironmentVariables) — finalize during implementation.

## 18. Repository layout

```
BasePod/
+- cmd/basepod-server/
+- cmd/basepod/                  # CLI
+- internal/
+- web/                          # Vue SPA
|  +- src/
|  +- package.json
|  +- vite.config.ts
+- templates/bundled/*.yaml
+- docs/superpowers/specs/
+- Makefile
+- go.mod
+- README.md
```

## 19. Dependencies (Go)

- `github.com/go-chi/chi/v5` — router
- `modernc.org/sqlite` — CGO-free SQLite
- `github.com/sqlc-dev/sqlc` — codegen (build-time)
- `github.com/pressly/goose/v3` — migrations
- `github.com/zalando/go-keyring` — macOS Keychain
- `github.com/containers/podman/v5/...` REST (or thin custom client)
- `gopkg.in/yaml.v3` — YAML
- `github.com/golang-jwt/jwt/v5` — token signing (HMAC)
- `github.com/spf13/cobra` — CLI

## 20. Versioning

- SemVer for the binary.
- API versioned at `/api/v1`. Future breaking changes get `/api/v2`.
- `basepod.yaml` schema versioned via an optional `schema: 1` field
  (defaults to 1 if omitted).
