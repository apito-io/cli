# Changelog

## v0.5.2

- `apito start` checks public GHCR Studio tags. A pinned tag shows the current image version and asks before upgrade. Declining keeps the current image. A failed refresh uses the cached image.
- Start mounts an nginx config so the console stays on port 4055 and the engine keeps port 5055. Images through v0.1.8 otherwise bind nginx to the engine port.
- A busy 4055 or 5055 names the Docker container or process and asks to stop it, kill it, or skip.
- Anonymous CLI usage is off unless you answer yes. `DO_NOT_TRACK=1`, `APITO_TELEMETRY=0`, and `apito telemetry disable` keep it off.
