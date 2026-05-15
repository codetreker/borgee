# Milestone 1: Helper Enrollment Status

## Capability Goal

Create the visible and authoritative foundation for Helper enrollment before any typed host-management job executes.

## Acceptance Boundary

Accepted by this milestone:

- Helper enrollment has its own server-side record, host/device identity, owner/org binding, allowed job categories, and revoke/uninstall state.
- Web can distinguish connected, offline, revoked, and uninstalled Helper states without implying Configure OpenClaw has succeeded.
- Helper credentials and Remote Agent file-proxy credentials remain separate.

Rejected by this milestone:

- Typed job execution, queue/lease/result behavior, service lifecycle operations, or OpenClaw configuration closure.
- Any shared token, shared grant, or merged enforcement rail between Helper and Remote Agent.
- Any new user-facing privacy/compliance product surface.

## Dependencies

| Dependency | Status | Handling |
|---|---|---|
| `HB-RA-1A` guardrails | READY | Preserve product boundary |
| `HB-RA-1B` execution contract | READY FOR PLANNING | Carry rail/credential/status requirements into tasks |
| `PS-1` privacy guard | READY | Reverse-check existing controls remain intact |

## Task-Split Trigger

Run milestone breakdown after the phase-plan PR is accepted. This milestone should break into multiple real tasks; if breakdown produces fewer than 3 tasks, re-check whether this milestone is too narrow.

## First Task Seed

Likely first task: `task-1-helper-enrollment-model-and-status`.

The task should establish Helper enrollment identity and visible status only; it must not execute host-management jobs.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-helper-enrollment-model-and-status` | ACCEPTED | Create distinct Helper enrollment identity and visible host status foundation | none | no | yes |
| `task-2-helper-credential-rotation-and-revoke` | TASKING | Add helper credential lifecycle, stale-device handling, and revoke/uninstall authority | `task-1-helper-enrollment-model-and-status` | yes, after task 1 | no |
| `task-3-helper-status-ui-and-current-sync` | READY | Surface Helper status and sync accepted enrollment/status contracts to current docs | `task-1-helper-enrollment-model-and-status` | yes, after task 1 | no |

Dependency order: task 1 must land first because later credential and UI work need the enrollment identity and owner/org binding. Tasks 2 and 3 can run in parallel after task 1 if their touched files do not conflict.

Task 1 acceptance state: PR #934 merged at `547f869`. PR #935 was closed before landing the shared acceptance-state cleanup, so task 2 carries that remediation while remaining unaccepted until its own acceptance evidence is complete.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Enrollment identity, credential lifecycle, and status UI boundaries are separated with task 1 as the dependency base. |
| PM | LGTM | User-visible value flows from enroll/status to credential/revoke and status UI without merging Helper and Remote Agent rails. |
| QA | LGTM | Acceptance slices are checkable for identity, credential rotation/revoke, and status/current-doc sync. |
| Dev | LGTM | Each task is sized for one PR; tasks 2 and 3 can split after enrollment identity lands. |
| Security | LGTM | Sensitive credential, owner/org authority, revoke, and rail-separation paths are identified for task execution. |
