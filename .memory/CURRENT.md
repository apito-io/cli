# cli — Current

**Branch:** `main` (clean @ **v0.4.8**)

## Working on

- **2026-07-23 released v0.4.8:** Treat nil vs empty `validation.locals` /
  fixed-list slices as equal so draft schemaPreview no longer invents dozens of
  false `update_field` diffs after a timed-out / partial sync.
- Prior **v0.4.7:** Deep nested schema sync (`projectModelsInfo` depth 5,
  recursive flatten + `Path` keys).

## Next

- Install **v0.4.8**; Discard broken empty-ledger drafts on prod then re-sync
- Engine still needs StageMutation refresh-before-flush deploy for timeout→empty
  plan (separate from this CLI release)

## Last Updated

2026-07-23
