# Workspace Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the channel workspace surface. It does not define product behavior, implementation contracts, file-format support, or verification state.

## Surface

Workspace belongs to the user SPA file-work surfaces. Channel workspace focuses on the active channel; all-workspaces sidepane provides a cross-channel projection. Server workspace APIs own durable file trees and file bodies.

## Layout Sketch

```
+────────────────────+─────────────────────────────────────────────────────────+
│  Workspace         │  notes/architecture.md                         [Raw]    │
│                    ├─────────────────────────────────────────────────────────┤
│  ▾ notes/          │                                                         │
│    README.md       │  # Architecture                                         │
│    architecture.md │                                                         │
│    schema.yaml     │  ## Overview                                            │
│  ▾ src/            │                                                         │
│    index.ts        │  The system uses REST-backed data and realtime signals. │
│    auth.ts         │                                                         │
│  ▾ assets/         │  ## Components                                          │
│    logo.png        │                                                         │
│                    │  - Server rail: API and realtime hub                    │
│                    │  - Client: React SPA                                    │
│                    │  - Database: SQLite                                     │
│                    │                                                         │
│                    │  ```typescript                                           │
│                    │  interface Message {                                     │
│                    │    id: string;                                           │
│                    │    content: string;                                      │
│                    │    author: User;                                         │
│                    │  }                                                       │
│                    │  ```                                                     │
+────────────────────+─────────────────────────────────────────────────────────+
```

### Context Menu Shape

```
│    README.md       │
│    architecture.md │       ┌──────────────────┐
│    schema.yaml  <- │       │  Rename          │
│  ▾ src/            │       │  Copy path       │
│    index.ts        │       │  Move to...      │
│                    │       │  Delete          │
│                    │       └──────────────────┘
```

## Architecture Notes

- The file tree and file body are REST-owned data; viewer selection and edit drafts are local presentation state.
- Context menu entries are illustrative surface affordances, not a complete command contract.
- Workspace content does not become realtime-authoritative because a WebSocket signal arrives.

## Related Docs

- `../feature-surfaces.md`
- `../realtime-sync.md`
- `../ui-map.md`
