# Getting Started

BasePod is a single Go binary that drives [Podman](https://podman.io) and
[Caddy](https://caddyserver.com) on macOS to give you a one-click deploy
experience similar to [CapRover](https://caprover.com).

## Requirements

- Apple Silicon Mac (`darwin/arm64`)
- macOS 13+
- `podman` 5+; the installer can install it through Homebrew when missing
- A public IP and a domain pointing at it for public app subdomains and TLS
- Ports `80` and `443` open on the host for public app traffic

## Install

Full server + CLI install:

```sh
curl -fsSL https://raw.githubusercontent.com/flakerimi/basepod/main/scripts/install.sh | sh
```

This:

1. Prompts to install Podman through Homebrew when Podman is missing
2. Installs `basepod-server` and `basepod` into `/usr/local/bin/`
3. Writes a launchd plist at `~/Library/LaunchAgents/dev.basepod.server.plist`
4. Loads the agent — the server stays running across reboots

To install only the CLI for managing an existing BasePod server:

```sh
curl -fsSL https://raw.githubusercontent.com/flakerimi/basepod/main/scripts/install-cli.sh | sh
```

The CLI-only installer does not install Podman, start the server, or register a
launchd service.

Or build from source:

```sh
git clone https://github.com/flakerimi/basepod
cd basepod
make build && make web
./bin/basepod-server
```

## First boot

On first run the server:

1. Detects `podman` and starts the `podman machine` if it isn't running
2. Creates the `basepod` Podman network
3. Pulls and starts a Caddy container, publishing host ports `:80` and `:443`;
   Caddy admin stays off TCP and is controlled through an internal Unix socket
   via `podman exec`
4. Leaves admin setup pending unless `BASEPOD_ADMIN_PASSWORD` is set

To provision the admin user from the shell instead of the browser:

```sh
launchctl unload ~/Library/LaunchAgents/dev.basepod.server.plist
BASEPOD_ADMIN_PASSWORD=secret /usr/local/bin/basepod-server &
```

## Open the dashboard

Visit <http://localhost:8080>. On first run, create the admin user in the
browser.

## Point a domain

In Settings, set:

| Field | Example |
|---|---|
| **Root domain** | `example.com` |
| **Admin subdomain** | `bp` (default) |
| **ACME email** | `you@example.com` |

DNS records:

```
example.com       A    <mac mini public ip>
*.example.com     A    <mac mini public ip>
```

After that, the dashboard moves to `https://bp.example.com` and apps get
`https://<app>.example.com`. Caddy obtains certs automatically.

## Log in via the CLI

```sh
basepod login --server https://bp.example.com
# enter admin / your password
```

`basepod` issues an API token and writes it to `~/.basepod/config.yaml`.

## Deploy your first app

The simplest form is an OCI image you already trust:

```sh
basepod app create hello --image nginxdemos/hello --port 80
basepod app deploy hello --image nginxdemos/hello
```

In the dashboard the app appears under **Apps**. Visit
`https://hello.example.com` once DNS propagates and Caddy issues the cert.

To deploy from a directory containing a `Dockerfile`:

```sh
basepod app create my-api --port 3000
basepod app deploy my-api .
basepod app logs my-api          # SSE tail
```

## Install a database from a template

```sh
basepod template list
basepod template install postgres my-db POSTGRES_PASSWORD=secret
```

Bundled templates: Postgres, MySQL, Redis, MongoDB, MinIO, Meilisearch,
Mailpit, Adminer, Uptime Kuma.

Templates default to `internal_only: true` — no Caddy route — so the DB is
only reachable inside the `basepod` Podman network (use the container DNS
name, e.g. `my-db:5432`, from another app).

## Back up

```sh
basepod backup -o backup.tar.gz
```

Tar contains `state.db`, the rendered `caddy.json`, and the
`~/BasePodData/<app>/...` data tree (excluding internal `_basepod/`).

## Next

- [Architecture](architecture.md)
- [CLI Reference](cli/basepod.md)
- [Templates](templates.md)
- [Troubleshooting](troubleshooting.md)
