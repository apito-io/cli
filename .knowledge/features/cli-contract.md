---
type: feature
title: Unified CLI contract
description: One Apache CLI for a local engine and Apito Cloud, gated by server capabilities
resource: cli
tags: [cli, accounts, codegen, mcp]
timestamp: 2026-10-12T00:00:00Z
---

# Unified CLI contract

## Purpose

One `apito` binary talks to a self-hosted engine and to Apito Cloud. Cloud-only commands stay visible and fail closed until the server reports the capability. The CLI does not embed private host source.

## Account and login

- `apito login` asks for Local, Cloud, or Custom. Local calls the engine at `http://localhost:5055` for a device code, then opens `http://localhost:4055/cli` with that code. Cloud calls `https://api.apito.io` and opens the console URL that server returns. The page reuses the console access-token form with the CLI sync preset. The approval action is **Give CLI access**. The engine only issues the code and redirects. If the console is offline, the browser shows that.
- Custom asks for a server URL and an `apt_` token. There is no `--token` flag and no `apito account add` command.
- `apito mcp connect` asks for the same three kinds and opens `/mcp`. Custom asks for a server URL and uses browser approval, not a pasted token. Approval is locked to the read-only `mcp_assistant` preset. The approval action is **Give MCP access**. Resource and tenant scope still come from the access-token form.
- The MCP secret is stored beside the CLI secret (`mcp_accounts` in the credential file, or keychain service `apito-cli-mcp`). A later MCP connect does not replace the CLI token. Account removal deletes both. Deleting the local credential does not revoke the server token; revoke it from the console token inventory.
- `apito mcp connect` also installs the latest verified MCP runtime under `~/.apito/mcp`. `apito mcp update` refreshes it. `apito mcp serve` runs that bundle with Node.js and downloads only when nothing is installed. Offline use keeps the last verified bundle. Generated `apito mcp config` launches `apito mcp serve` and contains no token. Serve refuses to start when no explicit project is available.
- `apito account create` remains a compatibility alias for a custom account. `apito account remove` removes an account.
- Tokens are stored in `~/.apito/credentials.yml` (mode `0600`) or the OS keychain when `APITO_CREDENTIAL_STORE=keychain`. `config.yml` keeps a token reference, not the secret.
- Retired `cli-`, `sdk-`, and `mcp-` prefixes are rejected.

## Capabilities

`GET /system/cli/v1/capabilities` is the authority. `/system/health` is connectivity only. The CLI also calls `/system/access-tokens/me`, `/system/access-tokens/catalog`, and `schemaVersioningStatus`.

| Flag | Core | Cloud host |
|------|------|------------|
| project_access, project_tokens, published_introspection, mcp, cli_login | on | on |
| schema_staging | when versioning is on | when versioning is on |
| tenant_catalog, client_lifecycle, managed_backups | off | on |
| restore_actions | off | on only when restore is enabled |

A missing Cloud capability returns: `This command requires an Apito Cloud account with the required capability. Run apito login`.

## Commands

Global flags: `--account`, `--project`, `--tenant`, `--json`.

- `apito project list|open`
- `apito key project list|create|revoke`
- `apito mcp connect|update|config|serve` — connect approves `mcp_assistant` and installs the runtime. Generated config runs `apito mcp serve` and does not contain `apt_`. Write and deploy tools return capability-denied for that read-only token.
- `apito gen` and `apito types` — TypeScript/TSX or Go. `--framework none` is types and SDK only. `--draft` requires schema staging. Python and Dart are refused.
- `apito tenant list|create` — `/system/cli/v1/tenants`. Self-hosted engines do not advertise this until a core catalog exists.
- `apito client invite|status` — engagement fields only. Separate from `apito handover`.
- `apito backup list|run` and `apito restore preview|apply` — restore says it is not enabled while `BACKUP_RESTORE_ENABLED` is unset.

## Codegen safety

Output is staged, then promoted. Unowned files and modified generated files are refused unless `--force`. `--clean` deletes only manifest-owned files whose hash still matches. `.apito-gen-manifest.json` stores a schema digest and file hashes, never a token.

## Related

- [apito-sync](apito-sync.md)
- [apito-handover](apito-handover.md)
- [admin-and-ops](admin-and-ops.md)

## Blast radius

The new routes are additive. `cli_sync` is unchanged. SDKs, the console, MCP, and Udbhabon keep their existing auth. `apito gen` still emits the full TypeScript app artifacts unless `--framework none` is set. `apito-gen` with no flags still walks to `go.mod` the way `go generate` did. Restore stays off unless `BACKUP_RESTORE_ENABLED` is set on the Cloud host.
