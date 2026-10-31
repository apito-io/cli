# cli — AI Changelog

## 2026-10-29 — CLI v0.5.7

- **Changed:** `apito mcp serve` bridges to the engine HTTP MCP endpoint.
  Studio nginx forwards the public host port. Tagged and pushed `v0.5.7`
  (`c87c3f9`).
- **Why:** Serve could still launch the retired `apito-mcp` package, and
  `$host` dropped `:4055`, so token exchange failed.
- **Affected:** `mcp_cmd.go`, `studio.nginx.conf`. GitHub release succeeded.
  Ask before `./command.sh save`.

## 2026-10-27 — CLI v0.5.2

- **Changed:** Committed the Studio upgrade prompt, nginx port fix, busy-port prompt, and opt-in telemetry as `40d170a`. Tagged **v0.5.2** locally. Added `CHANGELOG.md` and a README v0.5.2 section.
- **Why:** `apito start` needed a released binary that asks before replacing a pinned Studio image and does not let nginx take the engine port.
- **Affected:** `apito/cli` `main`. Live start kept `studio_version: v0.1.8`. Not pushed. Ask before `./command.sh save`.

## 2026-10-25 — Studio nginx leaves 5055 for the engine

- **Changed:** `apito start` writes and mounts `studio.nginx.conf` so nginx listens on 4055 and proxies API paths, including `/admin`, to the engine on 5055.
- **Why:** Image `v0.1.8` ran nginx on 5055 and proxied to 5051. The engine then failed with `listen tcp :5055: bind: address already in use`, and `http://localhost:4055` never answered.
- **Affected:** `studio.nginx.conf`, `studio_nginx.go`, `studio_runtime.go`. Restarted Studio and confirmed console 200, `/admin` reached the engine, and the engine stayed bound to 5055. Uncommitted. Ask before `./command.sh save`.

## 2026-10-25 — Studio start checks ports and pings before URLs

- **Changed:** `apito start` checks host ports 4055 and 5055, asks to stop, kill, or skip when one is busy, then pings the console, `/admin`, and the engine before printing URLs.
- **Why:** Start printed the console URL while an old container still held the ports, and it did not notice when the new container's engine failed to bind.
- **Affected:** `studio_ports.go`, `studio_runtime.go`, `studio_ports_test.go`, local orchestration feature doc. Focused tests passed. Uncommitted. Ask before `./command.sh save`.

## 2026-10-25 — Studio upgrade prompt shows current image

- **Changed:** The upgrade prompt prints the current Studio image version beside the latest tag. A release pin stays as that tag. A non-release pin such as `local` includes the image build date, and a matching local release tag when one exists.
- **Why:** `local` is a pin, not a version, so the first prompt did not say what image was already on the machine.
- **Affected:** `studio_runtime.go`, `version_manager.go`, `version_manager_test.go`, local orchestration feature doc. Focused tests passed. Uncommitted. Ask before `./command.sh save`.

## 2026-10-24 — Studio image upgrade check

- **Changed:** `apito start` queries public GHCR tags, prompts before upgrading pinned Studio images, persists a confirmed tag only after pulling it, and keeps a cached current image when refresh fails. Added unit tests and updated CLI/local orchestration docs.
- **Why:** `studio_version: local` previously attempted only the nonexistent remote `:local` tag, then silently started the old cached image without checking for a newer published Studio image.
- **Affected:** `studio_runtime.go`, `version_manager.go`, `config.go`, `version_manager_test.go`, CLI README, local orchestration feature doc. `go test ./...` passed; `go vet ./...` still reports existing unreachable code in `stop.go`. Uncommitted. Ask before `./command.sh save`.

## 2026-10-19 — CLI v0.5.0

- **Changed:** Release the unified CLI: browser login, `apito types` / `apito gen`, MCP connect, and capability-gated project, key, tenant, backup, and restore commands. Login stores `cli_login` (`projects.read` and `schema.read`). Type generation reads published schema and refuses credential output.
- **Why:** Suchok and the other apps should generate types from the installed CLI, and a stolen login token must not read tenant documents or change the system.
- **Affected:** CLI `main`. Tag **v0.5.0**. Needs an engine that mints `cli_login` and allows pure `__schema` / `__type` queries with `schema.read`.

