# BasePod

CapRover-like PaaS for **macOS** powered by Podman + Caddy.

**Status:** WIP — v1 in development.

## What

Deploy and manage container-based services (web apps, databases, workers) on a single Mac. BasePod ships as two binaries: `basepod-server` runs the local control plane, and `basepod` is the CLI for day-to-day operations.

Inspired by [CapRover](https://caprover.com/), which does not run on macOS.

## Highlights

- Deploy from tarball, Dockerfile, OCI image, or one-click templates
- Automatic HTTPS via Caddy (Let's Encrypt HTTP-01 or DNS-01 wildcard)
- Blue/green deploys, per-app env vars (encrypted at rest), persistent volumes
- Single admin user + API tokens for CLI/CI
- Bundled + remote one-click app templates (Postgres, Redis, etc.)
- Single server binary with embedded Vue SPA

## Docs

- [Getting started](docs/getting-started.md)
- [Architecture](docs/architecture.md)
- [API reference](docs/api.md)
- [CLI reference](docs/cli/basepod.md)
- [Templates](docs/templates.md)
- [Persistence](docs/persistence.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Design spec](docs/superpowers/specs/2026-05-25-basepod-design.md)

## Build

```
make build       # builds bin/basepod-server and bin/basepod
make test
make run         # runs the server locally
```

Requires Go 1.26 for source builds and an Apple Silicon Mac. The installer can
install Podman 5+ through Homebrew when it is missing.

## Install

Apple Silicon Mac (arm64) only.

Full server + CLI install:

```
curl -fsSL https://raw.githubusercontent.com/flakerimi/basepod/main/scripts/install.sh | sh
```

CLI-only install, for managing an existing BasePod server:

```
curl -fsSL https://raw.githubusercontent.com/flakerimi/basepod/main/scripts/install-cli.sh | sh
```

Or build from source:

```
make build && make web
./bin/basepod-server
```

The installer prints the dashboard URL when it finishes. If you install over
SSH, use that printed `http://<server-ip>:<port>` URL from your browser. The
installer chooses the first free port in `8080-8090`; override it with
`BASEPOD_HTTP_ADDR=:9090` if you want a specific port.

On first run, create the admin user in the browser.

The installer places two binaries in `/usr/local/bin`:

- `basepod-server`: the launchd-managed server and dashboard
- `basepod`: the CLI client that talks to the server API

The CLI-only installer places only `basepod` in `/usr/local/bin`; it does not
install Podman, start the server, or register launchd.

## CLI

After creating the admin user in the browser, log in from the terminal:

```sh
basepod login --server http://<server-ip-or-domain>:<port>
```

This stores the server URL and an API token in `~/.basepod/config.yaml`.
Subsequent commands use that token automatically.

Common commands:

```sh
basepod app list
basepod app create my-api --port 3000
basepod app deploy my-api .
basepod app deploy my-api --image ghcr.io/me/my-api:1.2.0 --follow
basepod app logs my-api
basepod app env set my-api DATABASE_URL=postgres://...
basepod app rollback my-api <version>

basepod settings set root_domain=example.com acme_email=ops@example.com
basepod domain add my-api api.example.com

basepod template list
basepod template install postgres my-db POSTGRES_PASSWORD=secret

basepod backup -o basepod-backup.tar.gz
basepod logout
```

Use `basepod --help` or the [CLI reference](docs/cli/basepod.md) for the full
command list.

## Project layout

```
cmd/basepod-server/    Go server entrypoint
cmd/basepod/           Go CLI entrypoint
internal/              Server packages
templates/             (legacy) — see internal/templates/bundled/
web/                   Vue 3 SPA (built into bin via go:embed)
scripts/               build.sh, install.sh
```

## License

TBD.
