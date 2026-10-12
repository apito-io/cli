---
type: feature
title: Project and Plugin Create
description: Create a project on a connected account; scaffold plugins separately
resource: create.go
tags: [cli, create, accounts]
timestamp: 2026-10-12T00:00:00Z
---

# Project and Plugin Create

## Purpose

`apito create` creates a project on a connected account through `POST /system/project/create`. Plugin scaffolds stay on `apito plugin create`.

## Flows

- **Account**: one configured account is announced and used. Several accounts use the same picker as `apito sync`. Zero accounts asks to connect an account, then creates the project on that account.
- **Project**: prompts for name, description, and general or SaaS, then sends a SQLite `db_config` to the system API. Local Studio and cloud accounts both work.
- **Plugin**: `apito plugin create` scaffolds a plugin. `apito create` does not.

## Main files

- `create.go`
- `plugin_deploy.go` (`createPluginScaffold`)
- `sync.go` (`resolveSyncAccount`)

## Dependencies

- A connected account with `server_url` and an access token
- Engine `ProjectCreation` on `/system/project/create`

## Invariants

- The command does not open a browser.
- Silent config runs before account selection.

## Common bugs

- No token on the account → the system API returns unauthorized.
- Destination engine rejects the SQLite file config when that account requires another database engine.

## Tests

- `apito create --help` describes account selection and points plugin scaffolds at `apito plugin create`.

## Related

- [local-dev-orchestration](local-dev-orchestration.md)
- [admin-and-ops](admin-and-ops.md)
