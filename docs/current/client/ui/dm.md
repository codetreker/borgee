# Direct Message Sketch

## Purpose

This sketch is an Interaction And Layout Reference for direct messages in the user SPA. It does not define product behavior, implementation contracts, or verification state.

## Surface

DMs use the shared selected-channel model for conversation state, but they do not expose the non-DM channel tab strip. The DM rail refreshes lazily after the shell initializes.

## Layout Sketch

### Start DM

```
         ┌─────────────────────────────────────┐
         │  New Direct Message            [X]  │
         ├─────────────────────────────────────┤
         │                                     │
         │  To:                                │
         │  ┌─────────────────────────────┐    │
         │  │ Search users or agents...   │    │
         │  └─────────────────────────────┘    │
         │                                     │
         │  Alice                              │
         │  Bob                                │
         │  AgentX                             │
         │  Builder                            │
         │  Carol                              │
         │                                     │
         └─────────────────────────────────────┘
```

### DM Thread

```
+──────────────────────────────────────────────────────────────────────────────+
│  Alice                                                        [pin] [gear]   │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──┐  Alice                       15:00                                  │
│  │AV│  Can you review the workspace note for the new feature?               │
│  └──┘                                                                        │
│                                                                              │
│  ┌──┐  You                         15:02                                  │
│  │AV│  I will take a look this afternoon.                                   │
│  └──┘                                                                        │
│                                                                              │
│  ┌──┐  Alice                       15:03                                  │
│  │AV│  The note is in workspace-notes.md.                                   │
│  └──┘                                                                        │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────────────┐  [Send]    │
│  │  Type a message...                                           │            │
│  └──────────────────────────────────────────────────────────────┘            │
+──────────────────────────────────────────────────────────────────────────────+
```

## Architecture Notes

- DMs share message and pending-message state with the chat surface.
- DM-specific local presentation should not create a separate durable data owner.
- Realtime delivery and REST reconciliation follow the same sync model as channel chat.

## Related Docs

- [../app-shell-state.md](../app-shell-state.md)
- [../feature-surfaces.md](../feature-surfaces.md)
- [../realtime-sync.md](../realtime-sync.md)
- [message.md](message.md)
