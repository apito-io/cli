# cli — Knowledge

Part of the **apito** ecosystem. See `/.knowledge/projects/apito.md` for how this repo fits and its blast radius.

## Read order
1. This file. 2. `ARCHITECTURE.md`. 3. `DECISIONS.md`. 4. `features/README.md`. 5. `../../.memory/CURRENT.md` and `../../.memory/HANDOFF.md`.

## Purpose

**Apito CLI** (`apito` binary): local operator tool for accounts, start/stop engine+console, schema/content sync, plugin build/deploy, self-upgrade, and admin ops. Uses system GraphQL/REST with per-account access tokens.

## Responsibilities

- Account + YAML config management (`config.go`)
- Local service orchestration (`start`, `stop`, `status`, `logs`)
- Cross-account sync (`sync` — schema drafts on pro engines)
- Plugin build/deploy pipeline
- Version pins and self-upgrade

## Consumers / blast radius

| Consumer | What breaks on CLI changes |
|----------|----------------------------|
| **Developers** | Sync/plan/apply behavior, token auth headers |
| **Plugin authors** | Build flags, deploy multipart contract |
| **CI/codegen** | Assumes `apito start` + valid sync tokens for introspection |

## Reasoning archive

- Root `ARCHITECTURE.md` (detailed command flows)
- Historical Cursor plans distilled into this knowledge base live in `archive/plans/`.

Last Updated: 2026-07-07
