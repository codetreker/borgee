# Milestone 3: Configure OpenClaw Closure

## Capability Goal

Deliver the user-visible Configure OpenClaw loop after the Helper job/policy substrate exists.

## Acceptance Boundary

Accepted by this milestone:

- Web can request OpenClaw plugin install/config, OpenClaw agent config, Borgee plugin connection, and channel binding through closed typed jobs with channel ACL/authority checks carried into binding work.
- Long-lived Helper/OpenClaw services restart after OS reboot and crash without making the installer a privileged persistent daemon.
- UI shows connected, failed, denied, revoked, and manual-debug states truthfully.

Rejected by this milestone:

- Remote-host setup, arbitrary runtime ownership, arbitrary command execution, or merged Helper/Remote Agent rails.
- New privacy/compliance product surface.

## Dependencies

| Dependency | Status | Handling |
|---|---|---|
| Helper enrollment/status | PLANNED | Required authority and visibility base |
| Typed job policy loop | PLANNED | Required before OpenClaw action execution |
| Boot/crash service packaging | PLANNED | Required for reliable user-visible runtime path |

## Task-Split Trigger

Run milestone breakdown after the typed job/policy loop is accepted or has task skeletons that make Configure OpenClaw closure executable without hidden prerequisites.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-openclaw-install-and-agent-config-jobs` | BLOCKED | Implement typed jobs for OpenClaw plugin install/config and agent config | Phase 1 milestone 2 task set accepted | no | after dependency clears |
| `task-2-borgee-plugin-channel-binding-job` | BLOCKED | Implement Borgee plugin connection and channel binding with channel ACL checks | `task-1-openclaw-install-and-agent-config-jobs` | yes, after task 1 | no |
| `task-3-service-lifecycle-boot-crash` | BLOCKED | Add non-sudo long-lived Helper/OpenClaw service boot/crash reliability | Phase 1 milestone 2 task set accepted | yes, after milestone 2 | no |
| `task-4-configure-openclaw-terminal-ui` | BLOCKED | Show Configure OpenClaw queued/running/succeeded/failed/denied/revoked/manual-debug states | `task-1-openclaw-install-and-agent-config-jobs`, `task-2-borgee-plugin-channel-binding-job`, `task-3-service-lifecycle-boot-crash` | no | no |

Dependency order: this milestone is reviewed now but remains blocked until the typed job policy loop has accepted task work. Tasks 2 and 3 may run in parallel after their dependencies clear if channel-binding and service-packaging files are independent.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Closure tasks stay on the typed job substrate and preserve Helper/Remote Agent rail separation. |
| PM | LGTM | The user-visible Configure OpenClaw path reaches install/config, binding, service lifecycle, and terminal UI. |
| QA | LGTM | Terminal states are checkable for queued/running/succeeded/failed/denied/revoked/manual-debug outcomes. |
| Dev | LGTM | Each task is one-PR sized; channel binding and service lifecycle can split after typed-job prerequisites. |
| Security | LGTM | Host actions, channel ACL, credentials, and service lifecycle are carried as sensitive paths. |
