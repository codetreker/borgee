# Desktop Shell Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the desktop user SPA shell. It does not define product behavior, implementation contracts, or verification state.

## Surface

The desktop shell hosts the navigation rail, selected channel host, channel tabs, message stream, and composer. App shell state owns view selection; feature surfaces own local workflow state.

## Layout Sketch

```
+────────────────────+─────────────────────────────────────────────────────────+
│ COLLAB             │  [Chat]  [Workspace]  [Remote]                         │
│                    ├─────────────────────────────────────────────────────────┤
│ ▾ CHANNELS         │  # general                                    [⚙]  [📌] │
│   # general        ├─────────────────────────────────────────────────────────┤
│   # dev            │                                                         │
│   # design         │  ┌──┐  Alice           10:30                        │
│                    │  │AV│  The new build is ready for review.              │
│ ▾ DIRECT MESSAGES  │  └──┘                                                   │
│   Bob              │                                                         │
│   Carol            │  ┌──┐  AgentX         10:31                         │
│   Dave             │  │AV│  Build summary is available.                     │
│                    │  └──┘  ```                                              │
│                    │        lint passed                                      │
│                    │        unit checks passed                               │
│                    │        ```                                              │
│                    │                                                         │
│                    │  ┌──┐  Bob              10:33                        │
│                    │  │AV│  I will review it now.                            │
│                    │  └──┘                                                   │
│                    │                                                         │
│                    ├─────────────────────────────────────────────────────────┤
│ [AV][Agents][Files]│  ┌─────────────────────────────────────────┐  [Send]    │
│ [Settings][More]   │  │  Type a message...                     │            │
│                    │  └─────────────────────────────────────────┘            │
+────────────────────+─────────────────────────────────────────────────────────+
```

## Architecture Notes

- The rail and channel host are one browser shell, not independent applications.
- Chat, workspace, and remote are selected-channel surfaces; sidepane buttons open global sidepanes.
- The sidebar footer primary row stays small: avatar/account identity, Agents, Workspaces, and Settings. Less frequent shell actions such as Invitations, Remote Nodes, Helper Status, and Logout live behind the footer overflow menu until their later account/runtime placement tasks move them.
- Durable message and workspace data remain REST-authoritative, with realtime used for direct updates and signals.

## Related Docs

- [../app-shell-state.md](../app-shell-state.md)
- [../ui-map.md](../ui-map.md)
- [../feature-surfaces.md](../feature-surfaces.md)
