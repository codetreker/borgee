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
