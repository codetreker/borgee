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
