# cli — AI Changelog

Not git history — the _reasoning_ behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-08-11 — skip multiline/media/geo leaves in schema sync

- **Changed:** `flattenModelFields` no longer walks `sub_field_info` under
  engine composites (`multiline`, `media`, `geo`). Plan text falls back to
  identifier when label is empty.
- **Why:** deep nesting (v0.4.7) surfaced fixed `html`/`markdown`/`text`
  children as fake `Add field ""` when one side omitted built-in leaves
  (e.g. Rosna `app_release_policy` message_bn/en).
- **Affected:** `sync_diff.go`, `sync_plan.go`, `sync_diff_test.go`.

---
## 2026-08-07 — v0.4.12 content sync survives per-document failures

- **Changed:** `copyModelContent` returns per-document failures instead of
  aborting the whole run; relation sync failures collected too. Summary block
  lists up to 20 failures and the command exits non-zero. `syncCmd` sets
  `SilenceUsage` so the flag dump no longer buries the real error.
- **Why:** one `document <id> not found` killed a 43-model content sync after
  copying 0 rows (engine fix: open-core **v1.8.11**).
- **Affected:** `sync_content.go`, `sync.go`. Tag **v0.4.12**.

---
## 2026-08-05 — v0.4.11 fix relation direction sync

- **Changed:** `ChangeUpdateConnection` when dest has peer edge with flipped
  `Type` or different relation; sync still calls `UpsertConnection`.
- **Why:** Prod edges typed `backward` on owning model looked "missing"; CLI
  planned phantom Adds that pro staging no-op'd.
- **Affected:** sync_diff.go, sync_apply.go, sync_plan.go, tests. Tag **v0.4.11**.

---

## 2026-08-01 — v0.4.10 system composites in sync validate

- **Changed:** `validateSyncModels` allows `multiline` / `media` / `geo` fields to carry fixed `sub_field_info` (engine system composites). Regression test for `tenant.bio`.
- **Why:** CLI preflight blocked Rosna sync on valid composite shapes the engine accepts.
- **Affected:** `sync_validate.go`, `sync_nested_order_test.go`. Release **v0.4.10**.

---

## 2026-07-27 — v0.4.9 nested apply order + dependency closure

- **Changed:** `buildSyncTasks` sorts by `fieldPathDepth` (parents before children; deletes deepest-first). Selection closes ancestor field adds. Local `validateSyncModels` preflight. Clearer stage/publish handoff + first-failure path.
- **Why:** Alphabetical `ParentField` staged `label_id` before `test_label_results` → engine `parent field not found in draft` on Prottoy.
- **Affected:** `sync_apply.go`, `sync_deps.go`, `sync_validate.go`, `sync_schema.go`, `sync_plan.go`, tests. Release **v0.4.9**.

---

## 2026-07-23 — false update_field after draft overlay (v0.4.8)

- **Changed:** Treat nil and empty `validation.locals` / fixed-list slices as equal in `validationEqualForSync`.
- **Why:** After a timed-out schema sync, destination live+draft compare treated draft `locals: null` vs live `locals: []` as real updates (~25 false `Update field` rows on Protiva).
- **Affected:** `sync_diff.go`, `sync_diff_test.go`. Released as **v0.4.8** (CLI-only; engine empty-ledger-on-timeout is a separate deploy).

---

## 2026-07-22 — nested schema sync depth

- **Changed:** Recursive flatten + Path keys; projectModelsInfo nested to depth 5; structural validation flags in sync equality; live nested exam probe.
- **Why:** Prod exam.routine.details empty children were invisible to sync while public GraphQL died.
- **Affected:** sync_graphql.go, sync_diff.go, sync_diff_test.go, sync_live_nested_test.go. Released as v0.4.7 with open-core 1.8.3 / engine 2.4.18.

---
