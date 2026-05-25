# Troubleshooting

## Server starts but bootstrap fails

> `podman not found in PATH`

Install via Homebrew: `brew install podman`. Then `podman machine init && podman machine start`.

> `podman REST socket not reachable after 30s`

The Podman machine is up but the REST socket isn't responding. Check:

```sh
podman system connection list
podman info --format '{{.Host.RemoteSocket.Path}}'
```

If empty: `podman machine stop && podman machine start`.

> `caddy admin endpoint not reachable`

The Caddy container started but port `2019` isn't bound. Check:

```sh
podman ps --filter name=basepod-caddy
podman logs basepod-caddy --tail 50
```

Often a port conflict with a host-installed Caddy or another local service.

## Port :80 / :443 already in use

Common with host-installed Caddy or nginx. Either:

- Stop the other listener: `brew services stop caddy`
- Or run BasePod on alt ports (would require code change — track in [#17](../docs/superpowers/specs/2026-05-25-basepod-design.md)).

## Deploy fails — `build error`

Check the SSE feed (UI Logs tab or `basepod app deploy --follow`). Most
common causes:

- Missing `Dockerfile` at the tar root → set `build.dockerfile` in
  `basepod.yaml`.
- Network errors fetching base images → retry; `podman info` shows DNS.
- ENOSPC inside the Podman VM → `podman system prune -af` then retry.

## Deploy fails — `healthcheck timeout`

Container started but never reported healthy within 60s. Either:

- The app takes longer than 60s on first boot → switch to a custom
  `healthcheck.path` that returns 200 only when warm.
- The app crashes immediately → tail `basepod app logs <name>`.

## Custom domain stuck on `tls_state=pending`

Caddy needs port `80` reachable from the public internet for HTTP-01.

- Verify DNS: `dig +short <domain>` should match the Mac's public IP.
- Verify reachability: `curl -v http://<domain>/.well-known/acme-challenge/test`
  should hit the BasePod Caddy container (404 is OK; "connection refused"
  means the port is closed).
- Check Caddy logs: `podman logs basepod-caddy --tail 100 | grep -i acme`.

For DNS-01 wildcard, set `dns_provider` + `dns_token` in Settings.

## Admin UI at `bp.example.com` is unreachable but UI on localhost works

Caddy is rendering the admin route, but `host.containers.internal` may not
resolve inside the container. Verify:

```sh
podman exec basepod-caddy nslookup host.containers.internal
```

It should resolve to the VM gateway IP. If not, recreate the Caddy
container — Podman injects this entry only at create time.

## "App name reserved" when creating an app

You tried to use the admin subdomain as an app name (default `bp`).
Either pick another app name or change `admin_subdomain` in Settings
(do this before launching apps to avoid breaking links).

## Env vars decrypt failures after restore

The AES key lives in the macOS Keychain (service `basepod`, account
`aes-key`). A `state.db` restored on a different Mac with a different
Keychain key cannot decrypt env vars. Recovery: re-enter env vars via UI
or CLI (`basepod app env set`).

## Logs

- Server log: `~/BasePodData/_basepod/logs/server.err.log`
- Caddy: `podman logs basepod-caddy`
- An app: `basepod app logs <name>` (live tail) or `podman logs <name>`

## Reset everything

```sh
launchctl unload ~/Library/LaunchAgents/dev.basepod.server.plist
rm -rf ~/BasePodData                    # wipes state, encrypted env, app data
podman machine rm -f                    # nuke the VM
security delete-generic-password -s basepod -a aes-key  # forget the key
```

Then re-run the installer.
