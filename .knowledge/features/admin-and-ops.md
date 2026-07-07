---
type: feature
title: Admin and Ops
description: CLI account configuration, admin commands, and operator utilities
resource: admin.go
tags: [cli, admin, config, accounts]
timestamp: 2026-07-07T00:00:00Z
---

# Admin and Ops

## Purpose

Operator-facing CLI beyond sync/plugins: **account** management, **config** YAML, **admin** subcommands, **build** zip/docker packaging, and shared **utility** helpers.

## Flows

- **Accounts**: `apito account create|list` — map friendly names to `server_url` + `cloud_sync_key`.
- **Config**: `apito config` — default account, mode (docker/manual), timeouts.
- **Admin**: privileged maintenance commands (see `admin.go` for current surface).
- **Build**: `apito build` — package project for docker or zip deploy.

## Main files

- `admin.go`, `config.go`, `build.go`, `utility.go`
- `config.yml.example` — documented keys

## Dependencies

- Engine system REST/GraphQL endpoints
- [auth-and-tokens](../engine/.knowledge/features/auth-and-tokens.md)

## Invariants

- Sync keys are secrets — do not log full keys in CI output.
- Default account must exist before non-interactive sync/deploy scripts run.

## Common bugs

- Typo in `server_url` (missing `/api` vs raw engine port) → 404 on deploy.
- Multiple accounts with same URL but different keys → confusing sync direction.

## Tests

- Config load tests if added; manual account CRUD smoke

## Related

- [apito-sync](apito-sync.md)
- [plugin-lifecycle](plugin-lifecycle.md)
