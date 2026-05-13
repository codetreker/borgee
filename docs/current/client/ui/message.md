# Message Surface Sketch

## Purpose

This sketch is an Interaction And Layout Reference for chat and system-message shapes. It does not define product behavior, implementation contracts, copy authority, or verification status.

## Surface

Messages belong to the channel or DM host. The app context coordinates shared message state and pending sends; durable message history, reactions, and permission-sensitive content remain owned by the user REST and realtime rails.

## Interaction Model

- Normal messages show author, timestamp, body, and hover actions.
- Agent messages use the same stream with agent identity treatment.
- Mentions, reactions, command results, and system action prompts are layered around the same message surface.
- Optimistic send and reaction state should reconcile with server updates through the realtime sync model.

## Layout Sketches

### Normal Message

```
┌──┐  Username                      14:30
│AV│  This is a normal text message that can span
└──┘  multiple lines if the content is long enough.
                                      [React] [Edit] [Delete]
      👍 3   🎉 1
```

### Code Block Message

```
┌──┐  AgentX                       14:31
│AV│  Here is the status note:
└──┘
      ┌─────────────────────────────────────────────────────────┐
      │ ```text                                                 │
      │ status: ready                                           │
      │ owner: release-agent                                    │
      │ artifact: release-notes.md                              │
      │ ```                                                     │
      └─────────────────────────────────────────────────────────┘
```

### Mention

```
┌──┐  Bob                           14:33
│AV│  @Alice the artifact update from @AgentX
└──┘  is ready for review.
       ↑                                ↑
       user mention                      agent mention
```

### Capability Prompt Example

```
┌──┐  System                         14:30
│SY│  AgentX wants to use a capability.
└──┘  ┌───────┐ ┌──────┐ ┌───────┐
      │ Grant │ │ Deny │ │ Later │
      └───────┘ └──────┘ └───────┘
```

### Recovery Prompt Example

```
┌──┐  System                         14:30
│SY│  AgentX needs attention.
└──┘  ┌───────────┐
      │ Reconnect │
      └───────────┘
```

### Reaction Row

```
┌──┐  Username                      14:30
│AV│  Message text...
└──┘                                        [Add reaction]
      👍 3   🎉 1
```

## Architecture Notes

- System-message examples illustrate interaction shape only; backend action semantics live outside this sketch.
- Reactions and edits are part of the chat surface but reconcile through server authority.
- DM messages use the same message surface without channel tabs.

## Related Docs

- [../feature-surfaces.md](../feature-surfaces.md)
- [../realtime-sync.md](../realtime-sync.md)
- [dm.md](dm.md)
- [slash-commands.md](slash-commands.md)
