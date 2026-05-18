# task-3-client-mention-controls

Purpose:
- Make mention delivery behavior understandable and controllable in the client.

Scope:
- Expose channel mention delivery settings and `@Everyone` behavior in the relevant client surface.
- Show truthful control state without implying broader delivery authority than the server grants.

Out of scope:
- No broad visual redesign, notification center rewrite, or history backfill UI.

Depends on:
- `task-1-requiremention-policy-model`
- `task-2-everyone-fanout-acl-rate-loop`

Blueprint anchors:
- `MR-1`: `docs/blueprint/next/migration-analysis.md` §3.3

Acceptance slice:
- A reviewer can use the client to understand channel mention delivery state and broadcast behavior while server authority remains the source of truth.

Parallelism:
- Runs after server policy and fanout behavior are available.

Sensitive paths:
- channel ACL visibility, privacy, client authority display
