# Agent Collaboration Sketch

## Purpose

This sketch is an Interaction And Layout Reference for showing agents as visible collaborators inside a channel. It does not define product behavior, implementation contracts, verification status, or design-system rules.

## Surface

Agent collaboration belongs to the channel host and agent surfaces. It shares the same collaboration path as human participants: channel membership, messages, artifact activity, and owner-visible agent status remain in the user rail.

## Interaction Model

- Agent rows appear alongside human members where the channel surface exposes participant context.
- Agent collaboration is visible to the owner instead of being hidden behind a separate agent-only scope.
- Conflict or collaboration hints should orient the user without creating a separate admin or private visibility path.

## Layout Sketch

```
+──────────────────────────────────────────────+
│  #channel-name                  [Settings]   │
├──────────────────────────────────────────────┤
│  Members:                                    │
│   Alice (owner)                              │
│   AgentA [Bot]   <-- hover: "collaborating" │
│   AgentB [Bot]   <-- hover: "collaborating" │
│   Bob                                        │
└──────────────────────────────────────────────+
```

## Architecture Notes

- This is a user SPA surface, not an admin surface.
- The sketch assumes existing channel and artifact rails; it does not introduce a separate collaboration API.
- Agent collaboration visibility belongs with [../feature-surfaces.md](../feature-surfaces.md) under Agent And Invitation Surfaces and Channel, Chat, And DM.

## Related Docs

- [../feature-surfaces.md](../feature-surfaces.md)
- [../ui-map.md](../ui-map.md)
