# Spec

## Task Contract

Move the Helper Status and Remote Nodes entry points out of the sidebar footer overflow and into a coherent runtime-management location inside user Settings. The move is navigation-only: it must not merge Remote Agent and Helper credentials, grants, status, or enforcement rails.

## Dependency Decision

Task7 is unblocked. The canonical task depends on `task-5-sidebar-footer-primary-entries` only. PR #947 is merged and its merge commit `47dc6805abaf98fffcd727ec5917b641367f2eeb` is an ancestor of this branch. PR #950 is also present on `origin/main`, but Task7 does not depend on avatar/logout behavior beyond preserving the existing footer/account layout.

## In Scope

- Add a Settings runtime tab as the chosen IA location for the two runtime launch entries.
- Wire the Settings entries to the existing `remote-nodes` and `helper-status` app views.
- Remove Remote Nodes and Helper Status from the sidebar footer overflow.
- Preserve Invitations in the footer overflow and pending invitation badge behavior.
- Add focused tests and current-doc updates for the new placement.

## Out Of Scope

- No Remote Agent or Helper API changes.
- No credential reuse, host-grant reuse, rail merge, or enforcement-policy change.
- No changes to `NodeManager` or `HelperStatusPanel` internal behavior.
- No broad sidebar redesign, account settings expansion, or route/deep-link work.
