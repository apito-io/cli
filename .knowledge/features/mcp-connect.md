---
type: feature
title: MCP connect
description: Browser approval for a read-only MCP token and a checksum-verified local stdio runtime
resource: mcp_connect.go
tags: [cli, mcp, credentials]
timestamp: 2026-10-12T00:00:00Z
---

# MCP connect

## Purpose

`apito mcp connect` authorizes a named account for the MCP server and installs the runtime that `apito mcp serve` runs in the foreground.

## Flows

- Local calls `http://localhost:5055/system/mcp/v1/login/start` and opens `http://localhost:4055/mcp`. Cloud and Custom open the verification URL returned by the server. Custom asks for the server URL and does not accept a raw token flag.
- The console reuses the access-token form, locked to `mcp_assistant`. The button is **Give MCP access**. The CLI button is **Give CLI access**. `/mcp` without a device code stays the documentation page.
- The poll response returns the secret once, plus project and tenant grant metadata. The CLI stores that secret apart from the CLI token and keeps a default project only when the grant selected exactly one project.
- `apito mcp serve` passes the MCP token, the account endpoint, the edition derived from server capabilities, and an explicit project allowlist. It exits when no safe project is available.
- The runtime is one bundled `server.mjs` plus the user's Node.js. Releases publish `apito-mcp-<version>.tar.gz` and `SHA256SUMS`. The CLI verifies the checksum, rejects archive paths that escape the destination, and writes `~/.apito/mcp/versions/<version>`. A failed download leaves the previous verified bundle in place.

## Invariants

- Generated `apito mcp config` launches `apito mcp serve` and does not embed `apt_` or `ak_`. `--transport http` prints a URL instead, still without a token. See [remote-mcp-oauth](remote-mcp-oauth.md).
- `APITO_MCP_DIR` runs this checkout through `npx tsx`. `APITO_MCP_COMMAND` replaces the launch command. `APITO_MCP_RELEASE_URL` overrides the release manifest.
- `mcp serve` does not check for a newer release on every start. `mcp connect` and `mcp update` do.
- Local credential deletion does not revoke the issued server token.

## Related

- [cli-contract](cli-contract.md)
