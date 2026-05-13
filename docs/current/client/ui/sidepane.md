# Sidepane Navigation Sketch

## Purpose

This sketch is an Interaction And Layout Reference for switching between global sidepanes in the user SPA. It does not define product behavior, implementation contracts, state-machine authority, or verification status.

## Surface

Sidepane navigation belongs to the app shell and UI map. It chooses which global sidepane is visible while keeping channel mode as the default workspace view.

## Interaction Model

- The shell exposes one active main view at a time.
- Sidepane buttons open settings, agents, invitations, all workspaces, or remote nodes.
- A back or close action returns the shell to the selected channel view.
- Feature surfaces that own local draft state can participate in unsaved-change guards before navigation changes.

## Layout Sketch

```
+────────────────────+────────────────────────────────────+
│ Navigation rail    │  Active sidepane                   │
│                    │                                    │
│ # general          │  [Back]  Settings / Agents / ...   │
│ # dev              │                                    │
│                    │  Surface-specific content           │
│ Direct messages    │                                    │
│                    │                                    │
│ [Settings]         │                                    │
│ [Agents]           │                                    │
│ [Invitations]      │                                    │
│ [Workspaces]       │                                    │
│ [Remote nodes]     │                                    │
+────────────────────+────────────────────────────────────+
```

## Architecture Notes

- Sidepanes are not independent browser routes in the user SPA.
- Sidepane selection is shell orchestration; durable feature data still belongs to each feature's REST rail.
- This sketch maps to `../ui-map.md` and the app shell state boundary.

## Related Docs

- `../ui-map.md`
- `../app-shell-state.md`
- `../feature-surfaces.md`
