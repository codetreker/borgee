# task-3-service-lifecycle-boot-crash

Purpose:
- Make long-lived Helper/OpenClaw services reliable without making privileged installers persistent.

Scope:
- Add boot restart, crash restart, bounded restart/backoff, and declared service ID handling for enrolled Helper/OpenClaw services.
- Keep long-lived services non-sudo and keep `install-butler` short-lived and visible.

Out of scope:
- No boot-time installer, sudo cache, arbitrary local service restart, or Remote Agent rail merge.
- No product promise for Teamlead cron concepts.

Depends on:
- Phase 1 `milestone-2-typed-job-policy-loop` task set accepted

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §9 and §12

Acceptance slice:
- A reviewer can verify only declared enrolled Helper/OpenClaw services receive non-sudo boot/crash handling and privileged installer behavior stays short-lived.

Parallelism:
- Can run after milestone 2. May run alongside task 2 if service packaging and channel binding files are independent.

Sensitive paths:
- service authority, dangerous-commands, privilege boundary, host authority
