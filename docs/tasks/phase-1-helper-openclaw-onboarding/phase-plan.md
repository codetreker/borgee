# Phase 1: Helper / OpenClaw Onboarding

> Superseded for v1.1 execution grouping. This file is retained as accepted planning history and task-detail context. Canonical grouping now lives in `docs/tasks/phase-1-v11-trust-usability-closure/phase-plan.md`, with these three old milestones collapsed into `milestone-1-helper-openclaw-bounded-actuator`. The collapse records that this was one prerequisite chain toward one user-facing Helper/OpenClaw loop, not three separate coarse milestones.

## Source Anchors

- `HB-RA-1A`: Helper bounded actuator product guardrails.
- `HB-RA-1B`: Helper actuator execution contract for implementation planning.
- `PS-1`: Privacy scope guard; preserve existing admin/privacy/security controls without adding new user-facing privacy/compliance product scope.
- Source issues: gh#681, gh#659, gh#654.

## Value Loop

A user enrolls Helper once on a host, then can configure OpenClaw from Web without a second SSH step. The user can see Helper connection state, allowed job categories, Configure OpenClaw progress, terminal success/failure, bounded redacted logs, and revoke/uninstall controls.

## Boundary

In scope:

- Helper enrollment identity, host/device status, and revoke/uninstall visibility.
- Server-authorized, Helper-revalidated typed jobs using outbound Helper poll/long-poll.
- Non-sudo long-lived Helper/OpenClaw service lifecycle, boot restart, and crash restart.
- Configure OpenClaw closure: install plugin, create/update OpenClaw agent config, configure Borgee plugin connection/channel binding.
- Strict separation between Helper actuator credentials/grants/enforcement and Remote Agent file-proxy credentials/grants/enforcement.

Out of scope:

- Arbitrary host command channel, shell, argv, executable path, script, or client-supplied service unit dispatch.
- Reusing Remote Agent credentials for Helper actuator work.
- Sudo cache or privileged long-lived services.
- New user-facing privacy/compliance product surface.

## Milestones

| Milestone | Goal | Status | Task-split trigger |
|---|---|---|---|
| `milestone-1-helper-enrollment-status` | Establish Helper enrollment identity, connected/last-seen state, allowed job categories, and revoke/uninstall visibility | PLANNED | Break down after this phase plan is accepted |
| `milestone-2-typed-job-policy-loop` | Add server enqueue authority, Helper outbound pull, lease/result/status semantics, local policy checks, and bounded logs | PLANNED | Break down after milestone 1 task skeletons show enrollment/status contracts are executable |
| `milestone-3-configure-openclaw-closure` | Deliver Web-side Configure OpenClaw: plugin install/config, agent config, channel binding, boot/crash service reliability, and terminal UI states | PLANNED | Break down after the typed job/policy loop is planned |

Historical note: this superseded Phase had 3 user-facing milestones, which are now one canonical Helper/OpenClaw milestone.

## Exit Gates

Strict checks:

- Helper enrollment and job execution never reuse Remote Agent file-proxy credentials or rails.
- Server enqueue authorization and Helper local policy both validate owner/org/enrollment/delegation/job type/revocation before action.
- Unknown job types, extra fields, client-supplied argv/executable/script/service IDs, out-of-allowlist paths/domains, stale credentials, and revoked enrollment are rejected.
- Long-lived services remain non-sudo; any privileged installer remains short-lived and visible.

User-perceivable checks:

- Web can show Helper connected/offline/last seen and allowed job categories.
- Configure OpenClaw shows queued/running/succeeded/failed terminal status, failure reason, and bounded redacted logs.
- Revoke/uninstall prevents future jobs and makes queued/leased work settle truthfully.

Carry-over checks:

- If a task cannot preserve Helper/Remote Agent rail separation, it must stop and reopen the anchor rather than patch around the boundary.
- `PS-1` remains a reverse-check guardrail for privacy/security/admin/capability surfaces; it cannot be used to remove existing controls.

## First Milestone Seed

First milestone: `milestone-1-helper-enrollment-status`.

Likely first task seed: `task-1-helper-enrollment-model-and-status`.

Expected PR atom:

- Add the Helper enrollment record and host/device status foundation.
- Add server-side authority checks for owner/org/enrollment visibility.
- Add current-doc sync for the accepted enrollment/status contracts.
- Do not add typed job execution, service lifecycle operations, or Configure OpenClaw closure in this first task.

First acceptance check:

- A reviewer can prove that an enrolled Helper has a distinct Helper identity, visible connected/last-seen state, allowed job categories, and revoke/uninstall state without using Remote Agent credentials.
