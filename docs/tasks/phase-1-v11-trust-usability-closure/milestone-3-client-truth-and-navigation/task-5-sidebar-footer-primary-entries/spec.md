# Spec Brief: Sidebar Footer Primary Entries

## 0. Constraints

Task contract: reduce the sidebar footer to a calmer primary entry set while keeping account and workspace access visible. This task owns production sidebar/footer edits and does not own ArtifactComments, Settings PermissionsView reachability, avatar account panel/logout behavior, Helper/Remote Nodes final placement, or broad sidebar redesign.

Blueprint anchors:

- `IA-1` (`docs/blueprint/next/migration-analysis.md` section 7.3): sidebar footer cleanup should expose a small primary entry set. Default primary candidates are avatar/account, Agents, Workspace, and Settings. Avatar account panel/logout and Helper/Remote Nodes placement are later tasks.
- `PS-1` (`docs/blueprint/next/migration-analysis.md` section 6.1): sidebar IA movement must preserve existing admin/privacy/security controls, data minimization, capability boundaries, and Helper/Remote Agent rail separation.

Implementation ownership:

- Owned production files include `packages/client/src/components/Sidebar.tsx`, related sidebar CSS, and focused app shell/sidebar tests.
- Do not edit M3 Task1 ArtifactComments files or M3 Task3 Settings Permissions files.
- M2 Task7 remains inventory-only and must not own production sidebar/footer edits.

## 1. Segmentation

Segment A: Primary footer set.
The accepted footer exposes only the repeated primary entries as first-class footer controls: current-user avatar/account identity, Agents, Workspaces, and Settings. The footer must not present Invitations, Remote Nodes, Helper Status, or Logout as equal primary buttons.

Segment B: Secondary reachability.
Secondary destinations that still exist in the shell remain reachable through an overflow/menu control in this task. This preserves access until later tasks move logout behind the avatar account panel and place Helper/Remote Nodes into their final runtime-management surface.

Segment C: Agent-session boundary.
Agent users keep the same authority constraints as before: owner-only entries such as Agents, Invitations, Remote Nodes, Helper Status, and Settings do not become available to agent sessions through the new footer structure. Account logout remains reachable because it is an account/session action, not an owner-only sidepane.

Segment D: Layout stability.
The footer remains compact and scan-friendly on desktop/mobile sidebar widths. Buttons have stable dimensions, no wrapping row of many icons, no broad visual redesign, and no text labels that force width expansion.

Segment E: Current-doc sync.
After implementation, `docs/current` reflects that app shell sidepane navigation now distinguishes a small primary footer set from secondary overflow actions. Current docs must not imply that Helper and Remote Agent rails merge.

## 2. Carry-Over

- Task 6 owns turning the avatar into the account panel and moving logout behind that panel. Task 5 may keep logout reachable as a secondary action, but must not build the account panel.
- Task 7 owns final Helper/Remote Nodes entry placement. Task 5 may keep those surfaces reachable as secondary actions, but must not merge their credentials, grants, or enforcement rails.
- Invitations remain an existing sidepane. This task may demote it from primary footer exposure, not delete the sidepane or its pending-count refresh behavior.
- Exact icon glyphs should follow the existing Sidebar emoji/icon-button pattern unless a later design-system task replaces the sidebar icon system.

## 3. Reverse Checks

- If the footer still renders logout, invitations, remote nodes, and helper status as primary siblings of Agents/Workspaces/Settings, it fails the task.
- If account/avatar or workspace access disappears from the footer, it fails the acceptance slice.
- If secondary entries become unreachable before Task 6 or Task 7 replaces them, the task violates its own scope.
- If agent sessions gain owner-only sidepane access through the overflow menu, it violates existing authority boundaries. If agent sessions lose logout access, the task hides account/session access.
- If the implementation changes ArtifactComments production mount or Settings PermissionsView routing, it overlaps with M3 Task1 or M3 Task3 and must stop.
- If Helper and Remote Nodes are presented as one shared credential/grant/enforcement rail, it violates `PS-1` and `IA-1`.

## 4. Out-Of-Scope

- Account panel, account settings expansion, profile redesign, or moving logout behind avatar as final behavior.
- Final Helper/Remote Nodes IA placement, remote-agent behavior, helper credentials, host grants, or status semantics.
- ArtifactComments production mount, ACL forbidden-state UX, Settings PermissionsView reachability, and production-surface e2e reverse proof.
- Broad sidebar visual redesign, channel list redesign, mobile shell rewrite, notification/collapse/sort rewrite, or pixel-art restyling.
