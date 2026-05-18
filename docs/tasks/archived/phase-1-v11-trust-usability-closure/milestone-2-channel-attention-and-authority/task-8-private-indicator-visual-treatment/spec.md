# Spec Brief: Private Indicator Visual Treatment

## 0. Constraints

Task contract: make private-channel indicators accurate without dominating the sidebar. Preserve channel authority, ACL behavior, membership visibility, channel management, mention behavior, and footer/sidebar IA.

Dependency base:

- Task 7 state inventory is satisfied by PR #945 (`378835f0a98878963fbab3d0cae2545867894378`).
- Task 1 is merged as PR #949. Task 8 does not depend on Task 2, Task 4, channel management, or fanout work.
- M3 Task5 already owns sidebar/footer primary-entry scope; this task touches channel rows only.

Blueprint anchor:

- `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.3): private channel lock UI must not collide with unread, fault, or presence badges.

## 1. Product Slice

Users should still identify private channels at a glance, but the marker must be quieter than the current colorful lock emoji and must not consume unread, pinned, archived, preview, selected, hover, drag-over, or DM-only presence/fault state surfaces.

## 2. Implementation Scope

- Replace the private channel leading marker in `SortableChannelItem` and `ChannelItemStatic` with a muted leading-slot text marker.
- Add DOM anchors that make the private state testable: row `data-private="true"`, row `data-kind="channel"`, and marker `data-private-indicator="true"`.
- Keep archived rows authoritative over public/private state and keep unread suppressed on archived rows.
- Keep DM presence/fault semantics separate; no channel-level `data-presence` or failure badge behavior is added.
- Update current docs and task evidence.

## 3. Out Of Scope

- No ACL, membership, mention fanout, allowed-action, channel management, API, server, migration, or footer/sidebar IA changes.
- No broad visual redesign, pixel-art restyling, notification rewrite, or channel-level presence/fault model.
- No changes to public preview visibility authority; server ACL remains the source of truth.
