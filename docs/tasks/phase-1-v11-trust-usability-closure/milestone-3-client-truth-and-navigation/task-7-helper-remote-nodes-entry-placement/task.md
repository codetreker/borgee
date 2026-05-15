# task-7-helper-remote-nodes-entry-placement

Purpose:
- Place Helper/Remote Nodes entry points coherently without merging Helper and Remote Agent authority.

Scope:
- Move or regroup Helper/Remote Nodes entry points into Settings or another runtime-management surface chosen at task design time.
- Preserve separate Helper actuator and Remote Agent file-proxy credentials, grants, and enforcement rails.

Out of scope:
- No Helper/Remote Agent rail merge, credential reuse, or host-management implementation work.
- No broad sidebar redesign.

Depends on:
- `task-5-sidebar-footer-primary-entries`

Blueprint anchors:
- `IA-1`: `docs/blueprint/next/migration-analysis.md` §7.3
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can find Helper/Remote Nodes entry points in the chosen IA location and verify the move does not imply shared credentials, grants, or enforcement rails.

Parallelism:
- Can run after task 1. Can run alongside task 2 if file ownership is separable.

Sensitive paths:
- auth, credentials, remote-agent, rail separation
