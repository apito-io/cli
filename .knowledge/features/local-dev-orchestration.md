---
type: feature
title: Local Dev Orchestration
description: Start, stop, restart, status, and logs for the public Studio image
resource: studio_runtime.go
tags: [cli, dev, docker, studio]
timestamp: 2026-10-12T00:00:00Z
---

# Local Dev Orchestration

## Purpose

`apito start`, `stop`, `restart`, `status`, and `logs` run the public Studio image `ghcr.io/apito-io/studio`. Monorepo engine and console development stays `go run` and `pnpm dev`.

## Flows

- **Silent config**: the first command that needs `~/.apito` writes `config.yml`, `bin/.env.engine`, and `bin/.env.console`, generates `BRANKA_KEY` and the RSA signing keys, prints one warning, and continues.
- **Start**: check public GHCR for the newest Studio tag before pulling. An empty `studio_version` or an explicit `latest` tag tracks `latest` and refreshes automatically. A pinned tag, including the historical `local` tag, prompts before upgrading to a newer published semver tag. The prompt names the current image (its release tag, or its build date when the pin is not a release) and the latest tag. `y` pulls and persists the new tag, while `n` starts the pinned image. If the registry is unavailable, or a pull fails while a cached image exists, start continues with the cached image. Before the container is replaced, start checks host ports `4055` and `5055`. A busy port names the Docker container or process holding it and asks to stop that service, kill it, or skip. Skip leaves the old process running and does not start Studio. After the container starts, start pings `http://localhost:4055`, `http://localhost:4055/admin`, and `http://localhost:5055`. The console, admin, and engine URLs are printed only after those checks answer.
- **Database**: SQLite and system files stay in `~/.apito/db`, so a restart keeps them. That folder uses the same layout as Pro's data directory: `sqlite/system`, `sqlite/projects`, `sqlite/tenants`, and `logs`. The engine env points the volume at `/data` and listens on `5055` inside the container. Nginx publishes the console on `4055` and proxies `/system/`, `/auth/v2/`, `/auth/refresh/`, `/admin/`, `/plugin/`, `/secured/`, `/function/`, and `/media/` to that engine port. The host also publishes engine port `5055`. Start mounts `studio.nginx.conf` over the image nginx config so the console stays on `4055` and the engine keeps `5055`. Images through `v0.1.8` listen on `5055` and proxy to `5051`, which makes the engine fail with `address already in use`. The login page stays on the console. The first Studio start prints `oss@apito.io` and a random admin password. Later starts keep the same password.
- **Status/logs**: inspect that container and tail its logs.
- **Stop/restart**: stop or replace that container.
- **Migrate**: `apito migrate` copies a project or tenant database onto an existing destination. Schema hashes are checked. A remote destination needs `--allow-push` and a second confirmation.

## Main files

- `studio_runtime.go`, `studio_env.go`, `start.go`, `stop.go`, `status.go`, `logs.go`
- `config.go` (`ensureCLIConfig`, `studio_version`)
- `dump.go` (command name `migrate`)

## Dependencies

- Docker, to pull and run the Studio image
- Published GHCR tags from the public Studio repository

## Invariants

- The image tag is `studio_version` in `~/.apito/config.yml`, or `latest` when that field is empty.
- Pinned Studio tags never change without an explicit upgrade confirmation.
- Missing config is recreated with one warning and no prompt.
- `.env.engine` keeps `DEFAULT_DATABASE_DIR=/data`, `SERVE_PORT=5055`, `CORS_ORIGIN=http://localhost:4055`, and key paths under `/data/keys`. Those paths match the container mounts, not the host `~/.apito` path.
- `BRANKA_KEY` and `APITO_ADMIN_RESET_SECRET` are generated once and kept on later starts.
- The console env uses `http://localhost:4055` for the REST and GraphQL URLs.

## Common bugs

- Docker daemon stopped → start fails before the image is pulled.
- An old container or process on `4055` or `5055` → start asks before replacing it. Skip does not start Studio.
- Studio container is up but `4055` or `4055/admin` does not answer → start reports that failure and does not print the URLs.
- Empty `studio_version` follows whatever GHCR currently tags `latest`.
- GHCR unavailable → update check is warned and the configured/cached image remains usable.

## Tests

- `apito migrate --help` describes a database copy onto an existing destination.
- `apito start --help` names `ghcr.io/apito-io/studio`.

## Related

- [project-and-plugin-create](project-and-plugin-create.md)
- [admin-and-ops](admin-and-ops.md)
