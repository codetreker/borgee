# task-10-borgee-plugin-channel-binding-job

Purpose:
- Complete Borgee plugin connection and channel binding through typed jobs without weakening channel authority.

Scope:
- Add typed job coverage for Borgee plugin connection and channel binding.
- Carry owner/org/channel ACL checks into binding work and preserve Helper/Remote Agent rail separation.
- Use existing channel authority if sufficient; if the required channel authority is missing, this task must wait for or explicitly depend on canonical Milestone 2 channel authority work rather than bypassing ACL checks.

Out of scope:
- No channel management feature expansion beyond binding authorization needed for Configure OpenClaw.
- No arbitrary plugin/runtime command execution.

Depends on:
- `task-9-openclaw-install-and-agent-config-jobs`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §7
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify channel binding rechecks channel ACL/authority, cannot bind through owner/org/enrollment alone, and records any dependency on canonical Milestone 2 channel authority before implementation proceeds.

Parallelism:
- Can run after task 9. May run alongside task 11 if shared Configure OpenClaw files do not conflict.

Sensitive paths:
- auth, channel ACL, credentials, host authority, rail separation
