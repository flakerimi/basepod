# BasePod

CapRover-like PaaS for **macOS** powered by Podman + Caddy.

**Status:** WIP — v1 in development.

## What

Deploy and manage container-based services (web apps, databases, workers) on a single Mac. Single Go binary that talks to `podman machine`, drives a Caddy reverse proxy, and exposes a Vue dashboard plus a `basepod` CLI.

Inspired by [CapRover](https://caprover.com/), which does not run on macOS.

## Highlights

- Deploy from tarball, Dockerfile, OCI image, or one-click templates
- Automatic HTTPS via Caddy (Let's Encrypt HTTP-01 or DNS-01 wildcard)
- Blue/green deploys, per-app env vars (encrypted at rest), persistent volumes
- Single admin user + API tokens for CLI/CI
- Bundled + remote one-click app templates (Postgres, Redis, etc.)
- Single binary; embedded Vue SPA

## Design

See [`docs/superpowers/specs/2026-05-25-basepod-design.md`](docs/superpowers/specs/2026-05-25-basepod-design.md).

## Build

```
make build       # builds bin/basepod-server and bin/basepod
make test
make run         # runs the server locally
```

Requires Go 1.26, Podman 5+, Apple Silicon Mac.

## Install

Apple Silicon Mac (arm64) only.

```
curl -fsSL https://raw.githubusercontent.com/flakerimi/basepod/main/scripts/install.sh | sh
```

Or build from source:

```
make build && make web
./bin/basepod-server
```

Then open <http://localhost:8080> to log in (the admin password is printed on
first run unless `BASEPOD_ADMIN_PASSWORD` is set).

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
