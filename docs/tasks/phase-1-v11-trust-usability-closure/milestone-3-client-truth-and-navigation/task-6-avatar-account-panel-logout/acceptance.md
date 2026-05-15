# Acceptance: Avatar Account Panel Logout

## Source Alignment

- Task: `task-6-avatar-account-panel-logout`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`
- Blueprint anchors: `migration-analysis.md` section 7.3 (`IA-1`) and section 6.1 (`PS-1`)

## Segment A: Avatar Account Entry

Acceptance checks:

- The sidebar footer avatar is an interactive account trigger.
- Opening the avatar account trigger renders an account panel near the footer.
- The avatar remains visible as the account identity signal in the primary footer row.

Negative checks:

- The account trigger does not replace Agents, Workspaces, or Settings primary entries.
- The footer does not gain a broad visual redesign or text-heavy primary account control.

## Segment B: Account Panel Summary

Acceptance checks:

- The account panel shows the current user's display name and role/account context from existing client state.
- The panel is locally controlled and can close by choosing an action or clicking outside the panel.

Negative checks:

- No account settings, profile editing, privacy/compliance promise, audit, or admin-impact UI is added.

## Segment C: Logout Move

Acceptance checks:

- Logout is reachable from the account panel.
- Logout calls the existing `logout()` API and `onLogout` callback path.
- Logout is no longer rendered in the footer More menu.

Negative checks:

- Logout failure keeps the existing reload fallback.
- Removing Logout from More does not remove logout access for member or agent sessions.

## Segment D: Role And Scope Boundaries

Acceptance checks:

- Agent sessions can open the account panel and log out.
- Agent sessions do not gain owner-only Agents, Invitations, Remote Nodes, Helper Status, or Settings controls through the account panel.
- Invitations, Remote Nodes, and Helper Status remain in More when callbacks and role gates permit them.

Negative checks:

- No ArtifactComments, Settings PermissionsView, channel authority, NodeManager, or HelperStatusPanel production files are edited by this task.

## Segment E: Tests And Current Docs

Acceptance checks:

- TDD RED tests fail before production changes and cover account panel open, logout behavior, More-menu logout removal, and agent logout access.
- Focused Sidebar tests, client typecheck, client build, full client tests, and diff hygiene pass before PR merge.
- Current docs describe avatar account panel/logout as implemented without claiming account settings expansion or Task7 runtime-entry placement.
