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
- The rail should be read together with `../ui-map.md` and `../feature-surfaces.md`.

## Related Docs

- `../ui-map.md`
- `../feature-surfaces.md`
