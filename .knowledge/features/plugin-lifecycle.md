---
type: feature
title: Plugin Lifecycle
description: Build artifacts locally; install through Engine signed registry
resource: plugin_add.go
tags: [cli, plugins, deploy, build]
timestamp: 2026-09-03T00:00:00Z
---

# Plugin Lifecycle

## Purpose

CLI builds platform binaries and asks Engine to install reviewed catalog plugins. One verification path serves Console and CLI.

## Flows

- **Build**: `apito plugin build [dir]` — local/OS/arch zip (author loop).
- **Add**: `apito plugin add <id>` → `POST /system/plugin/install` with account sync key. Engine verifies signature, checksum, zip safety.
- **Deploy**: `apito plugin deploy` multipart remains the local-dev escape hatch (`PLUGIN_ALLOW_LOCAL_UPLOAD`).
- **List/status**: existing plugin subcommands.

## Main files

- `plugin_add.go` — Engine install
- `plugin_build.go`, `plugin_deploy.go`

## Dependencies

- Engine [plugin-registry](../../../engine/.knowledge/features/plugin-registry.md)

## Invariants

- Do not download GitHub assets in the CLI for `plugin add`.
- Target linux/amd64 for servers even when building on macOS arm64.
- Install still needs super-admin / `plugins.deploy` on the Engine.

## Related

- [project-and-plugin-create](project-and-plugin-create.md)
