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

- `apito login` asks for Local, Cloud, or Custom. Local calls the engine at `http://localhost:5055` for a device code, then opens `http://localhost:4055/cli` with that code. Cloud calls `https://api.apito.io` and opens the console URL that server returns. The page reuses the console access-token form with the `cli_login` preset (`projects.read` and `schema.read` only). The approval page lists those two capabilities and says the token cannot read tenant documents or change the system. The approval action is **Give CLI access**. The engine only issues the code and redirects. If the console is offline, the browser shows that. `cli_sync` stays available for an explicit sync token and is not what login mints.
- Custom asks for a server URL, opens `{server}/cli` for access-token approval, stores the approved `apt_` token, and then asks for an account name. There is no `--token` flag and no `apito account add` command.
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
- `apito gen` and `apito types` — TypeScript/TSX, a Dart schema snapshot, or Go. `--framework none` is types and SDK only. `--language dart` writes `apito_schema.json` for Flutter `build_runner` and does not generate Dart. TypeScript generation calls `apito-codegen`, which discovers models and bounded relations from the published schema. `--draft` requires schema staging. `--check` fails when the output does not match a fresh run. Python is refused. Go generation stays on the existing adapter.
- `apito tenant list|create` — `/system/cli/v1/tenants`. Self-hosted engines do not advertise this until a core catalog exists.
- `apito client invite|status` — engagement fields only. Separate from `apito handover`.
- `apito backup list|run` and `apito restore preview|apply` — restore says it is not enabled while `BACKUP_RESTORE_ENABLED` is unset.

## Codegen safety

Output is staged, then promoted. Unowned files and modified generated files are refused unless `--force`. `--clean` deletes only manifest-owned files whose hash still matches. `.apito-gen-manifest.json` stores a schema digest and file hashes, never a token. Published schema reads use the connected `apt_` account against `/secured/graphql` and require a project id. A query whose root fields are only `__schema` or `__type` is a `schema.read` operation. Any other secured query still requires `data.read`. `cli_login` does not include `data.read`, schema write, sync write, or function deploy, so a stolen login token can list granted projects and read their published schema and cannot read tenant documents or change the system. The CLI does not read an app `.env`, mint a tenant-admin token, or write `apt_` / `ak_` values into snapshots or generated files. A missing account, project, or `published_introspection` capability fails before generation. This phase covers Suchok and the Flutter snapshot consumer. Protiva, Prottoy, and Rosna adopt the same command later.

## Related

- [apito-sync](apito-sync.md)
- [apito-handover](apito-handover.md)
- [admin-and-ops](admin-and-ops.md)

## Blast radius

The new routes are additive. `apito login` mints `cli_login`. `cli_sync` remains the explicit sync preset and is unchanged. SDKs, the console, MCP, and Udbhabon keep their existing auth. `apito gen` still emits the full TypeScript app artifacts unless `--framework none` is set. `apito-gen` with no flags still walks to `go.mod` the way `go generate` did. Restore stays off unless `BACKUP_RESTORE_ENABLED` is set on the Cloud host.
