# Agent Config Sketch

## Purpose

This sketch is an Interaction And Layout Reference for owner-side agent configuration. It does not define product behavior, implementation contracts, verification status, or server schema authority.

## Surface

Agent config is part of the user SPA agent management sidepane. It is owner-scoped: a user edits configuration for agents they own, while durable state and authorization remain server-owned.

## Interaction Model

- Load the current agent configuration when the panel opens.
- Keep edits local until the user saves.
- Treat unsaved form state as feature-local state; app shell state should only coordinate navigation guards.
- Refresh durable configuration through REST after saving when the feature needs authoritative data.

## Layout Sketch

```
+──────────────────────────────────────────────────+
│  Agent Config                            [v3]    │
├──────────────────────────────────────────────────┤
│  Name       [_________________________________]  │
│  Avatar URL [_________________________________]  │
│  Prompt     ┌───────────────────────────────┐   │
│             │                               │   │
│             └───────────────────────────────┘   │
│  Model      [_________________________________]  │
│  memory_ref [_________________________________]  │
│  Enabled    [x]                                  │
│                                                  │
│                                       [ Save ]   │
+──────────────────────────────────────────────────+
```

## Architecture Notes

- The user REST rail owns the saved config state.
- The user SPA owns local draft state, dirty state, and save/error presentation.
- Admin SPA pages do not own this owner-side edit surface.
- Realtime frames are not treated as authoritative config data in this browser surface.

## Related Docs

- [../feature-surfaces.md](../feature-surfaces.md)
- [../ui-map.md](../ui-map.md)
- [agent-manager.md](agent-manager.md)
