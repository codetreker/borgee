# Milestone 2: Channel Management Authority

## Capability Goal

Give users a clear place to understand channel ownership, membership, and allowed actions.

## Acceptance Boundary

Accepted by this milestone:

- User-side channel management exposes joined/created channels and allowed membership/ownership actions.
- Self-created or owned channels do not expose a misleading `leave` action.
- Delete/archive/owner-transfer choices remain explicit task decisions, not accidental side effects.

Rejected by this milestone:

- Owner transfer or hard delete as default v1 commitments without task-level scope.
- Notification/collapse/sort rewrite unless explicitly included by a task.

## Task-Split Trigger

Break down after phase-plan acceptance. Expected tasks should cover management surface routing, allowed-action rules, and server/client authority checks.
