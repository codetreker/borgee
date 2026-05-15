# Acceptance: Sidebar Footer Primary Entries

## Source Alignment

- Task: `task-5-sidebar-footer-primary-entries`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`
- Blueprint anchors: `migration-analysis.md` section 7.3 (`IA-1`) and section 6.1 (`PS-1`)

## Segment A: Primary Footer Set

Acceptance checks:

- A member session sees only avatar/account identity, Agents, Workspaces, and Settings as primary footer entries.
- The primary footer does not expose Logout, Invitations, Remote Nodes, or Helper Status as equal top-level primary buttons.
- Workspace access remains directly visible from the footer.

Negative checks:

- The footer does not hide account/avatar identity.
- The footer does not turn secondary actions into text-heavy controls or a broad sidebar redesign.

## Segment B: Secondary Reachability

Acceptance checks:

- Invitations, Remote Nodes, Helper Status, and Logout remain reachable through a secondary footer menu when their callbacks are supplied and role gates permit them.
- Pending invitation count remains visible on the primary More toggle and secondary Invitations action when pending invitations exist.
- Logout still calls the existing logout API and `onLogout` callback path.

Negative checks:

- Secondary actions are not silently dropped before Task 6 or Task 7 provides their final placement.
- Secondary menu access does not navigate without the existing callbacks.

## Segment C: Agent-Session Boundary

Acceptance checks:

- Agent sessions keep only the entries they were previously allowed to use: avatar/account identity, Workspaces when the callback exists, and Logout as a secondary account/session action.
- Agent sessions do not see owner-only Agents, Invitations, Remote Nodes, Helper Status, or Settings overflow actions introduced by this task.

Negative checks:

- Moving actions into overflow does not bypass existing role gates or remove logout access.

## Segment D: Tests And Verification

Acceptance checks:

- TDD RED tests fail before production changes and cover primary footer count/content, secondary reachability, primary/secondary pending invitation badge, logout path, and agent role gating.
- Focused Sidebar tests pass after implementation.
- Client typecheck and diff hygiene pass before PR open.

Negative checks:

- No ArtifactComments, Settings PermissionsView, NodeManager, or HelperStatusPanel production files are edited by this task.

## Segment E: Current-Doc Sync

Acceptance checks:

- `docs/current/client/ui-map.md` and the relevant UI sketch/current docs describe the small primary footer set and secondary overflow model.
- Current docs preserve Helper/Remote Agent rail separation and do not claim Task 6 account-panel or Task 7 runtime-placement completion.

Negative checks:

- Current docs must not describe secondary overflow as an account panel or final Helper/Remote Nodes IA placement.
