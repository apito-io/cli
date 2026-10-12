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
- **Start**: pull `ghcr.io/apito-io/studio:<studio_version>` and run the `apito-studio` container with `~/.apito/db` mounted at `/data`, `.env.engine` mounted at `/opt/apito/.env`, and `.env.console` mounted at `/opt/apito/.env.console`. An empty `studio_version` uses the `latest` tag.
- **Database**: SQLite and system files stay in `~/.apito/db`, so a restart keeps them. That folder uses the same layout as Pro's data directory: `sqlite/system`, `sqlite/projects`, `sqlite/tenants`, and `logs`. The engine env points the volume at `/data` and listens on `5051` inside the container. Nginx publishes the console on `5055` and proxies `/system/`, `/auth/v2/`, `/auth/refresh/`, `/admin/`, `/plugin/`, `/secured/`, `/function/`, and `/media/` to that engine port. The login page stays on the console. The first Studio start prints `oss@apito.io` and a random admin password. Later starts keep the same password.
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
- Missing config is recreated with one warning and no prompt.
- `.env.engine` keeps `DEFAULT_DATABASE_DIR=/data`, `SERVE_PORT=5051`, and key paths under `/data/keys`. Those paths match the container mounts, not the host `~/.apito` path.
- `BRANKA_KEY` and `APITO_ADMIN_RESET_SECRET` are generated once and kept on later starts.
- The console env uses `http://localhost:5055` for the REST and GraphQL URLs.

## Common bugs

- Docker daemon stopped → start fails before the image is pulled.
- Empty `studio_version` follows whatever GHCR currently tags `latest`.

## Tests

- `apito migrate --help` describes a database copy onto an existing destination.
- `apito start --help` names `ghcr.io/apito-io/studio`.

## Related

- [project-and-plugin-create](project-and-plugin-create.md)
- [admin-and-ops](admin-and-ops.md)
