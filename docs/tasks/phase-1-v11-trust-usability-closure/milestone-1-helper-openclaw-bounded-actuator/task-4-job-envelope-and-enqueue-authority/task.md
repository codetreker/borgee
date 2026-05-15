# task-4-job-envelope-and-enqueue-authority

Purpose:
- Let Web requests create only server-authorized, schema-bound Helper jobs.

Scope:
- Define the typed job envelope, enqueue gate, owner/org/enrollment/delegation/job-type/revocation checks, idempotency, TTL, and terminal failure shape at task-contract level.
- Reject client-supplied shell, argv, executable path, script, service unit, arbitrary path, or arbitrary network domain.

Out of scope:
- No Helper polling client, local execution, service lifecycle action, or OpenClaw closure UI.

Depends on:
- `task-1-helper-enrollment-model-and-status`
- `task-2-helper-credential-rotation-and-revoke`
- `task-3-helper-status-ui-and-current-sync`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §6 and §7
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can prove enqueue accepts only authorized typed jobs and rejects unknown job types, extra fields, and client-supplied command/service/path/domain authority.

Parallelism:
- First task after enrollment/status dependency clears. Blocks Helper pull and policy tasks.

Sensitive paths:
- auth, credentials, dangerous-commands, remote-agent, host authority, privacy
