# Agent Manager Sketch

## Purpose

This sketch is an Interaction And Layout Reference for owner-side agent management. It does not define product behavior, implementation contracts, security posture, or verification status.

## Surface

Agent Manager is a global sidepane in the user SPA. It lets a signed-in user inspect and manage agents they own, while server APIs remain responsible for persistence, ownership checks, and sensitive value handling.

## Interaction Model

- The list view selects an agent and expands a detail surface.
- Detail content is grouped into identity, credentials, runtime, config, permissions, and channel membership areas.
- Sensitive credentials are presented as masked values in the browser surface.
- Agent config editing stays in the config sub-surface; the manager coordinates surrounding detail layout.

## Layout Sketch

```
+──────────────────────────────────────────────────+
│  [Identity card]                                  │
│  Avatar  AgentX  - online                         │
│      ID: ag-12345... | Created: 2026-04-29       │
│                              [Collapse] [Delete] │
├──────────────────────────────────────────────────┤
│  [Credentials card]                               │
│  API Key                                          │
│  bgr_...abc1                          [Copy]     │
│  [Rotate API Key]                                 │
├──────────────────────────────────────────────────┤
│  [Runtime card]                                   │
├──────────────────────────────────────────────────┤
│  [Config card]                                    │
├──────────────────────────────────────────────────┤
│  [Permissions card]                               │
├──────────────────────────────────────────────────┤
│  [Channels card]                                  │
+──────────────────────────────────────────────────+
```

## Architecture Notes

- Agent management is a user-owned workflow, not an admin console workflow.
- Credentials should not become shared app state; keep sensitive display and copy behavior local to the surface.
- Runtime metadata is read as owner-facing operational information, separate from admin runtime metadata.
- Durable changes flow through the user REST rail.

## Related Docs

- `../feature-surfaces.md`
- `../ui-map.md`
- `agent-config.md`
