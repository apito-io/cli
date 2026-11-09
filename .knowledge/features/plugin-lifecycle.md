---
type: feature
title: Plugin Lifecycle
description: Local plugin scaffold, check, build, and git-tag publish
resource: plugin_dev.go
tags: [cli, plugins, deploy, build]
timestamp: 2026-11-09T00:00:00Z
---

# Plugin Lifecycle

## Purpose

`apito plugin` scaffolds, checks, builds, and publishes a plugin repository on this machine. It does not connect to an account or an engine. Install, activate, and catalog update stay in the console.

## Flows

- **List**: `apito plugin` and `apito plugin list` scan the current directory and `~/.apito/plugins` for `config.yml`, including plugins grouped one folder deeper. On a terminal the list is searchable by name and version. Selecting one offers check, build, or push. Push then asks for the commit message and patch, minor, or major. Without a terminal it prints name, version, and path. `--dir <path>` searches that directory instead.
- **Init**: `apito plugin init [name]` asks for `system` or `payment`, then clones that boilerplate. A name without `hc-` is prefixed. A path argument writes there. With no path it writes `./<name>`. `--dir <path>` selects the parent. System clones `apito-hello-world-go-plugin` (query, mutation, protected REST, project env, settings UI). Payment clones `apito-payment-plugin-template` (initialize, verify, and a cancel that returns `not supported`). Identity is rewritten and `.git` is removed.
- **Build**: `apito plugin build [dir]` runs `go vet` and `go build` in the plugin directory.
- **Check**: `apito plugin check [dir]` fails unless `config.yml` has `id`, `language: go`, `version`, `binary_path`, a github.com `repository_url`, the handshake cookie, and an `api` or `payment` contribution. `main.go` `sdk.Init` must match the id and version. `go.mod` and `.github/workflows/build-release.yml` are required. The workflow must trigger on tags `v*`, run `go test`, and check the tag against `config.yml` and `sdk.Init`. `ui/` and the entry file are required when `ui_config.entry_path` is set. `origin` must be a github.com remote.
- **Push**: `apito plugin push [dir]` is also `deploy`. It runs check, bumps the patch (`--minor` or `--major` otherwise), writes the version into `config.yml` and `sdk.Init`, and requires `-m`. Then it commits every change in the plugin directory, tags `v<version>`, and pushes the branch and the tag. The CLI does not upload a binary or sign the catalog.

## Main files

- `plugin_dev.go` — init, list, check, push
- `plugin_build.go` — `go vet` and build
- `plugin_account.go` — identity rewrite

## Dependencies

- Public boilerplates `apito-io/apito-hello-world-go-plugin` and `apito-io/apito-payment-plugin-template`
- The plugin repository's GitHub release workflow, which builds the zip and dispatches catalog sync

## Invariants

- Do not resolve an account for plugin commands. `apito create` still creates a project on a connected account.
- Do not POST a plugin binary. Publishing is a git tag. The catalog sync stays the GitHub workflow.
- A provider that cannot end a subscription returns `not supported` from its cancel operation.

## Related

- [project-and-plugin-create](project-and-plugin-create.md)
