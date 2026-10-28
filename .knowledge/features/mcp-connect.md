---
type: feature
title: MCP connect
description: Browser approval for a read-only MCP token and a printed HTTP client config
resource: mcp_connect.go
tags: [cli, mcp, credentials]
timestamp: 2026-10-12T00:00:00Z
---

# MCP connect

## Purpose

`apito mcp connect` authorizes a named account and prints the HTTP MCP JSON. The `apito-mcp` npm package is deprecated and is not downloaded.

## Flows

- Local calls `http://localhost:5055/system/mcp/v1/login/start` and opens `http://localhost:4055/mcp`. Cloud and Custom open the verification URL returned by the server. Custom asks for the server URL and does not accept a raw token flag.
- The console reuses the access-token form, locked to `mcp_assistant`. The button is **Give MCP access**. The CLI button is **Give CLI access**. `/mcp` without a device code stays the documentation page.
- The poll response returns the secret once, plus project and tenant grant metadata. The CLI stores that secret apart from the CLI token and keeps a default project only when the grant selected exactly one project.
- `apito mcp serve` passes the MCP token, the account endpoint, the edition derived from server capabilities, and an explicit project allowlist. It exits when no safe project is available.
- The account list matches `apito login`: saved accounts, then Local, Cloud, and Custom. The choice builds the URL (`http://localhost:4055/mcp`, `https://api.apito.io/mcp`, or the custom origin plus `/mcp`).
- The printed JSON is `{ "mcpServers": { "apito": { "type": "http", "url": "…" } } }`. Cursor, Claude, OpenAI, and VS Code use that same document. It contains no token.
- `APITO_MCP_RUNTIME=node` is the retired stdio bundle. Connect does not install it.

## Invariants

- Generated `apito mcp config` prints the HTTP JSON and does not embed `apt_` or `ak_`. `--transport stdio` is the retired Node command. See [remote-mcp-oauth](remote-mcp-oauth.md).
- `APITO_MCP_RUNTIME=node` can still launch a local bundle. Connect does not download one.
- Local credential deletion does not revoke the issued server token.

## Related

- [cli-contract](cli-contract.md)
