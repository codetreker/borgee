# Spec Brief: Avatar Account Panel Logout

## 0. Constraints

Task contract: turn the sidebar avatar into the account entry and move logout into that account panel. This task builds only the v1 account summary plus logout behavior.

Blueprint anchors:

- `IA-1` (`docs/blueprint/next/migration-analysis.md` section 7.3): account panel v1 is account summary plus logout unless a task explicitly adds account settings.
- `PS-1` (`docs/blueprint/next/migration-analysis.md` section 6.1): account IA movement must preserve existing admin/privacy/security controls, data minimization, capability boundaries, and rail separation.

Implementation ownership:

- Owned production files include `packages/client/src/components/Sidebar.tsx`, related sidebar CSS, and focused Sidebar/app-shell tests.
- Do not edit M3 Task1 ArtifactComments files, M3 Task3 Settings Permissions files, channel authority files, or M3 Task7 Helper/Remote Nodes final placement internals.

## 1. Segmentation

Segment A: Avatar account entry.
The current-user avatar becomes an interactive account trigger. It remains the primary footer account identity signal and opens a compact account panel.

Segment B: Account panel contents.
The panel shows account summary information from the existing current user state and exposes Logout. It does not add account settings, profile editing, privacy/compliance promises, audit surfaces, or user-facing admin impact records.

Segment C: Logout move.
Logout moves out of the footer More menu into the account panel. The logout action keeps the existing `logout()` API call, `onLogout` callback, and reload fallback on API failure.

Segment D: Role and secondary-action boundaries.
Agent sessions can use the account panel and logout because those are account/session behavior. Owner-only sidebar destinations remain gated as before. Invitations, Remote Nodes, and Helper Status stay in the existing More overflow until Task7 moves runtime entries.

Segment E: Current-doc sync.
After implementation, `docs/current` reflects that avatar is now the account panel entry and logout is no longer a More overflow action.

## 2. Carry-Over

- Task5 already reduced the footer primary entries and kept secondary actions reachable through More. Task6 consumes that footer shape and moves only logout.
- Task7 still owns final Helper/Remote Nodes placement. This task must not move those entries or merge their credentials, grants, status, or enforcement rails.
- Settings remains a separate primary footer entry; this task does not add account settings to the avatar panel.

## 3. Reverse Checks

- If Logout remains reachable only through More instead of the avatar account panel, Task6 fails.
- If the avatar is no longer visible as account identity, Task6 fails.
- If the panel adds account settings or privacy/compliance product copy, Task6 exceeds scope.
- If agent sessions lose logout access, Task6 breaks account/session behavior.
- If owner-only sidepane controls become visible to agent sessions through the account panel, Task6 violates existing authority boundaries.
- If ArtifactComments, Settings Permissions, or channel authority files change, this task has overlapped with unsatisfied or separate milestone work and must stop.

## 4. Out Of Scope

- Account settings expansion, profile editing, organization switching, admin-awareness UI, audit surfaces, or privacy/compliance product promises.
- Helper/Remote Nodes final IA placement, remote-agent behavior, helper credentials, host grants, or status semantics.
- ArtifactComments production mount, ACL forbidden-state UX, Settings PermissionsView reachability, production-surface e2e reverse proof, and channel authority changes.
- Broad sidebar visual redesign, mobile shell rewrite, or design-system replacement.
