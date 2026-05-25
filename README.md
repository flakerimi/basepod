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

## Install (planned)

```
curl https://get.basepod.dev | sh
```

## License

TBD.