## 2026-10-19 — Low-harm login contract

- **Changed:** Documented that `apito login` mints `cli_login` and that type generation uses schema introspection without `data.read`.
- **Why:** A stolen login token must not read tenant documents or change the system.
- **Affected:** `cli/.knowledge/features/cli-contract.md`. Uncommitted. Ask before `./command.sh save`.

## 2026-10-18 — Cloud MCP HTTP URL

- **Changed:** Cloud HTTP config and token exchange use `https://api.apito.io/mcp` and `https://api.apito.io/oauth/token`.
- **Why:** `mcp.apito.io` does not resolve. Production advertises `MCP_PUBLIC_URL=https://api.apito.io`.
- **Affected:** `mcp_http.go`, `mcp_cmd.go`. Uncommitted. Ask before `./command.sh save`.

## 2026-10-18 — public README

- **Changed:** Database list, ports, license sentence, docs URL, and Discord invite in `README.md`.
- **Why:** The public CLI page still named SQL Server, Firestore, and Fair Source.
- **Affected:** `README.md` only. Uncommitted. Leave `.memory` out of the public commit.

## 2026-10-13 — opt-in HTTP MCP

- **Changed:** Added token-free HTTP config and a stdio-to-HTTP bridge. Default `mcp serve` remains the Node runtime.
- **Why:** Remote clients should receive a URL, while existing stdio installs keep working.
- **Affected:** `mcp_cmd.go`, `mcp_http.go`, `mcp_http_test.go`, `.knowledge/features/remote-mcp-oauth.md`. Targeted `go test` passed. Uncommitted.



Not git history — the _reasoning_ behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-10-13 — remote MCP URL/OAuth plan

- **Changed:** Refined token-free HTTP MCP configuration for Cloud, local Studio, and Apito-compatible custom hosts to use clean named client entries, a Go engine MCP route, and a Go stdio-to-HTTP bridge. `kind` is inferred from the endpoint; the Node runtime is only a temporary rollback path.
- **Why:** MCP clients should accept one URL like PostHog while the CLI keeps using the existing `mcp_assistant` token approval and stdio flow.
- **Affected:** Planning only; no CLI code changed in this pass. Proposed URLs are `https://mcp.apito.io/mcp` and `http://localhost:4055/mcp`. Ask before implementation or `./command.sh save`.

---
## 2026-10-13 — MCP connect

- **Changed:** `mcp connect` follows Local, Cloud, or Custom and stores the MCP secret in `mcp_accounts` or keychain service `apito-cli-mcp`. `mcp update` verifies a release archive into `~/.apito/mcp/versions/<version>`. `mcp serve` uses that bundle, the MCP token, and a fail-closed project. Local Studio env is engine `5055` and console `4055`.
- **Why:** The MCP token has a different capability set from the CLI token, and Cursor must launch a foreground stdio process that does not embed the secret.
- **Affected:** `mcp_connect.go`, `mcp_cmd.go`, `mcp_runtime.go`, `credentials.go`, `config.go`, `studio-oss` nginx via the CLI's published ports. `go test ./...` passed. Uncommitted. Ask before `./command.sh save`.

---

## 2026-10-12 — unified CLI contract and tests

- **Changed:** Capability discovery calls the access-token catalog and self endpoint. `account test` caches capabilities. Key revoke accepts a token id argument, and create can write the new `ak_` secret to an env file only with `--yes`. Gen dry-run can show `general`, `saas-shared`, or `saas-per-tenant`. Fake-server tests cover login expiry, fail-closed Cloud commands, restore disabled, MCP edition, and codegen artifact/module selection. README, architecture, and `features/cli-contract.md` document the contract.
- **Why:** The public CLI needs one written boundary and tests that prove Cloud commands do not fall through to private GraphQL.
- **Affected:** `capabilities.go`, `config.go`, `key_cmd.go`, `gen_cmd.go`, `cli_unified_test.go`, `README.md`, `.knowledge/`. `go test ./...` passed. Uncommitted. Ask before `./command.sh save`.

