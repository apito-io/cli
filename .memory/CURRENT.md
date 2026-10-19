# CLI — Current

## Now (2026-10-19) — CLI v0.5.0

Tagged **v0.5.0**. `apito login` stores a `cli_login` token that reads the project list and published schema. `apito types` generates from that schema. `cli_sync` remains the explicit sync preset. Needs an engine that mints `cli_login` and allows pure introspection with `schema.read`.

## Now (2026-10-18) — Cloud HTTP MCP URL

`apito mcp config --transport http` for a Cloud account prints `https://api.apito.io/mcp`. The token URL is `https://api.apito.io/oauth/token`. Uncommitted. Ask before `./command.sh save`.

## Now (2026-10-18) — README copy only

The README lists SQLite, PostgreSQL, MySQL, MariaDB, and MongoDB, with DynamoDB, Oracle, and Cassandra coming soon. Local URLs are console `4055` and engine `5055`. The license line is Apache-2.0 for the CLI and AGPL-3.0 for engine, console, and Studio. No CLI code changed. Uncommitted. Leave `.memory` out of the public commit. Ask before `./command.sh save`.

## Now (2026-10-13) — HTTP MCP is opt-in

`apito mcp config --transport http` prints `{ "type": "http", "url": "…" }` with no token. The host picks Cloud, local `http://localhost:4055/mcp`, or the custom origin. `apito mcp serve --transport http` exchanges the stored `apt_` grant for `mct_` and forwards Content-Length frames. Default serve still requires Node. `APITO_MCP_RUNTIME=node` forces that runtime. `go test` covered endpoint resolution, token-free config, the bridge, and the existing stdio edition test. Uncommitted. Ask before `./command.sh save`.

## Now (2026-10-13) — remote MCP URL/OAuth plan

The next MCP mode is token-free HTTP configuration: clients receive an Apito URL and complete OAuth authorization-code + PKCE in the browser. Cloud defaults to `https://mcp.apito.io/mcp`; local Studio is `http://localhost:4055/mcp`; custom uses the supplied Apito-compatible host. `kind` is optional context and defaults to Cloud.

This must preserve `apito mcp connect`, `mcp serve`, and stdio configs while reusing the existing `mcp_assistant` approval/mint flow. Multiple HTTP connections use distinct `mcpServers` names mapped from named CLI accounts; standard URLs remain clean and `kind` is inferred from the endpoint. The final `mcp serve` path will be a Go stdio-to-HTTP bridge to the engine; the Node bundle remains a temporary rollback path. Remote OAuth credentials remain resource-bound and separate from the stdio `mcp_accounts` store. No CLI implementation was started in this planning pass. Do not `./command.sh save` until asked.

## Now (2026-10-13) — MCP connect

- `apito mcp connect` approves Local, Cloud, or Custom in the browser and stores an `mcp_assistant` token apart from the CLI token. Local uses engine `5055` and console `4055`.
- `apito mcp update` downloads the versioned `server.mjs` bundle into `~/.apito/mcp` after a SHA-256 check. `apito mcp serve` uses that cache and Node.js, and it does not refresh on every start.
- Generated MCP config launches `apito mcp serve` and contains no token. Serve fails closed without an explicit project. Removing the local credential does not revoke the server token.
- The release workflow is in `apito-mcp/.github/workflows/mcp-runtime.yml`. No runtime archive is published until that tag workflow runs.
- Engine controller tests, `go test ./...` in the CLI, the stdio bundle smoke test, and the docs above are updated. Not committed. Do not `./command.sh save` until asked.

## Now (2026-10-12) — unified CLI contract

- One Apache binary. `apt_` tokens stay in `~/.apito/credentials.yml` (mode `0600`). `config.yml` keeps a reference.
- `apito login` asks for Local, Cloud, or Custom. Local calls the engine on `http://localhost:5055`, then opens the console at `http://localhost:4055/cli`. Cloud opens the console URL from `https://api.apito.io`. Custom asks for a URL and an `apt_` token. There is no `apito account add`.
- Project, key, MCP, gen/types, tenant, client, backup, and restore are registered. Self-hosted servers fail closed on tenant, client, backup, and restore. Restore reports that it is not enabled while the host gate is off.
- Contract: `.knowledge/features/cli-contract.md`. README and architecture notes match that contract.
- `go test ./...` passed, including the fake-server matrix. Not tagged. Ask before commit and `./command.sh save`.

## Now (2026-10-12) — unified commands

- One binary. Tokens live in `~/.apito/credentials.yml` (mode `0600`), not in `config.yml`.
- `apito login` is a browser device flow for Cloud and a prompted `apt_` token for a local URL.
- `apito project`, `key project`, `mcp`, `gen` / `types`, `tenant`, `client`, `backup`, and `restore` are registered. Cloud-only commands error with the capability sentence when the server does not offer them.

## Now (2026-10-11) — Fair Source copy

- Dump catalog confirm word is `tenants`, not `pro_tenants`.
- Sync messages say a schema change is staged as a draft until the console publishes.
- README documents local-to-Cloud sync and the GHCR images. License section stays Apache 2.0. `go build` succeeded. Not tagged. Ask before `./command.sh save`.

## Now (2026-10-11)

- **`apito sync` v0.4.17** — cardinality changes stage through the engine relation migration. Needs engine **v2.5.0** or newer. Sync still does not publish.
- **`apito dump` v0.4.16** — `schema_hash_version` gate. Needs engine **≥v2.4.40**.
- `handover.go` is local only and is not in the tag.

## Last Updated
2026-10-11
