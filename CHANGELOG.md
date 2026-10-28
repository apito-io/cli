# Changelog

## v0.5.4

- Studio console env points the content explorer at `/secured/graphql`. The system explorer stays on `/system/graphql`.

## v0.5.3

- `apito create` sends `X-Use-Cookies: false` and exits non-zero when the project is not created.
- `apito sync` skips project fields this engine does not have, and the token hint appears only on 401 or 403. A missing account prints the login hint once.
- `apito mcp` uses the active account, or the only account, instead of the name `default`.
- `apito self-upgrade` writes the new binary beside the current one and renames it into place. `--yes` skips the prompt.
- `apito status` fails when the console on port 4055 is not answering.
- `apito start --db` accepts `--db-engine` and `--yes` without a terminal prompt.

## v0.5.2

- `apito start` checks public GHCR Studio tags. A pinned tag shows the current image version and asks before upgrade. Declining keeps the current image. A failed refresh uses the cached image.
- Start mounts an nginx config so the console stays on port 4055 and the engine keeps port 5055. Images through v0.1.8 otherwise bind nginx to the engine port.
- A busy 4055 or 5055 names the Docker container or process and asks to stop it, kill it, or skip.
- Anonymous CLI usage is off unless you answer yes. `DO_NOT_TRACK=1`, `APITO_TELEMETRY=0`, and `apito telemetry disable` keep it off.
