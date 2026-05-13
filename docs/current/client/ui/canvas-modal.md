# Canvas Decision Flow Sketch

## Purpose

This sketch is an Interaction And Layout Reference for canvas and artifact decision flows. It does not define product behavior, implementation contracts, accessibility status, or verification status.

## Surface

Canvas decisions belong to the channel artifact surface. The surface can ask the user to confirm destructive actions or enter a value, but artifact bodies, versions, comments, and permissions remain REST-authoritative.

## Interaction Model

- Keep the user inside the Borgee surface when confirming canvas actions.
- Use modal or inline prompts as presentation state local to the artifact surface.
- Treat save, delete, rename, rollback, and comment actions as REST-backed operations.
- Preserve the app shell boundary: modal state should not become shared app state unless another surface needs it.

## Layout Sketch

```
+──────────────────────────────────────────────+
│  Canvas / Artifact                            │
│                                              │
│  Artifact content or version panel            │
│                                              │
│  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░      │
│  ░ ┌────────────────────────────────────┐ ░  │
│  ░ │  Delete artifact?              [X] │ ░  │
│  ░ ├────────────────────────────────────┤ ░  │
│  ░ │  This action affects the current   │ ░  │
│  ░ │  channel artifact surface.         │ ░  │
│  ░ │                                    │ ░  │
│  ░ │              [Cancel] [Delete]     │ ░  │
│  ░ └────────────────────────────────────┘ ░  │
│  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░      │
+──────────────────────────────────────────────+
```

## Architecture Notes

- Realtime signals can wake the artifact surface, but content is pulled through REST.
- Prompt values are local draft state until the user submits them.
- This sketch does not define modal component internals or browser-specific behavior.

## Related Docs

- `../feature-surfaces.md`
- `../realtime-sync.md`
