# CLI — Handoff

## CLI v0.5.7 (2026-10-29)

- **Done:** Commit `c87c3f9`. Tag **v0.5.7** pushed. Release workflow
  succeeded.
- **Behavior:** `apito mcp serve` refuses the Node server. Studio nginx
  keeps port 4055 on the MCP proxy.
- **Not done:** No live `apito start` against `v0.1.14` in this session.
  Memory is not committed.
- **Do not:** `./command.sh save` until asked.

## CLI v0.5.2 (2026-10-27)

- **Done:** Commit `40d170a`. Local tag **v0.5.2**. Changelog in `CHANGELOG.md` and the README. Not pushed.
- **Verified:** `go test ./...` passed. Live `/tmp/apito-v052 start` declined telemetry, saw no upgrade prompt because GHCR's newest tag is `v0.1.8`, stopped the old `apito-studio`, and started the same image with the nginx mount. Console `200`, engine `200`, `/admin` `404`. `studio_version` stayed `v0.1.8`.
- **Not done:** No push. No GitHub release.
- **Do not:** Push `v0.5.2` until asked. Do not change `studio_version`.

## Studio image upgrade check (2026-10-25)

- **Done:** `apito start` discovers the newest semver Studio tag from public GHCR, prompts before replacing a pinned `studio_version` (including `local`), shows the current image version, persists only after the pull succeeds, and falls back to the cached image on a declined or failed refresh. Docs and unit tests updated.
- **Seen:** Rebuilt `./cli start` offered `local → v0.1.7`. That prompt was the binary from before the current-version line and was not answered. Local image `ghcr.io/apito-io/studio:local` was built 2026-10-12 and has no release label. `~/.apito/config.yml` still says `studio_version: local`.
- **Verified:** `go test ./...` passed. `go vet ./...` still reports existing unreachable code in `stop.go` lines 41 and 141.
- **Not done:** No upgrade, commit, push, or `./command.sh save`.
- **Do not:** Answer the upgrade prompt. Change `studio_version` or pull `v0.1.7` until asked.

## Release v0.5.0 (2026-10-19)

- **Done:** Unified CLI committed and tagged **v0.5.0**. `apito login` stores the low-harm login token. `apito types` generates from the published schema.
- **Needs:** An engine that mints `cli_login` and serves introspection with `schema.read`. `apito sync` still needs a separate `cli_sync` token.
- **Do not:** Put tokens in generated files or MCP config.

## Low-harm login (2026-10-19)

- **Done:** Contract records `cli_login`, the introspection rule, and the stolen-token boundary.
- **Verified:** Rebuilt `./cli` logged in `udb-local` against `http://localhost:4000` and generated Suchok types.
- **Not done:** No CLI release. `apito sync` needs a separate `cli_sync` token.
- **Do not:** Put `apt_` into generated files. Ask before `./command.sh save`.

## Cloud HTTP MCP URL (2026-10-18)

- **Done:** `resolveMCPHTTPTarget` and the no-account HTTP fallback use `https://api.apito.io/mcp`. Tests passed.
- **Not done:** No CLI release.
- **Do not:** Put tokens in the HTTP config. `./command.sh save` until asked.

## README license and database copy (2026-10-18)

- **Done:** README only. Apache badge points at this repo's LICENSE. Discord is `https://discord.gg/Xb7p2ddC6E`.
- **Not done:** No commit or push.
- **Do not:** Include `.memory` in the public diff. Do not document `apito mcp connect`. `./command.sh save` until asked.

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
