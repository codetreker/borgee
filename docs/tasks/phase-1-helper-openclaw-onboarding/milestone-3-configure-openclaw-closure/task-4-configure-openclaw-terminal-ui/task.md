# task-4-configure-openclaw-terminal-ui

Purpose:
- Show Configure OpenClaw completion, denial, failure, revocation, and manual-debug states truthfully.

Scope:
- Add UI/API behavior for queued/running/succeeded/failed/denied/revoked/manual-debug states and bounded redacted logs for Configure OpenClaw.
- Ensure success appears only after required OpenClaw/plugin/channel/service closure conditions are met.

Out of scope:
- No broad onboarding copy rewrite, visual redesign, or privacy/compliance product surface.

Depends on:
- `task-1-openclaw-install-and-agent-config-jobs`
- `task-2-borgee-plugin-channel-binding-job`
- `task-3-service-lifecycle-boot-crash`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §4 and §11
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify Configure OpenClaw never shows false success, exposes failure reasons and bounded logs, and distinguishes revoked/denied/manual-debug states.

Parallelism:
- Runs after action and service tasks because it aggregates terminal closure state.

Sensitive paths:
- privacy, logs, status truthfulness, host authority
