---
type: feature
title: Remote MCP OAuth
description: URL-only MCP clients use the engine OAuth and Streamable HTTP endpoint
resource: mcp_cmd.go
tags: [cli, mcp, oauth]
timestamp: 2026-10-13T00:00:00Z
---

# Remote MCP OAuth

## Purpose

A remote MCP client configures an Apito URL and completes the existing Console approval. The engine stores the operator grant and gives the client a resource-bound credential.

## Contract

- Cloud: `https://api.apito.io/mcp`. `https://mcp.apito.io/mcp` is still recognized as Cloud, and it is not the live host.
- Local Studio: `http://localhost:4055/mcp`, proxied to engine `5055`
- Custom: the account origin plus `/mcp`
- The host selects the deployment. `kind` does not change the OAuth resource.
- Client config is `{ "type": "http", "url": "…" }`. It does not contain `apt_` or `ak_`.
- `GET /mcp?device_code=` and `GET /mcp?request_id=` stay on the Console page. Protocol methods go to the engine.

## Flows

- The client reads protected-resource and authorization-server metadata, registers a public client, and starts authorization-code with S256 PKCE.
- The Console reuses `CreateAccessTokenForm` locked to `mcp_assistant`. Approval posts to `/system/oauth/mcp/approve` and redirects to the client with a one-time code.
- The token endpoint returns `mct_` and `mcr_` credentials bound to that resource. `/mcp` rejects an `apt_` bearer.
- `apito mcp serve` stays on the Node runtime unless `--transport http` or `APITO_MCP_RUNTIME=http` is set. That mode exchanges the stored grant for `mct_` and forwards stdio frames. `APITO_MCP_RUNTIME=node` forces the TypeScript runtime.

## Related

- [mcp-connect](mcp-connect.md)
