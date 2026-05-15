# task-2-helper-credential-rotation-and-revoke

Purpose:
- Make Helper credential lifecycle and revoke/uninstall authority enforceable before job execution exists.

Scope:
- Add helper credential issuance/rotation/stale-device semantics at task-contract level.
- Add revoke/uninstall state handling that blocks future Helper authority and preserves rail separation.

Out of scope:
- No typed job execution, helper polling loop, service operations, or OpenClaw configuration action.
- No Remote Agent token reuse or merged grants.

Depends on:
- `task-1-helper-enrollment-model-and-status`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §5 and §10
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify stale, revoked, or uninstalled Helper authority cannot be used for future host-management work and cannot fall back to Remote Agent credentials.

Parallelism:
- Can run after task 1. Can run alongside task 3 if credential/status files do not conflict.

Sensitive paths:
- auth, credentials, revocation, remote-agent, data isolation
