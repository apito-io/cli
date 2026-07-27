# cli — AI Changelog

Not git history — the _reasoning_ behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

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

## 2026-07-21→22 — schema sync nested keys + optional deletes

- **Changed:** Nested field sync keys by `parent.identifier`; detect
  destination-only fields as optional `delete_field`; interactive scope prompt
  and `--include-deletes`; apply via `modelFieldOperation(type: delete)`.
- **Why:** Stop Rosna false update noise; surface Protiva teacher fields removed
  locally but still on prod, without auto-applying destructive deletes.
- **Affected:** `sync_diff.go`, `sync_plan.go`, `sync_schema.go`, `sync_apply.go`,
  `sync_graphql.go`, `sync.go`, `sync_diff_test.go`. Released earlier in v0.4.5–0.4.6 line.

## 2026-07-21 — access-token header contracts

- **Changed:** Added focused tests for canonical project/tenant/headless headers
  and retired `cli-` / `sdk-` / `mcp-` token recognition.
- **Why:** Lock CLI parity with SDK access-token request scoping without aliases.
- **Affected:** `sync_graphql_headers_test.go` only; no runtime behavior change.

## 2026-07-20 — release v0.4.2

- **Changed:** Tag **v0.4.2** — ships `sync --type functions` + active-revision
  deploy parity; bump release workflow Go to 1.25.5.
- **Why:** Publish CLI so consumers can sync Logic functions between projects.
- **Affected:** all sync_function\* modules, `sync.go` / `sync_graphql.go`,
  `.github/workflows/release.yml`, GoReleaser GitHub release + homebrew-tap.

---

## 2026-07-18 — sync --type functions

- **Changed:** Third sync type `functions` — project↔project transfer of Logic
  draft source + metadata; local import/export via `local` + `--dir`; optional
  `--deploy` after upsert; `--include-secrets` opt-in. Diff by name
  (add/update/capability drift). Feature doc + unit tests.
- **Why:** Move Deno/TS functions between projects and local worktrees without
  copying engine artifact stores.
- **Affected:** `sync.go`, `sync_graphql.go`, `sync_functions.go`,
  `sync_function_fs.go`, `sync_function_diff.go`, `sync_functions_test.go`,
  `.knowledge/features/apito-sync.md`

---

## 2026-07-06

- **Changed:** Bootstrapped knowledge system for this repo.
- **Why:** Cross-LLM durable knowledge + working memory.
- **Affected:** this repo only.

Last Updated: 2026-07-21
