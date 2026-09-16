---
type: feature
title: Apito handover CLI
description: Resumable self-host Enterprise to Apito Cloud infrastructure migration
resource: cli/handover.go
tags: [cli, migration]
timestamp: 2026-09-06T00:00:00Z
---

# Apito handover CLI

## Purpose

Move a self-hosted project onto Apito Cloud. Not used for Cloud ownership change.

## Flows

```
apito handover plan
apito handover export
apito handover import --to apito-cloud
apito handover verify
apito handover cutover
apito handover resume --job <id>
```

Cutover: maintenance window, checksums, media copy, secret reauth, DNS switch, rollback window. Reuses `apito sync` and `apito dump`.

## Main files

- `cli/handover.go`
- `cli/sync.go`, `cli/dump.go`
- engine `controller/dump_controller.go`, `services/backupmgr/catalog.go`

## Dependencies

- [apito-sync](apito-sync.md)
- [freelancer-project-handover](../../.knowledge/features/freelancer-project-handover.md)

## Invariants

- Never export platform operators, freelancer billing, temporary credentials, backup-provider secrets, or machine keys.
- Cloud→Cloud transfer is in-place GraphQL/REST, not this CLI.

## Common bugs

- Running CLI dump for a Cloud owner change (causes downtime). Use `moveProjectToWorkspace`.

## Tests

- Manual: `apito handover plan`

## Related

- engine [pro-backups](../../engine/.knowledge/features/pro-backups.md)
