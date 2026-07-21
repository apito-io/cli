# cli — Current

**Branch:** `main`

## Working on

- **2026-07-21→22:** Schema sync reliability:
  - Nested field keys (`parent.identifier`) — no more false `update_field`
  - Optional destination-only **delete field** plan with scope prompt +
    `--include-deletes`
  - Apply deletes via `modelFieldOperation(type: delete)`
  - Uncommitted — ask before commit/push

## Next

- User rebuilds `go build -o apito-cli` and re-runs Protiva/Rosna schema sync
- Confirm commit of sync_diff / plan / schema / apply / graphql / sync.go

## Last Updated

2026-07-22
