---
type: feature
title: Local Dev Orchestration
description: Start, stop, restart, status, and logs for local Apito engine and console
resource: start.go
tags: [cli, dev, docker, services]
timestamp: 2026-07-07T00:00:00Z
---

# Local Dev Orchestration

## Purpose

Developers run Apito locally via `apito start|stop|restart|status|logs` — wrapping Docker or manual service layout depending on CLI `mode` in config.

## Flows

- **Init**: `apito init` — first-time CLI + service paths.
- **Start**: launch engine (5050) and console dev/build as configured.
- **Status/logs**: inspect running services and tail logs.
- **Stop/restart**: clean shutdown for codegen/introspection workflows.

## Main files

- `start.go`, `stop.go`, `status.go`, `logs.go`
- `service_manager.go`, `docker_manager.go`
- `init.go`, `env.go`, `db_setup.go`

## Dependencies

- Local Docker (when `mode: docker`) or manual binary paths
- Engine + console repos checked out or pulled by `update`

## Invariants

- Codegen and SDK introspection expect engine reachable at account `server_url`.
- Do not commit machine-only `.env.*.local` overrides — monorepo tracks `.env.production` in submodules.

## Common bugs

- Port 5050 already in use → start fails silently until `status`.
- Docker mode without daemon running → switch to manual mode or start Docker.
- Stale console build served while engine schema changed → rerun console dev/build.

## Tests

- Manual smoke: `apito start` → hit `/heartbeat`

## Related

- [version-and-self-upgrade](version-and-self-upgrade.md)
- Global [introspection-codegen-pipeline](../../.knowledge/features/introspection-codegen-pipeline.md)
- `studio/.knowledge/features/quick-push-resumable.md` (CI parity)
