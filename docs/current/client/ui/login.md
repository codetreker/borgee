# Login Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the user SPA auth gate. It does not define product behavior, implementation contracts, or verification state.

## Surface

Login belongs to the unauthenticated user SPA boundary. The authenticated shell does not render until the session and bootstrap flow described in `../app-shell-state.md` completes.

## Layout Sketch

```
+──────────────────────────────────────────────────────────────────────────────+
│                                                                              │
│                              B O R G E E                                     │
│                      Human-Agent Collaboration                               │
│                                                                              │
│                   ┌──────────────────────────────┐                           │
│                   │  Email                        │                           │
│                   └──────────────────────────────┘                           │
│                   ┌──────────────────────────────┐                           │
│                   │  Password            [eye]    │                           │
│                   └──────────────────────────────┘                           │
│                                                                              │
│                   ┌──────────────────────────────┐                           │
│                   │          LOG IN               │                           │
│                   └──────────────────────────────┘                           │
│                                                                              │
│                   Have an invite code? Register ->                           │
│                                                                              │
+──────────────────────────────────────────────────────────────────────────────+
```

## Architecture Notes

- The sketch shows the auth gate shape only; auth endpoints and session authority are outside this UI reference.
- Login and registration are unauthenticated surfaces, separate from the initialized workspace shell.

## Related Docs

- `../app-shell-state.md`
- `../ui-map.md`
