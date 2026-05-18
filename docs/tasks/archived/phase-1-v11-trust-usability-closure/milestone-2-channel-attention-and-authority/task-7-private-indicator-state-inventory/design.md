# Design: Private Indicator State Inventory

## Decision

Execute this as a docs-only scouting task. The deliverable is a task-local matrix plus a current-doc note, not a production UI implementation.

## Inputs Read

- Task contract: `task.md`
- Blueprint anchors: `docs/blueprint/next/migration-analysis.md` sections 4.3 and 6.1
- Current channel row implementation anchors:
  - `packages/client/src/components/ChannelList.tsx`
  - `packages/client/src/components/ChannelGroupComponent.tsx`
  - `packages/client/src/components/SortableChannelItem.tsx`
  - `packages/client/src/components/Sidebar.tsx`
  - `packages/client/src/components/PresenceDot.tsx`
  - `packages/client/src/index.css`
- Current docs: `docs/current/client/ui/channel-sort-groups.md`, `docs/current/client/ui/main-desktop.md`, and `docs/current/client/ui-map.md`

## Output Shape

1. `spec.md`, `stance.md`, and `acceptance.md` establish the task-start baseline.
2. `state-matrix.md` records current DOM/CSS anchors and collision boundaries.
3. `docs/current/client/ui/channel-sort-groups.md` gains a current-behavior section for row state anchors and known collision risk.
4. `progress.md` records verification evidence and the production-edit negative check.

## Current-Doc Sync Choice

`docs/current/client/ui/channel-sort-groups.md` is the right current-doc target because it already describes the channel rail and grouping/sorting interaction reference. The update must stay current-state oriented: it can say the private channel marker is currently the leading `.channel-hash` glyph, but it must not specify Task 8's future treatment.

## Verification Design

Verification is docs/scout verification:

- `git diff --check`
- targeted grep that confirms no production files under `packages/client/src/components/` or `packages/client/src/index.css` changed
- targeted grep that confirms the matrix names the required private/unread/fault/presence/selection/hover states
- package-level docs are markdown-only, so no client unit test is required for this PR

## Non-Code Note

`READY_FOR_IMPL` is N/A because this task is the implementation: a scouting inventory and current-doc note. No production implementation design or code review is required before PR open.
