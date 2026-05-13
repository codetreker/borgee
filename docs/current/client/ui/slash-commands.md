# Slash Commands Sketch

## Purpose

This sketch is an Interaction And Layout Reference for command discovery and command-result presentation inside the chat surface. It does not define command contracts, product behavior, implementation details, or verification state.

## Surface

Slash commands belong to the chat composer and message stream. The composer may expose available command options, while command execution and durable message state remain outside this sketch.

## Interaction Model

- The command palette is a composer-adjacent overlay inside the active channel or DM surface.
- Commands can be grouped by source so users can distinguish system commands from agent-provided commands.
- Command output appears in the message stream as a structured message shape, not as a separate application surface.

## Layout Sketches

### Command Palette

```
+────────────────────+─────────────────────────────────────────────────────────+
│ (sidebar)          │  # general                                              │
│                    ├─────────────────────────────────────────────────────────┤
│                    │                                                         │
│                    │  ┌──┐  Alice           10:30                        │
│                    │  │AV│  Can someone check the release state?             │
│                    │  └──┘                                                   │
│                    │                                                         │
│                    ├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┤
│                    │  ┌───────────────────────────────────────────────────┐  │
│                    │  │  Slash Commands                             [X]  │  │
│                    │  ├───────────────────────────────────────────────────┤  │
│                    │  │  System                                           │  │
│                    │  │  /status      Show channel info                   │  │
│                    │  │                                                   │  │
│                    │  │  Agent tools                                      │  │
│                    │  │  /review      Start review workflow               │  │
│                    │  │  /deploy      Run a deployment helper             │  │
│                    │  └───────────────────────────────────────────────────┘  │
│                    ├─────────────────────────────────────────────────────────┤
│                    │  ┌─────────────────────────────────────────┐  [Send]    │
│                    │  │  /                                      │            │
│                    │  └─────────────────────────────────────────┘            │
+────────────────────+─────────────────────────────────────────────────────────+
```

### Result Message

```
┌──┐  Alice                         10:35
│AV│  /status
└──┘
      ┌─────────────────────────────────────────────────────┐
      │  Channel Status                                     │
      │                                                     │
      │  Channel    # general                               │
      │  Members    5 total                                 │
      │  Agents     2 available                             │
      └─────────────────────────────────────────────────────┘
```

## Architecture Notes

- This sketch only places command discovery and command-result shapes in the chat surface.
- Command availability, argument schemas, execution semantics, and permissions belong to the server/user API contracts.
- Result rendering should still follow the message stream's REST and realtime authority model.

## Related Docs

- [../feature-surfaces.md](../feature-surfaces.md)
- [../realtime-sync.md](../realtime-sync.md)
- [message.md](message.md)