---
## 2026-10-12 — unified CLI commands

- **Changed:** Named accounts keep the `apt_` token out of `config.yml`. Login, project, key, MCP, gen, tenant, client, backup, and restore commands share one account resolver. Missing Cloud capabilities fail closed.
- **Why:** One Apache CLI has to talk to a local engine and to Apito Cloud without copying private host code or printing secrets into MCP config and generated files.
- **Affected:** `system_client.go`, `credentials.go`, `capabilities.go`, `login.go`, `project_cmd.go`, `key_cmd.go`, `mcp_cmd.go`, `gen_cmd.go`, `tenant_cmd.go`, `cloud_cmd.go`, `config.go`. Uncommitted. Not tagged. Ask before `./command.sh save`.

---
## 2026-10-11 — Fair Source user strings

- **Changed:** Dump confirmation compares against `tenants`. Sync status text says the change is staged as a draft. README explains `apito sync` from a local engine to Apito Cloud.
- **Why:** The public CLI should not name private catalog tables or a Studio pin.
- **Affected:** `dump.go`, `sync.go`, `sync_apply.go`, `sync_plan.go`, `dump_preflight.go`, `README.md`. `go build` succeeded. Not tagged. Ask before `./command.sh save`.

---
## 2026-10-11 — schema sync stages cardinality changes (v0.4.17)

- **Changed:** A relation whose forward or reverse type changes storage is staged with `previewSchemaRelationTransition` and `stageRelationTransition`. New relations and same-shape repairs still use `upsertConnectionToModel`. Sync still does not publish.
- **Why:** Upserting a new cardinality left the old foreign key in place. The engine now rejects that upsert.
- **Affected:** `sync_diff.go`, `sync_apply.go`, `sync_graphql.go`. Tag **v0.4.17**. Needs engine **v2.5.0** or newer.

---
## 2026-08-13 — dump hash version gate (v0.4.16)

- **Changed:** Preflight reads `schema_hash_version`; fail with an algorithm-mismatch error when source/dest engines disagree (0 = older than v2.4.40).
- **Why:** Remote Studio still on v2.4.38 hasher while local is v2.4.39+ looks like a schema mismatch even when GraphQL sync is clean.
- **Affected:** `dump_preflight.go`. Needs engine **≥v2.4.40** on both sides.

---
## 2026-08-13 — dump tenant picker (v0.4.15)

- **Changed:** Search tenants or enter domain; one correlator for FROM and TO; print FROM/TO summary. Needs engine **≥v2.4.39** (dump hash).
- **Why:** SaaS per-tenant dump should pick like a tenant search, not a bare prompt.
- **Affected:** `dump.go`, `dump_preflight.go`.

---
## 2026-08-13 — apito dump (v0.4.14)

- **Changed:** `apito dump` pulls a portable project/tenant DB (Range download, chunked import). Preflight: profile, PortableDump, schema hash, dest tenant by domain. Prints dest system-DB write/skip list. Push to non-local dest needs `--allow-push`.
- **Why:** Replace a whole physical DB across installs without copying the instance system file.
- **Affected:** `dump.go`, `dump_transport.go`, `dump_preflight.go`, `sync_graphql.go` (`SearchTenantsByDomain`). Needs engine **≥v2.4.38**.

---
## 2026-08-12 — filesystem → project schema sync (Blueprints)

- **Changed:** `apito sync --from filesystem --type schema --dir <blueprint>`
  loads portable `schema.json` (+ optional `config.yml`), rejects secrets,
  sanitizes `system_*` / stale `parent_field`, reuses draft staging apply.
  Export via `--to filesystem`. Feature doc updated.
- **Why:** Ship Apito Blueprints without a second importer; Console remains
  publish gate.
- **Affected:** `sync.go`, `sync_schema.go`, `sync_schema_fs.go`,
  `sync_schema_merge.go`, `.knowledge/features/apito-sync.md`. Uncommitted
  pending separate ask (website-astro-only turn).

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
