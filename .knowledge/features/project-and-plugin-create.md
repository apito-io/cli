---
type: feature
title: Project and Plugin Create
description: Scaffold new Apito projects and plugin directories from CLI
resource: create.go
tags: [cli, create, scaffold]
timestamp: 2026-07-07T00:00:00Z
---

# Project and Plugin Create

## Purpose

`apito create` scaffolds new **projects** or **plugin** starter trees with expected `config.yml`, language layouts, and hooks into build/deploy commands.

## Flows

- **Project create**: interactive or flags → template files → optional link to engine account.
- **Plugin create**: starter for Go/JS plugin SDK layout → next steps `plugin build` / `deploy`.
- **Related**: `plugin add` for registering existing plugin metadata.

## Main files

- `create.go`, `plugin_add.go`
- Plugin templates referenced from create flow
- Engine project creation (server-side) via API when hosted create used

## Dependencies

- [plugin-lifecycle](plugin-lifecycle.md)
- Engine [multi-tenancy-saas](../engine/.knowledge/features/multi-tenancy-saas.md) for SaaS project types

## Invariants

- Generated plugin ids must be unique per engine account.
- Scaffold matches plugin build SDK expectations — do not hand-delete required files.

## Common bugs

- Create locally but never deploy/register on engine → plugin invisible to loader.
- Wrong language template → build command missing runtime.

## Tests

- Manual scaffold + `plugin build` smoke

## Related

- Global [plugin-grpc-protocol](../../.knowledge/features/plugin-grpc-protocol.md)
- [admin-and-ops](admin-and-ops.md)
