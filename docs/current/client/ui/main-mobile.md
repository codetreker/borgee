# Mobile Shell Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the mobile user SPA shell. It does not define product behavior, implementation contracts, breakpoint policy, or verification state.

## Surface

The mobile shell keeps the selected channel as the main workspace and exposes the navigation rail as an overlay-style surface. The same app shell and feature-surface boundaries apply as on desktop.

## Layout Sketch

### Channel View

```
+──────────────────────────────────+
│ [menu]  # general        [gear]  │
├──────────────────────────────────┤
│  [Chat]  [Workspace]  [Remote]   │
├──────────────────────────────────┤
│                                  │
│ Alice            10:30        │
│ The new build is ready.          │
│                                  │
│ AgentX           10:31        │
│ Build summary is available.      │
│                                  │
│ Bob              10:33        │
│ I will review it now.            │
│                                  │
├──────────────────────────────────┤
│ ┌──────────────────────┐ [Send]  │
│ │ Type a message...    │         │
│ └──────────────────────┘         │
+──────────────────────────────────+
```

### Navigation Overlay

```
+─────────────────────+────────────+
│ COLLAB              │░░░░░░░░░░░░│
│                     │░ dimmed  ░░│
│ ▾ CHANNELS          │░░░░░░░░░░░░│
│   # general         │░░░░░░░░░░░░│
│   # dev             │░░░░░░░░░░░░│
│   # design          │░░░░░░░░░░░░│
│                     │░░░░░░░░░░░░│
│ ▾ DIRECT MESSAGES   │░░░░░░░░░░░░│
│   Bob               │░░░░░░░░░░░░│
│   Carol             │░░░░░░░░░░░░│
│                     │░░░░░░░░░░░░│
│ [Settings][Agents]  │░░░░░░░░░░░░│
+─────────────────────+────────────+
```

## Architecture Notes

- Mobile layout changes presentation, not the ownership of shell state or feature data.
- Channel tabs remain selected-channel surfaces.
- Global sidepanes remain shell-selected views and should not become nested routes in this sketch.

## Related Docs

- `../app-shell-state.md`
- `../ui-map.md`
- `main-desktop.md`
