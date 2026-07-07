# cli — Architecture

Distilled from root `ARCHITECTURE.md` and the Go cobra codebase.

## Overview

**Apito CLI** (`apito` binary) is the local operator tool for Apito: manage accounts, start/stop engine+console, sync schema/content between projects, build/deploy plugins, self-upgrade, and run admin ops. It talks to engine **system GraphQL** and REST using per-account `ServerURL` + access tokens (`cli-` prefix).

## Command surface

| Area | Commands |
|------|----------|
| Lifecycle | `init`, `start`, `stop`, `restart`, `status`, `logs` |
| Accounts | `account`, `config` |
| Projects/plugins | `create`, `plugin build|deploy|…`, `build` |
| Sync | `sync` (schema or content, draft-aware on pro) |
| Maintenance | `update`, `self-upgrade`, `admin` |

Go **1.25+**, cobra + survey/promptui for interactive flags.

## Key components

- **`config.go`** — YAML config: accounts, default account, timeouts, cloud sync keys.
- **`service_manager.go` / `docker_manager.go`** — local engine/console process orchestration.
- **`sync*.go`** — GraphQL clients, schema diff/merge/apply, content sync.
- **`plugin_build.go` / `plugin_deploy.go`** — cross-platform plugin artifacts + deploy.
- **`version_manager.go` / `self_upgrade.go`** — engine/console/CLI version pins.
- **`create.go` / `admin.go`** — scaffolding and operator commands.

## Sync architecture (high level)

```
accounts (from/to) → select projects → validate profiles
  → schema: introspect/diff/plan/apply (stages drafts on pro)
  → content: model data copy with relation awareness
```

Uses `/system/graphql` with access tokens; schema writes on pro engines stage drafts — publish from Console.

## Plugin build flow

Flags-first (`--build`, `--platform`, `--arch`, `--type`) with interactive fallback → system or Docker compile → artifact for deploy.

## Consumers

- Developers syncing staging → production schema
- Plugin authors (build/deploy loop)
- Local dev: `apito start` before console codegen or SDK introspection

Last Updated: 2026-07-07
