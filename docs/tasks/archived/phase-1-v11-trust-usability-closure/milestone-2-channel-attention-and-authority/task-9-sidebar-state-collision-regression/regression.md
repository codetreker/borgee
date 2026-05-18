# Regression: Sidebar State Collision

## Matrix

| Collision case | Proof | Expected boundary |
|---|---|---|
| private/unread/selected/pinned/drag-over | `Sidebar-state-collision-regression.test.tsx` renders one private row with `unread_count=128`, `active`, `pinned`, and mocked `isOver=true` | Private marker, active class, pinned marker, `99+` unread badge, and `.drop-indicator` all remain present and ordered in separate anchors. |
| archived private + unread | `Sidebar-state-collision-regression.test.tsx` renders archived private row with unread | Archived row emits `data-archived="true"`, no `data-private`, no private marker, and no unread badge. |
| public preview | `Sidebar-state-collision-regression.test.tsx` renders a static public non-member preview row | Preview badge remains public-preview state; private metadata and unread badge are absent. |
| DM-only presence/fault semantics | `Sidebar-state-collision-regression.test.tsx` reads `SortableChannelItem.tsx` and `Sidebar.tsx` | Channel rows do not import `PresenceDot` and do not emit `data-presence` or `data-failure-badge`; DM rows remain the source of those semantics. |
| task/current evidence | `Sidebar-state-collision-regression.test.tsx` reads Task9 docs and `docs/current/client/ui/channel-sort-groups.md` | Regression proof remains discoverable for future sidebar changes. |

## Reviewer Notes

The suite is intentionally jsdom/source-level instead of broad e2e. The task needs collision proof around one component boundary, and `useSortable` is mocked only to force drag-over rendering deterministically.

Task9 makes no production UI or authority changes. If a future change moves private state out of the leading slot, changes unread/pinned/archive anchors, or introduces channel-level presence/fault, this regression should fail or be updated with an explicit accepted scope change.
