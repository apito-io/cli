# cli — Handoff

## Branch

- `main` — tag **v0.4.9**

## Done (2026-07-27 — v0.4.9)

- Fixed `buildSyncTasks` alphabetical ParentField bug (child-before-parent)
- `closeFieldDependencies` auto-includes ancestor adds
- `validateSyncModels` rejects list-with-children + stale parent_field
- Prottoy-shaped regression tests; schema.json validates clean

## Next

- User: rebuild/install CLI, inspect prod draft, re-sync Prottoy, publish in Console

## Do not touch

- Do not default `--yes` to include deletes
- Do not soft-succeed field "already exists in draft"

## Last Updated

2026-07-27
