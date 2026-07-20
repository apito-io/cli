# cli — Handoff

## Branch
- `main` → release **v0.4.2**

## Done (2026-07-18→20)
- `sync --type functions` in menu (schema → functions → content)
- Flags: `--dir`, `--deploy`, `--include-secrets`; reserved account `local`
- Modules: `sync_functions.go`, `sync_function_fs.go`, `sync_function_diff.go`
- GraphQL: `ProjectFunctionsInfo` / `UpsertFunction` / `DeployFunction` +
  `active_revision_hash` deploy parity
- Local layout: `{dir}/{name}/meta.json` + `source.ts`
- Docs: `.knowledge/features/apito-sync.md`; tests: `sync_functions_test.go`
- CI: release workflow Go bumped to **1.25.5** (matches `go.mod`)

## Broken / watch
- Local→project requires `--dir`; both sides cannot be `local`
- Function definition sync is tenant-agnostic; pass tenant at invoke/test time
- GoReleaser needs `GORELEASER_GITHUB_TOKEN` for GitHub release + homebrew-tap

## Next
- Verify https://github.com/apito-io/cli/releases/tag/v0.4.2 after push

## Do not touch
- Plan `.cursor/plans/mcp_cli_functions_74d67203.plan.md` unless user asks

## Last Updated
2026-07-20
