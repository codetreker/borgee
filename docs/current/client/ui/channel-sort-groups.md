# Channel Grouping Sketch

## Purpose

This sketch is an Interaction And Layout Reference for channel grouping and ordering in the navigation rail. It does not define product behavior, implementation contracts, verification status, or backlog scope.

## Surface

Channel grouping belongs to the channel rail. The rail reads shared channel metadata from app state and emits shell navigation; durable ordering and group membership are server-owned channel metadata.

## Interaction Model

- Ordering and group membership are channel metadata, not local UI truth.
- Expand/collapse is local presentation state and should not be confused with server-owned grouping.
- Owner and non-owner affordances below are illustrative; server authorization remains authoritative.
- DM entries are outside channel grouping and remain in the direct-message rail.

## Layout Sketches

### Owner View

```
+────────────────────+
│ COLLAB         [+] │
│                    │
│ ≡ # general        │
│ ≡ # random         │
│                    │
│ ▾ Engineering      │
│ ≡ # dev            │
│ ≡ # ci             │
│ ≡ # infra          │
│                    │
│ ▾ Design           │
│ ≡ # design         │
│ ≡ # research       │
│                    │
│ ▸ Archived         │
│                    │
│ ▾ DIRECT MESSAGES  │
│   Bob              │
│   Carol            │
│                    │
│ [Settings][Agents] │
+────────────────────+
```

### Read-Only View

```
+────────────────────+
│ COLLAB             │
│                    │
│   # general        │
│   # random         │
│                    │
│ ▾ Engineering      │
│   # dev            │
│   # ci             │
│   # infra          │
│                    │
│ ▾ Design           │
│   # design         │
│   # research       │
│                    │
│ ▾ DIRECT MESSAGES  │
│   Bob              │
│   Carol            │
│                    │
│ [Settings][Agents] │
+────────────────────+
```

### Reordering State

```
+────────────────────+
│ ▾ Engineering      │
│ ≡ # dev            │
│ ────────────────   │  <- insertion position
│ ≡ # ci             │
│                    │
│ ┏━━━━━━━━━━━━━━━━┓ │
│ ┃  # research    ┃ │  <- moving item
│ ┗━━━━━━━━━━━━━━━━┛ │
│   . . . . . . .    │
+────────────────────+
```

## Architecture Notes

- The sketches show possible rail affordances, not exact timing, styling, storage, or broadcast behavior.
- Server-owned channel metadata is the authority for ordering and group membership.
- Local expand/collapse presentation can differ by user without changing channel metadata.
- The rail should be read together with [../ui-map.md](../ui-map.md) and [../feature-surfaces.md](../feature-surfaces.md).

## Current Channel Row State Anchors

Current channel rows use `packages/client/src/components/SortableChannelItem.tsx` for member rows and `ChannelItemStatic` for public preview rows. The row container is `.channel-item`; selected state is `.channel-item-active`; hover state is `.channel-item:hover`; drag-over insertion is `.drop-indicator`; unread count is `.unread-badge`; archived state uses `data-archived="true"`, `.channel-item-archived`, the leading archive glyph, and `.archived-badge`.

Private channel visibility is currently a leading `.channel-hash` glyph: public rows render `#`, private rows render `🔒`, and archived rows render `📦` instead of the public/private glyph. Pinned rows can add `data-pinned="true"` and `.channel-pinned-indicator` in the trailing accessory area. Public preview rows can add `.channel-item-preview` and `.preview-badge`; private non-member rows should not be assumed visible from this sketch because server ACL remains authoritative.

Direct-message rows reuse `.channel-item` styling under `data-kind="dm"`, but they do not use `SortableChannelItem`. User online dots and agent `PresenceDot` fault/presence markers live in the DM rail only. Channel-level private indicators should not reuse DM presence or fault semantics unless a later accepted task explicitly adds a channel-level state model.

Known current gap: the leading lock glyph is the only current private-channel signal, so later private-indicator visual work must prove it does not hide or compete with unread, selected, hover, drag-over, archived, preview, pinned, or DM-only presence/fault signals. This document records current anchors only; it does not promise the later visual treatment.

## Related Docs

- [../ui-map.md](../ui-map.md)
- [../feature-surfaces.md](../feature-surfaces.md)
