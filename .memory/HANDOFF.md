# cli — Handoff

## Branch

- `main` (dirty: schema sync nested keys + deletes)

## Done (2026-07-21→22)

- `fieldSyncKey` / `fieldMap` use `parent.identifier`
- `computeSchemaDeleteDiff` for destination-only fields
- Scope prompt: additive | full | deletes-only | cancel
- `--include-deletes`; `--yes` skips deletes unless that flag is set
- `SyncGraphQLClient.DeleteField` → `modelFieldOperation(type: delete)`
- Tests: nested collision, Protiva-style deletes, task order

## Broken / watch

- Nested flatten is still one `SubFieldInfo` level (same as add path)
- Model-level delete / relation delete still out of scope

## Next

- Commit/push after user confirmation
- Smoke Protiva sync: expect teacher deletes when Full/Deletes-only chosen

## Do not touch

- Do not default `--yes` to include deletes

## Last Updated

2026-07-22
