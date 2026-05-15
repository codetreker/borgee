# Sidepane Navigation Sketch

## Purpose

This sketch is an Interaction And Layout Reference for switching between global sidepanes in the user SPA. It does not define product behavior, implementation contracts, state-machine authority, or verification status.

## Surface

Sidepane navigation belongs to the app shell and UI map. It chooses which global sidepane is visible while keeping channel mode as the default workspace view.

## Interaction Model

- The shell exposes one active main view at a time.
- Primary sidebar footer buttons open the avatar account panel, Agents, all workspaces, and Settings.
- The avatar account panel shows account summary and Logout only; it is not account settings expansion.
- The footer overflow opens Invitations only; Remote Nodes and Helper Status are launched from Settings Runtime.
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
│ [AV][Agents]       │                                    │
│ [Workspaces]       │                                    │
│ [Settings][More]   │                                    │
│   More:            │                                    │
│   Invitations      │                                    │
+────────────────────+────────────────────────────────────+
```

## Architecture Notes

- Sidepanes are not independent browser routes in the user SPA.
- Sidepane selection is shell orchestration; durable feature data still belongs to each feature's REST rail.
- This sketch maps to [../ui-map.md](../ui-map.md) and the app shell state boundary.
- The account panel is local shell chrome, not a sidepane or account settings surface.
- The Settings Runtime tab launches Remote Nodes and Helper Status as separate rail entries. Remote Agent node tokens, Helper enrollment credentials, host grants, and enforcement checks remain separate.

## Related Docs

- [../ui-map.md](../ui-map.md)
- [../app-shell-state.md](../app-shell-state.md)
- [../feature-surfaces.md](../feature-surfaces.md)
