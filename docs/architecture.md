# Architecture

```
+-------------------- macOS host -----------------------+
|                                                       |
|  basepod-server (single Go binary)                    |
|   - HTTP :8080 (admin UI + REST + SSE)                |
|   - embed.FS:  Vue SPA (compiled web/dist)            |
|   - SQLite:    ~/BasePodData/_basepod/state.db        |
|   - AES key:   macOS Keychain                         |
|   - Talks to:  podman REST socket                     |
|                          |                            |
|                          v                            |
|  +----- podman machine (Linux VM, vfkit) ----------+  |
|  |  podman.sock                                     | |
|  |                                                  | |
|  |  network: basepod (bridge, container DNS)        | |
|  |   - caddy        :80, :443 (published to host)   | |
|  |   - app1         internal                        | |
|  |   - postgres-1   internal                        | |
|  |                                                  | |
|  |  bind mounts: ~/BasePodData/<app> -> VM          | |
|  +--------------------------------------------------+  |
+-------------------------------------------------------+

Internet  --80/443--> Mac -> Podman VM -> Caddy -> app container
```

## Components

| Package | Purpose |
|---|---|
| `cmd/basepod-server` | Entry point. Loads config, opens store, runs bootstrap, starts HTTP server. |
| `cmd/basepod` | CLI. Cobra commands talk to `/api/v1` over HTTP/SSE. |
| `internal/api` | chi router and HTTP handlers (apps, auth, deploy, logs, templates, settings, backup). |
| `internal/apps` | App service: CRUD, env (encrypted), domains, ports, volumes, versions. |
| `internal/auth` | Login (password), sessions (cookie), API tokens (bearer). |
| `internal/podman` | REST client over Unix socket (containers, images, networks, volumes, build, logs). |
| `internal/caddy` | Admin API client + JSON config renderer. |
| `internal/bootstrap` | Idempotent first-run: podman machine, network, Caddy container. |
| `internal/builder` | Tarball context → `podman build` → tag. |
| `internal/deploy` | Orchestrator. Blue/green or stop-start, healthcheck, Caddy reload. |
| `internal/templates` | YAML schema, bundled embed + remote-fetch merge. |
| `internal/crypto` | AES-GCM for env vars, bcrypt for passwords, macOS Keychain key store. |
| `internal/events` | In-process pub/sub for SSE (logs, build, deploy state). |
| `internal/store` | SQLite (modernc.org/sqlite), goose migrations, sqlc-generated queries. |
| `web/` | Vue 3 + Vite + Nuxt UI v4 SPA. Built into `web/dist/`, `go:embed`-ed. |

## Key flows

### Deploy

```
client                    server                       podman             caddy
  | POST /deploy --(tar)-> save -> podman build -tag
  |                                  start blue/green
  |                                  poll healthcheck (60s)
  |                                  render caddy json -> POST /load -----> reload
  |                                  rename old -> new, stop+remove old
  | <--- SSE: build|deployed
```

Fall back on any error: stop the new container, revert Caddy to the prior
JSON, leave the old container untouched.

### TLS

Caddy handles ACME by itself. HTTP-01 for per-app custom domains. DNS-01 for
the wildcard cert if you supply a DNS provider + token in Settings.

### Admin UI routing

The dashboard lives at `<admin_subdomain>.<root_domain>` (default `bp.`),
reverse-proxied through Caddy to the host port `:8080` via
`host.containers.internal:8080`. The root domain itself is **never** claimed
by BasePod automatically — assign it to an app via `domain add` if you want.

## Data persistence

| Data | Where | Survives container recreate | Survives `podman machine reset` | Survives Mac wipe |
|---|---|---|---|---|
| App data (bind) | `~/BasePodData/<app>/` | yes | yes | only via backup or Time Machine |
| App data (named volume) | podman VM disk | yes | no | no |
| BasePod state | `~/BasePodData/_basepod/state.db` | n/a | yes | only via backup |
| Caddy certs | podman volume `basepod-caddy-data` | n/a | no (re-acquired via ACME) | no |
| Images | podman local store | n/a | no (rebuild) | no |
| Env-var AES key | macOS Keychain | n/a | yes | no — back up separately! |

See [persistence model](persistence.md) for detail.

## Auth model

| Client | Mechanism | Header |
|---|---|---|
| Web UI | Session cookie set by `/auth/login` | `Cookie: basepod_session=…` |
| CLI / CI | Long-lived API token from `/auth/tokens` | `Authorization: Bearer bp_…` |

Tokens are stored only as `sha256` hashes server-side. Plaintext is shown
once when issued and never again.

## See also

- [Design spec](superpowers/specs/2026-05-25-basepod-design.md) — the original v1 spec.
