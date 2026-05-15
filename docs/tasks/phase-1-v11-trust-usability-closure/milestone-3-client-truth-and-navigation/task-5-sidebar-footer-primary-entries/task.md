# task-5-sidebar-footer-primary-entries

Purpose:
- Make the sidebar footer calmer by exposing only the primary entries users need repeatedly.

Scope:
- Reduce footer entry exposure toward the v1 primary set, such as avatar/account, Agents, Workspace, and Settings.
- Keep removed or secondary entries reachable through appropriate secondary surfaces when still in scope.

Out of scope:
- No full visual redesign, pixel-art restyling, account settings expansion, or Helper/Remote Agent rail merge.

Depends on:
- Canonical Milestone 3 start

Blueprint anchors:
- `IA-1`: `docs/blueprint/next/migration-analysis.md` §7.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can see the footer expose a small primary entry set without hiding account or workspace access.

Parallelism:
- First task for this milestone when the canonical milestone opens. Blocks avatar account panel and runtime-entry placement.

Sensitive paths:
- account navigation, rail-separation visibility
