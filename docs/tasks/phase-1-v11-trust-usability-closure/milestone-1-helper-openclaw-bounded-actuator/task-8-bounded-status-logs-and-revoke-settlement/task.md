# task-8-bounded-status-logs-and-revoke-settlement

Purpose:
- Make job status, logs, and revoke/uninstall races settle truthfully.

Scope:
- Add queued/running/succeeded/failed/cancelled/expired terminal semantics, failure reasons, bounded redacted logs, and revoke/uninstall precedence for queued, leased, and running work.
- Ensure failed or revoked work cannot look successful or spin indefinitely.

Out of scope:
- No new user-facing privacy/compliance product surface.
- No OpenClaw closure action beyond generic job status semantics.

Depends on:
- `task-6-helper-pull-lease-result`
- `task-7-local-policy-manifest-and-sandbox-profile`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §10 and §11
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify revoked, stale, denied, expired, and failed work produce deterministic terminal state and bounded logs without exposing tokens, secrets, or private content.

Parallelism:
- Runs after Helper pull and local policy tasks because it depends on both result reporting and policy denial shape.

Sensitive paths:
- privacy, credentials, logs, revocation, host authority
