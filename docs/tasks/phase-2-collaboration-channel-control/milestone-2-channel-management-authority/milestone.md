# Milestone 2: Channel Management Authority

> Remapped history. This milestone remains the detailed task home for channel management tasks, but the authoritative coarse grouping is now `docs/tasks/phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority/`.

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

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-channel-management-surface` | PLANNED | Add a user-side place to inspect joined/created channels and available management actions | Canonical Milestone 2 start | no | after dependency clears |
| `task-2-channel-allowed-action-rules` | PLANNED | Define leave/delete/archive/owner-transfer availability without accidental defaults | `task-1-channel-management-surface` | yes, after task 1 | no |
| `task-3-channel-authority-checks` | PLANNED | Enforce server/client authority checks for membership and ownership actions | `task-1-channel-management-surface`, `task-2-channel-allowed-action-rules` | no | no |

Dependency order: task 1 establishes the management surface; task 2 defines visible action rules; task 3 closes authority enforcement and regression proof.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Management surface, action rules, and authority checks preserve server-authoritative channel ownership. |
| PM | LGTM | Owner-created channel behavior and leave/delete/archive affordances are split for user clarity. |
| QA | LGTM | Action availability and forbidden-action checks are explicit enough for acceptance coverage. |
| Dev | LGTM | Surface, rules, and authority enforcement are one-PR sized with clear sequencing. |
| Security | LGTM | Membership and ownership authority remains a hard security path, especially for destructive actions. |
