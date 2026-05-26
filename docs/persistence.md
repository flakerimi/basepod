# Persistence

Three layers of data, three lifecycles. Know which is which before you
`podman machine reset`.

## 1. App user data — bind-mount to host (default)

In `basepod.yaml`:

```yaml
volumes:
  - container: /var/lib/postgresql/data
    host: ~/BasePodData/myapp/pgdata     # optional; default if omitted
```

- Server resolves `~` to the actual user home.
- Stored in DB as `app_volumes(container_path, host_path, named_volume)`.
- At deploy time written into the podman create request as a `bind` mount.
- **Survives** container recreation, image rebuild, podman machine reset,
  BasePod re-install.
- **Backed up by** Time Machine (Mac filesystem) and `basepod backup`.

## 2. App user data — named Podman volumes (opt-in)

If you set `named_volume: pg-data` instead of `host:`, the volume lives
inside the Podman VM disk.

- Faster I/O than VirtioFS, especially for DBs with heavy fsync.
- **Survives** container recreation and podman machine restart.
- **Lost on** `podman machine reset` (the VM is wiped).
- Use when perf matters more than host-visibility.

## 3. The Podman machine bind

Containers on macOS run inside a Linux VM, not directly on the Mac. The default
data directory is `~/BasePodData`; Podman's default `/Users` machine mount makes
that path visible inside the VM at the same `/Users/...` path.

For a custom `BASEPOD_DATA_DIR` outside Podman's default mounts, BasePod
initializes new machines with:

```sh
podman machine init --volume /absolute/custom/BasePodData:/BasePodData
```

If you have an existing machine without that custom volume mapping, recreate it
with the same `--volume` option.

## What survives what

| Data | Where | `destroy` app | `machine reset` | Mac wipe |
|---|---|---|---|---|
| App data (bind) | `~/BasePodData/<app>/` | yes | yes | only via backup or Time Machine |
| App data (named volume) | podman VM disk | yes | no | no |
| BasePod state (SQLite) | `~/BasePodData/_basepod/state.db` | n/a | yes | only via backup |
| Caddy certs | podman volume `basepod-caddy-data` | n/a | no (ACME re-acquires) | no |
| Built images | podman local image store | n/a | no (rebuild) | no |
| AES env-var key | macOS Keychain | n/a | yes | no — env vars unrecoverable! |

> ⚠️ The encryption key for env vars lives in your macOS Keychain, separate
> from `state.db`. A backup tar without the key is **unrecoverable** —
> restored env vars will fail to decrypt. Plan for migration if you wipe
> the Mac.

## Backup and restore

```sh
basepod backup -o backup.tar.gz
```

Contains:

- `state.db` — sqlite state (apps, env ciphertext, versions, domains, settings)
- `caddy.json` — current Caddy config snapshot
- `data/<app>/...` — every bind-mount host path under `~/BasePodData`,
  excluding the internal `_basepod/` dir

Restore is manual in v1:

1. Stop the server (`launchctl unload`)
2. Replace `~/BasePodData/_basepod/state.db` with the one from the tar
3. Restore `~/BasePodData/<app>/...` paths
4. Start the server
5. Caddy will re-acquire certs on first request
