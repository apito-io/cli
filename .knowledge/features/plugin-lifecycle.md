---
type: feature
title: Plugin Lifecycle
description: Account-scoped plugin path, signed catalog install, and from/to deploy
resource: plugin_ops.go
tags: [cli, plugins, deploy, build]
timestamp: 2026-10-31T00:00:00Z
---

# Plugin Lifecycle

## Purpose

Every `apito plugin` command resolves an account first. That account chooses the plugin directory. A local account uses `~/.apito/plugins` unless `--dir` is set. A cloud account uses `~/.apito/plugins/<account>`.

## Flows

- **Status**: `apito plugin` and `apito plugin status [id]` print one table. LOCAL is the account directory and its `config.yml` version. REMOTE, INSTALLED, and ACTIVATED come from `GET /system/plugin` on that account. A directory with no engine copy is `local yes`, `remote no`, `installed no`, `activated no`.
- **Create**: `apito plugin create [name]` clones the Go hello-world plugin into `<path>/hc-<name>`.
- **Build**: `apito plugin build [name]` runs `go vet ./...`, then the Go build. Server binaries stay linux/amd64 when that platform is selected.
- **Add**: `apito plugin add <id-or-github-url>` posts `{id}` to `POST /system/plugin/install`. A GitHub URL contributes only the repository name. The engine downloads the signed release. The CLI does not fetch the zip. A local account then copies the installed bundle into its plugin path.
- **Remove**: `apito plugin remove <id>` calls `DELETE /system/plugin/:id`.
- **Deploy**: `apito plugin deploy [id] --from <account> --to <account>`. Catalog ids install with the signed registry. An unpublished plugin uploads only when the destination has `PLUGIN_ALLOW_LOCAL_UPLOAD`. Cloud to local uses `GET /system/plugin/:id/archive`. Secret env values are blank unless `--include-secrets`.
- **Project enable**: `apito plugin upsert|activate|deactivate <id> --project <project-id>` calls `upsertPlugin` on `/system/graphql`. Activate sets `enable: true`. Deactivate sets `enable: false` and leaves the engine install in place. The command does not send env values.
- **Catalog update**: `apito plugin update <id>` calls `POST /system/plugin/:id/update`. The engine installs the approved signed release. This does not flip a project's enable flag.

## Main files

- `plugin_account.go` — account and path
- `plugin_ops.go` — list, build path, add, remove, deploy
- `plugin_add.go` — catalog install request
- `plugin_build.go` — `go vet` and build

## Dependencies

- Engine plugin registry and `GET /system/plugin/:id/archive`
- Access-token preset `cli_plugin` (`plugins.read`, `plugins.deploy`)

## Invariants

- Do not download GitHub release assets in the CLI. The engine verifies the catalog signature and checksum.
- `cli_login` stays `projects.read` and `schema.read`. Plugin install asks for the `cli_plugin` preset.
- Install still needs a super-admin issuer plus `plugins.deploy`.
- Do not turn on `PLUGIN_ALLOW_LOCAL_UPLOAD` from the CLI. When an unsigned upload is refused, warn that hosted Apito keeps it off. Only the operator of a self-hosted or enterprise engine sets it true.

## Related

- [project-and-plugin-create](project-and-plugin-create.md)
