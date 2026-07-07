---
type: feature
title: Plugin Lifecycle
description: Build cross-platform plugin artifacts and deploy to engine accounts
resource: plugin_build.go
tags: [cli, plugins, deploy, build]
timestamp: 2026-07-07T00:00:00Z
---

# Plugin Lifecycle

## Purpose

CLI manages the plugin author loop: **build** platform-specific binaries (system or Docker toolchain) and **deploy/update** packages to engine via multipart REST with sync key auth.

## Flows

- **Build**: `apito plugin build [dir]` → flags or prompts for method, OS/arch, Go build type.
- **Deploy**: `apito plugin deploy [dir] -a account` → read `config.yml` → POST `/system/plugin/deploy`.
- **Update**: same flow with update endpoint when plugin id exists.
- **List/status**: plugin subcommands for operational visibility.

## Main files

- `plugin_build.go` — build method, platform, validation
- `plugin_deploy.go`, `plugin_add.go` — deploy HTTP + metadata
- Root `ARCHITECTURE.md` — detailed flow diagrams

## Dependencies

- Global [plugin-grpc-protocol](../../.knowledge/features/plugin-grpc-protocol.md)
- Engine [plugin-system](../engine/.knowledge/features/plugin-system.md)
- Account `CloudSyncKey` in CLI config

## Invariants

- Plugin `config.yml` must declare id, version, language, type — deploy rejects incomplete metadata.
- Target Linux amd64 for most server deploys even when building on macOS arm64.
- Deploy uses sync key header — not console JWT cookie.

## Common bugs

- Docker build fallback when local Go runtime missing — unexpected slow path.
- Wrong account default → deploy to unintended server.
- Version not bumped → engine treats deploy as no-op update.

## Tests

- Manual deploy smoke; build validation covered indirectly via compile

## Related

- `sdk/go-plugin-build-sdk/.knowledge/features/` (when present)
- [project-and-plugin-create](project-and-plugin-create.md)
