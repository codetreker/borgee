# Private Indicator State Matrix

## Source Anchors

| Surface | Current anchor | Current behavior | Collision note |
|---|---|---|---|
| Channel list container | `ChannelList.tsx` renders `<div className="channel-list" data-kind="channel">` | Channel rows and grouped channel rows live under the channel rail. | Keep channel private-state treatment separate from DM rows. |
| Ungrouped channel row | `SortableChannelItem` from `ChannelList.tsx` | Member channels render sortable channel buttons. | Row can combine private, selected, hover, drag-over, pinned, archived, and unread states. |
| Grouped channel row | `SortableChannelItem` from `ChannelGroupComponent.tsx` | Grouped channels reuse the same row renderer. | Group membership does not change private/unread collision rules. |
| Public preview row | `ChannelItemStatic` from `SortableChannelItem.tsx` | Non-member public preview rows render without drag affordance. | Private non-member rows should not be assumed visible; if ever visible, they must not leak protected names beyond server ACL. |
| DM row | `Sidebar.tsx` `MergedDmList` uses `<div className="dm-list" data-kind="dm">` | DM rows reuse `.channel-item` styling but not `SortableChannelItem`. | Presence/fault dots are DM-only today and must not be conflated with channel-private state. |

## State Matrix

| State | Current DOM/CSS anchor | Occupied visual area | Current priority | Task 8 boundary |
|---|---|---|---|---|
| Public channel | `.channel-hash` text `#` | Leading icon slot before `.channel-name` | Baseline | A new private treatment must keep public rows visually distinct. |
| Private channel | `.channel-hash` text `🔒` when `channel.visibility === "private"` | Leading icon slot before `.channel-name`; same slot as `#` and archived marker | Medium: communicates visibility but can be visually heavy | May be changed by Task 8, but must remain identifiable and must not move into unread/fault/presence slots. |
| Archived channel | `.channel-item-archived`, `data-archived="true"`, `.channel-hash` text `📦`, `.archived-badge` | Leading icon plus trailing badge; row opacity and line-through | High: archived marker currently overrides private/public marker | Task 8 must not make private state obscure archived state. Archived rows do not render unread. |
| Unread channel | `.unread-badge` when `unread_count > 0 && is_member && !archived` | Trailing accessory after name and any pinned/preview/archive badge | High: count is the user attention signal | Private treatment must not consume trailing unread space or reduce count legibility. |
| Selected channel | `.channel-item-active` | Whole-row background, color, font weight | High: navigation truth | Private treatment must remain legible against active background and must not replace selection styling. |
| Hovered channel | `.channel-item:hover` | Whole-row background and text color on hover-capable devices | Medium: discoverability | Private treatment must survive color inheritance/opacity changes on hover. |
| Drag-over insertion | `.drop-indicator` absolute span when sortable `isOver && !isDragging` | Absolute top line across row | High during drag | Private treatment must not be implemented as an absolute overlay that hides insertion feedback. |
| Dragging channel | `.channel-item-dragging` | Whole-row opacity and shadow | Medium during drag | Private marker may fade with row, but must not introduce independent drag opacity. |
| Pinned channel | `data-pinned="true"`, `.channel-pinned-indicator` text `📌` | Trailing accessory after `.channel-name` before unread | Medium: personal ordering signal | Private treatment must not compete with the pinned trailing icon. |
| Preview public channel | `.channel-item-preview`, `.preview-badge` | Row opacity plus trailing preview badge | Low-to-medium: discovery state | Private treatment must not imply a private non-member preview is safe unless server/API supplies it. |
| Group collapsed/expanded | `.group-header`, `data-collapsed`, `.group-header-arrow` | Group header, not channel row | Structural | Private treatment is row-local; no group-header edits in Task 8 unless explicitly scoped. |
| DM user online | `.online-dot.avatar-status` in DM row avatar | Avatar corner inside DM row | DM-only | Do not treat user online status as a channel presence state. |
| Agent DM presence/fault | `PresenceDot` with `[data-presence]`, `[data-failure-badge]`, `.presence-dot` in DM row | Inline accessory after DM peer name | DM-only high priority for agent state | Channel private treatment must not reuse `data-presence` or failure badge semantics. |
| Channel-level fault/presence | No current channel-row anchor found | N/A | Not current behavior | Adding channel fault/presence markers is not Task 8 by default; it needs explicit scope if reopened. |

## Collision Boundaries For Task 8

- Leading slot: currently owns public/private/archive glyphs through `.channel-hash`. If Task 8 moves private state away from this slot, it must still preserve the archive override and public/private distinction.
- Trailing accessory slot: currently owns pinned, archived, preview, and unread badges. Do not move the private indicator here unless Task 8 also proves unread and badge coexistence.
- Whole-row state: selected, hover, preview opacity, archived opacity, and dragging opacity are row-level. Private treatment must work under these row-level styles.
- Absolute overlay: `.drop-indicator` owns the top overlay during drag. Do not add private overlays that could hide drag placement.
- DM-only status: presence and fault dots are currently only in DM rows. Private channel visual truth cannot depend on DM presence selectors.

## Regression Seeds For Later Tasks

Task 8 or Task 9 should cover these combinations at minimum:

| Case | Expected proof |
|---|---|
| Private + unread + selected | Private state identifiable; unread count remains legible; active state remains obvious. |
| Private + unread + hover | Hover does not wash out private marker or unread badge. |
| Private + pinned + unread | Pinned marker and unread count both remain visible; private marker does not move into trailing accessory space. |
| Private + drag-over | Insertion line remains visible. |
| Private + archived | Archived state wins; no false implication that archived private channel is active/unread. |
| DM agent fault next to channel rows | DM fault dot stays DM-only; channel private treatment does not reuse presence/fault semantics. |
