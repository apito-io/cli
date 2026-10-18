# CLI — Handoff

## HTTP MCP (2026-10-13)

HTTP config and the stdio bridge are in `mcp_http.go`. They do not replace `mcp connect` or the default Node `mcp serve`. Contract: `/.knowledge/features/remote-mcp-oauth.md`.



## Remote MCP URL/OAuth plan (2026-10-13)

- **Done:** Defined token-free URL config and the Cloud/local/custom endpoint defaults. The CLI must not print or embed an `apt_` token. The final design uses named client entries and a Go stdio-to-HTTP bridge to the engine.
- **Done since this note:** HTTP config, the endpoint resolver, and the stdio bridge are in the section above. Default serve stays on Node.
- **Next:** After plan confirmation, add the Go bridge/HTTP mode without regressing `mcp connect`, the temporary `mcp update` fallback, `mcp serve`, or stdio config generation.
- **Do not:** Duplicate access-token permissions, add a raw token flag, or run `./command.sh save` until asked.

## MCP connect (2026-10-13)

- **Done:** `mcp connect`, `mcp update`, and `mcp serve`. MCP and CLI secrets coexist. Fake-server tests cover Cloud/Custom approval, checksum rejection, offline cache, and token-free config. `go test ./...` passed.
- **Broken:** A live local connect still needs the engine debugger on `5055` restarted, and a GitHub release archive does not exist yet.
- **Next:** Restart the engine, then `apito mcp connect --kind local`.
- **Do not:** Treat local credential deletion as remote revocation. Give the MCP approval page a write preset. `./command.sh save` until asked.

## Unified CLI (2026-10-12)

- **Done:** Shared client, credential file mode `0600`, capability probe, device login on the `cli_sync` preset, and project/key/MCP/gen/tenant/client/backup/restore commands. README, architecture, and `.knowledge/features/cli-contract.md` describe the boundary. `go test ./...` passed.
- **Not done:** A release tag. Commit.
- **Do not:** Add a raw token flag. Claim JavaScript or Python codegen. Enable `BACKUP_RESTORE_ENABLED`. `./command.sh save` until asked.

## Unified commands (2026-10-12)

- **Done:** Account token migration, capability probe, login, project/key/MCP/gen/tenant/client/backup/restore commands. Generated MCP config calls `apito mcp serve` and does not embed `apt_`.
- **Not done:** README and `.knowledge` contract. Re-run `cli_unified_test.go`. A release tag.
- **Do not:** Accept a raw token flag on normal commands. Claim pure JavaScript or Python codegen. `./command.sh save` until asked.

## Fair Source copy (2026-10-11)

- **Done:** User strings no longer say pro engines or Studio `ENGINE_REF`. SaaS catalog replace confirms with `tenants`. README has a project-to-project sync section.
- **Not done:** A new tag. The running `apito` binary is still the last release until rebuilt.
- **Do not:** Change the confirm word back to `pro_tenants`. Tag from this alone. `./command.sh save` until asked.

## Released

- `main` — tag **v0.4.17** (schema sync stages cardinality changes). Needs engine **v2.5.0** or newer.
- Previous tag **v0.4.16** (dump hash algorithm version gate). Pair dump with engine **≥v2.4.40**.

## Already on main

- Filesystem→schema sync is in `084d722`. This release does not include the local `handover.go` draft.

## Last Updated
2026-10-12
