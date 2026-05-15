# task-1-openclaw-install-and-agent-config-jobs

Purpose:
- Make Configure OpenClaw able to install/configure OpenClaw through closed typed jobs after the job/policy substrate exists.

Scope:
- Add typed jobs for OpenClaw plugin install/config and OpenClaw agent config using signed manifest/artifact binding and approved config paths.
- Preserve non-sudo normal Configure OpenClaw behavior after enrollment.

Out of scope:
- No arbitrary runtime ownership, remote-host setup, shell command channel, or merged Helper/Remote Agent rail.
- No Borgee plugin channel binding; that is task 2.

Depends on:
- Phase 1 `milestone-2-typed-job-policy-loop` task set accepted

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §7 and §9

Acceptance slice:
- A reviewer can verify OpenClaw install/config jobs are closed typed jobs bound to signed artifacts and approved paths, with no client-supplied command or service unit authority.

Parallelism:
- First task after typed job/policy dependency clears. Blocks channel-binding and terminal UI closure.

Sensitive paths:
- dangerous-commands, credentials, host file authority, service authority
