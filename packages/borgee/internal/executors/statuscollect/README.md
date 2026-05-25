# status.collect executor

Implements `JobTypeStatusCollect` for the borgee daemon dispatcher (PR-3
#1041). Runs inside the `borgee daemon` process as `User=borgee`; no root,
no rootd companion.

## Payload

```json
{ "scope": "helper" | "openclaw" | "service" }
```

Schema is enforced by `jobpolicy.validatePayload`; the executor re-validates
defensively (same allow-list).

## What it does

- Collects read-only system info for the requested scope:
  - `helper`: pid, GOOS/GOARCH, Go runtime version, executable path
  - `openclaw`: parsed `installed-versions.json` snapshot (best-effort)
  - `service`: uptime from `/proc/uptime` on Linux (best-effort)
- Returns the JSON snapshot in `outbound.ResultSummary.LogRefs[0]` so the
  server-recorded terminal status carries the helper's observation. NO
  filesystem write — status.collect is "read + report", not "write a
  cache". The server is the authority for status persistence.

## Path resolution

`status.collect` reads only well-known read-only paths (`/proc/uptime`,
the install-butler-maintained `installed-versions.json`). It does not need
a manifest-declared write target — the schema for this job type does not
include any path binding, and `jobpolicy.requiresManifest` correctly
returns false.

## Failure codes

| code | meaning |
|---|---|
| `schema_invalid` | bad payload / unknown scope / nil job |
| `encode_failed` | JSON marshal of snapshot failed |
