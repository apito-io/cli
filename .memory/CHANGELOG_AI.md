# cli — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-07-20 — release v0.4.2

- **Changed:** Tag **v0.4.2** — ships `sync --type functions` + active-revision
  deploy parity; bump release workflow Go to 1.25.5.
- **Why:** Publish CLI so consumers can sync Logic functions between projects.
- **Affected:** all sync_function* modules, `sync.go` / `sync_graphql.go`,
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

Last Updated: 2026-07-20
