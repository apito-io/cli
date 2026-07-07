---
type: feature
title: Version and Self Upgrade
description: Pin and update Apito CLI, engine, and console versions
resource: version_manager.go
tags: [cli, upgrade, version]
timestamp: 2026-07-07T00:00:00Z
---

# Version and Self Upgrade

## Purpose

CLI tracks compatible **engine** and **console** versions and can **self-upgrade** the `apito` binary via release artifacts (GoReleaser, Homebrew tap).

## Flows

- **Check**: `apito self-upgrade` — compare current vs released CLI.
- **Update components**: `apito update` — pull/build engine or console per flags.
- **Version manager**: centralizes pinned refs for local orchestration.

## Main files

- `self_upgrade.go`, `update.go`, `version_manager.go`
- `.goreleaser.yaml`, `.github/workflows/release.yml`
- `Formula/apito-cli.rb` — Homebrew

## Dependencies

- GitHub/GitLab release artifacts
- Studio `ENGINE_REF` / `CONSOLE_REF` (deploy pins, separate repo)

## Invariants

- CLI version skew with engine can break sync GraphQL operations — upgrade together when breaking changes ship.
- Self-upgrade replaces local binary — verify checksum/source (official releases only).

## Common bugs

- Old CLI against new pro schema versioning → sync apply errors until CLI updated.
- `update` without network/git credentials → partial upgrade state.

## Tests

- Manual: `apito self-upgrade` dry run on dev machine

## Related

- [local-dev-orchestration](local-dev-orchestration.md)
- `studio/.knowledge/features/engine-console-ref-pinning.md`
