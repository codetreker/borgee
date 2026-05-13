# Public Channel Preview Sketch

## Purpose

This sketch is an Interaction And Layout Reference for a public channel preview. It does not define product behavior, implementation contracts, join policy, or verification state.

## Surface

Public preview belongs to the channel host. It presents a read-only channel-oriented state before the user has joined or gained full channel interaction rights.

## Layout Sketch

```
+──────────────────────────────────────────────────────────────────────────────+
│  # announcements                                      Public Channel         │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──┐  Alice                       09:00                                  │
│  │AV│  Welcome to the shared channel.                                       │
│  └──┘  - Recent updates                                                     │
│        - Agent collaboration                                                 │
│        - Workspace sharing                                                   │
│                                                                              │
│  ┌──┐  ReleaseBot                  09:01                                  │
│  │AV│  Release notes are available.                                         │
│  └──┘                                                                        │
│                                                                              │
│  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
│  ░   You are previewing #announcements                                    ░  │
│  ░   12 members · 156 messages                                            ░  │
│  ░                                                                        ░  │
│  ░                    ┌──────────────────┐                                 ░  │
│  ░                    │    Join Channel   │                                 ░  │
│  ░                    └──────────────────┘                                 ░  │
│  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │
│                                                                              │
+──────────────────────────────────────────────────────────────────────────────+
```

## Architecture Notes

- Preview content is illustrative; server authorization and join behavior remain outside this sketch.
- The preview state is a channel-host presentation mode, not a separate application.
- Full chat interaction remains gated by the user/channel data authority described in feature surfaces.

## Related Docs

- `../feature-surfaces.md`
- `../ui-map.md`
- `message.md`
