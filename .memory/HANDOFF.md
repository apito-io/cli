# cli — Handoff

## Branch

- `main` clean — tagged **v0.4.8** (pushed)

## Done (2026-07-23 — v0.4.8)

- `validationEqualForSync`: nil/empty `Locals` and empty fixed-list slices match
- Unit test `TestFieldsMatchForSync_EmptyLocalsNilVsSlice`
- Confirmed live-only vs live+draft: false updates drop from 25 → 0

## Done (2026-07-22→23 — v0.4.7)

- Recursive flatten + full dotted `Path` for sync keys
- `projectModelsInfo` nested `sub_field_info` to depth 5 + validation selection
- Prior: optional deletes, soft-success model-only already-exists (v0.4.6)

## Broken / watch

- Empty execution ledger with pending ops is an **engine** staging order bug
  (flush before ledger refresh) — not fixed in this CLI release
- `parent_field` still immediate id — colliding nested parents can mis-target
- Nesting deeper than 5 under root truncated by GraphQL selection

## Next

- Discard empty-ledger drafts; re-sync with v0.4.8
- Deploy engine StageMutation fix separately when ready

## Do not touch

- Do not default `--yes` to include deletes

## Last Updated

2026-07-23
