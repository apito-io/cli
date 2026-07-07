---
type: feature
title: Apito Sync
description: Schema and content synchronization between configured accounts and projects
resource: sync.go
tags: [cli, sync, schema, graphql]
timestamp: 2026-07-07T00:00:00Z
---

# Apito Sync

## Purpose

`apito sync` copies **schema** (models, fields, relations) or **content** (model rows) between two configured accounts/projects using system GraphQL and access tokens. On pro engines, schema applies as **drafts** — publish from Console.

## Flows

- **Setup**: `apito account create` → server URL + cloud sync key per account.
- **Run**: `apito sync --from A --to B --type schema|content` (or interactive prompts).
- **Schema path**: introspect → diff (`sync_diff.go`) → plan → apply (`sync_apply.go`, merge helpers).
- **Content path**: model selection → paginated copy with relation awareness (`sync_content.go`).
- **Dry run**: `--dry-run` shows plan; `--yes` skips confirmations.

## Main files

- `sync.go`, `sync_graphql.go`, `sync_schema.go`, `sync_schema_merge.go`
- `sync_diff.go`, `sync_apply.go`, `sync_content.go`, `sync_plan.go`
- Tests: `sync_diff_test.go`, `sync_apply_test.go`, `sync_schema_merge_test.go`, `sync_select_test.go`

## Dependencies

- Engine `/system/graphql` + sync token auth
- Global [naming-engine](../../.knowledge/features/naming-engine.md)
- Engine [schema-change-versioning](../engine/.knowledge/features/schema-change-versioning.md)

## Invariants

- Validate project profiles match before apply (driver/type compatibility).
- Schema sync on pro never bypasses versioning — stages drafts, not live DDL directly.
- Use canonical naming from engine — sync merge relies on stable model/field IDs.

## Common bugs

- Missing account or token → configure via `apito config` / account create.
- Destination pro engine: forgetting Console publish after sync → API still on old schema.
- Content sync without schema parity → relation FK violations.

## Tests

- `sync_diff_test.go`, `sync_apply_test.go`, `sync_schema_merge_test.go`

## Related

- Global: [introspection-codegen-pipeline](../../.knowledge/features/introspection-codegen-pipeline.md)
- [auth-and-tokens](../engine/.knowledge/features/auth-and-tokens.md)
