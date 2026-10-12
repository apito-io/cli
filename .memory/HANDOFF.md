# CLI — Handoff

## Fair Source copy (2026-10-11)

- **Done:** User strings no longer say pro engines or Studio `ENGINE_REF`. SaaS catalog replace confirms with `tenants`. README has a project-to-project sync section.
- **Not done:** A new tag. The running `apito` binary is still the last release until rebuilt.
- **Do not:** Change the confirm word back to `pro_tenants`. Tag from this alone. `./command.sh save` until asked.

## Released

- `main` — tag **v0.4.17** (schema sync stages cardinality changes). Needs engine **v2.5.0** or newer.
- Previous tag **v0.4.16** (dump hash algorithm version gate). Pair dump with engine **≥v2.4.40**.

## Already on main

- Filesystem→schema sync is in `084d722`. This release does not include the local `handover.go` draft.

## Last Updated
2026-10-11
