# cli — Architecture

Distilled from root `ARCHITECTURE.md` and the Go cobra codebase.

## Overview

**Apito CLI** (`apito` binary) is one Apache-licensed tool for a local engine and Apito Cloud. It manages accounts, local Studio, schema sync, plugins, codegen, and MCP. Cloud-only commands stay visible and fail closed until the server reports the capability. Account tokens are `apt_`. Project keys are `ak_`.

## Command surface

| Area | Commands |
|------|----------|
| Accounts | `login`, `account list|select|test|remove|add local` |
| Projects and keys | `project list|open`, `key project list|create|revoke` |
| Codegen and MCP | `gen` / `types`, `mcp config|serve` |
| Cloud-gated | `tenant list|create`, `client invite|status`, `backup list|run`, `restore preview|apply` |
| Local Studio | `start`, `stop`, `restart`, `status`, `logs` |
| Sync and migration | `sync`, `migrate`, `handover` |
| Plugins | `plugin`, `create` |

Go **1.25+**, cobra. Global selectors are `--account`, `--project`, and `--tenant`.

## Key components

- **`system_client.go` / `capabilities.go`** — HTTP and GraphQL, plus capability discovery. `/system/health` is connectivity only.
- **`credentials.go` / `config.go`** — named accounts. The access token is not written to `config.yml`.
- **`login.go`** — device login for Cloud and a prompted token for a local URL.
- **`gen_cmd.go`** — stages TypeScript or Go output and records a credential-free manifest.
- **`sync*.go`** — schema and content sync on top of `SystemClient`.

## Sync architecture (high level)

```
accounts (from/to) → select projects → validate profiles
  → schema: introspect/diff/plan/apply (stages drafts when the server supports staging)
  → content: model data copy with relation awareness
```

Schema writes stage a draft when the server has schema staging. Publish stays in the console.

Last Updated: 2026-10-12

## Plugin build flow

Flags-first (`--build`, `--platform`, `--arch`, `--type`) with interactive fallback → system or Docker compile → artifact for deploy.

## Consumers

- Developers syncing staging → production schema
- Plugin authors (build/deploy loop)
- Local dev: `apito start` before console codegen or SDK introspection

Last Updated: 2026-07-07
