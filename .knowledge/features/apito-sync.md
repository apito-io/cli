---
type: feature
title: Apito Sync
description: Schema, functions, and content synchronization between configured accounts and projects
resource: sync.go
tags: [cli, sync, schema, functions, graphql]
timestamp: 2026-07-16T00:00:00Z
---

# Apito Sync

## Purpose

`apito sync` copies **schema** (models, fields, relations), **functions** (Logic function draft source + metadata), or **content** (model rows) between two configured accounts/projects using system GraphQL and access tokens. On pro engines, schema applies as **drafts** — publish from Console.

## Flows

- **Setup**: `apito account create` → server URL + cloud sync key per account.
- **Run**: `apito sync --from A --to B --type schema|functions|content` (or interactive prompts; menu order: schema → functions → content).
- **Schema path**: introspect → diff (`sync_diff.go`) → plan → apply (`sync_apply.go`, merge helpers).
- **Functions path**: `projectFunctionsInfo` (incl. `active_revision_hash`) → diff by name (`sync_function_diff.go`: add/update/deploy) → multiselect → upsert and/or `deployFunctionToProject`. Orchestration in `sync_functions.go`.
- **Content path**: model selection → paginated copy with relation awareness (`sync_content.go`).
- **Dry run**: `--dry-run` shows plan; `--yes` skips confirmations.

## Functions sync

Reuses the shipped engine lifecycle GraphQL (no new ops). Transfers **draft source + metadata**; the destination engine mints its own callable secret and creates a new revision in its own artifact store when publishing — revision binaries are never copied between engines.

**Diff kinds:** add / update (draft drift) / **deploy** (draft equal but destination live `active_revision_hash` missing or ≠ `sha256(source)` while source has an `active_revision_id`).

**Apply:**

- `deploy` → `deployFunctionToProject` only (no upsert).
- `add` / `update` → upsert draft; then deploy if `--deploy` **or** the source function is published (`active_revision_id` set).
- `projectFunctionsInfo` returns `active_revision_hash` (enriched from the active revision row) so the CLI does not N+1 `listFunctionRevisions`.

Three directions:

- **Project → project** (default): both sides are remote accounts. Diff by function name (add / update / deploy / capability drift), multiselect, upsert and/or deploy. Prints the destination `active_revision_id`. **No disk / `--dir` involved** — GraphQL reads draft `source` from the system DB and upserts it to the destination system DB.
- **Project → filesystem** (`--to filesystem`): export each function to `{dir}/{name}/meta.json` + `source.ts`. Default dir: `~/.apito/temp/functions` (override with `--dir`). The REST secret is stripped unless `--include-secrets`. `active_revision_hash` may be kept as informational; `active_revision_id` is not.
- **Filesystem → project** (`--from filesystem --to prod`): scan the dir, validate meta/source (folder name must equal `meta.json.name`), upsert (+ `--deploy` or published-source auto-deploy when meta still carries a revision id — normally stripped on export).

Flags: `--dir` (optional; defaults to `~/.apito/temp/functions` for filesystem sides), `--deploy` (also publish after upsert for draft-only sources), `--include-secrets` (copy `rest_api_secret_url_key` instead of regenerating). Reserved account name is **`filesystem`** (not `local` — a configured account named `local` is a normal remote localhost engine). At least one side must be a remote account.

Function definitions are tenant-agnostic — for SaaS, pass a tenant at invoke/test time (MCP `tenant_id` / `X-Apito-Tenant-ID`), not at definition-sync time.

### Local on-disk format

```text
{dir}/
  {functionName}/
    meta.json      # name, description, capabilities, trigger_type, language,
                   # graphql_schema_type, runtime_config, request/response, env_vars
    source.ts      # draft Deno/TS source
```

## Main files

- `sync.go`, `sync_graphql.go`, `sync_schema.go`, `sync_schema_merge.go`
- `sync_diff.go`, `sync_apply.go`, `sync_content.go`, `sync_plan.go`
- Functions: `sync_functions.go`, `sync_function_fs.go`, `sync_function_diff.go`
- Tests: `sync_diff_test.go`, `sync_apply_test.go`, `sync_schema_merge_test.go`, `sync_select_test.go`, `sync_functions_test.go`

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

- `sync_diff_test.go`, `sync_apply_test.go`, `sync_schema_merge_test.go`, `sync_functions_test.go` (diff kinds + local fs round-trip)

## Related

- Global: [introspection-codegen-pipeline](../../.knowledge/features/introspection-codegen-pipeline.md)
- [auth-and-tokens](../engine/.knowledge/features/auth-and-tokens.md)
