# Milestone 2: Sidebar Account Entry

## Capability Goal

Make the sidebar footer calmer and put account/logout behavior behind the avatar/account panel.

## Acceptance Boundary

Accepted by this milestone:

- Footer exposes a small primary set of entries such as avatar/account, Agents, Workspace, and Settings.
- Avatar opens the account entry and logout moves into that panel.
- Helper/Remote Nodes entry movement does not merge Helper and Remote Agent credentials, grants, or enforcement rails.

Rejected by this milestone:

- Full visual redesign or pixel-art restyling.
- Account settings expansion beyond the task-level scope.

## Task-Split Trigger

Break down after phase-plan acceptance or after production-surface tasks if shared shell files would create avoidable conflict.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-sidebar-footer-primary-entries` | PLANNED | Reduce sidebar footer clutter to a small primary entry set | Phase 3 execution slot | no | after dependency clears |
| `task-2-avatar-account-panel-logout` | PLANNED | Move account/logout behavior behind avatar/account panel | `task-1-sidebar-footer-primary-entries` | yes, after task 1 | no |
| `task-3-helper-remote-nodes-entry-placement` | PLANNED | Move Helper/Remote Nodes entry points without merging rails | `task-1-sidebar-footer-primary-entries` | yes, after task 1 | no |

Dependency order: task 1 establishes the footer budget. Tasks 2 and 3 can run in parallel after task 1 if account-panel and runtime-entry files are separable.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Footer, account panel, and runtime-entry movement keep Helper and Remote Agent rails separate. |
| PM | LGTM | Sidebar/account clarity is split into primary entries, avatar/account panel, and runtime placement. |
| QA | LGTM | Navigation, logout, and runtime-entry reachability checks are scoped for task acceptance. |
| Dev | LGTM | Footer entry, account panel, and runtime-entry placement tasks are one-PR sized after task 1. |
| Security | LGTM | Logout/session behavior and Helper/Remote Agent rail separation are marked for execution review. |
