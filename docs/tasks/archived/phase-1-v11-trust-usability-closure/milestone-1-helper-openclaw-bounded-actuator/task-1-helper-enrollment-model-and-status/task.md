# task-1-helper-enrollment-model-and-status

Purpose:
- Establish Helper as a distinct enrolled host-management authority before any typed job can run.

Scope:
- Add the Helper enrollment record, helper device identity, owner/org/host binding, allowed job category shape, and server-side visibility authority.
- Preserve separation from Remote Agent file-proxy credentials and grants.
- Sync `docs/current` for the accepted enrollment identity/status foundation if this task changes accepted behavior.

Out of scope:
- No job queue, lease/result contract, service lifecycle action, or Configure OpenClaw execution.
- No new user-facing privacy/compliance product surface.

Depends on:
- none

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.1-§1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §5
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can prove Helper enrollment has its own identity, owner/org binding, host/device status shape, allowed job categories, and matching current-doc sync without reusing Remote Agent credentials.

Parallelism:
- First ready task for this milestone. Blocks later credential lifecycle and status UI tasks.

Sensitive paths:
- auth, credentials, privacy, remote-agent, data isolation
