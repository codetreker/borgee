# task-1-requiremention-policy-model

Purpose:
- Let channel context reduce or require agent attention without broadening external-agent capability.

Scope:
- Add per-channel `requireMention` inherit/on/off semantics and authority checks.
- Preserve that agent owner authorization is the ceiling and channel owners can only reduce, mute, or remove delivery.

Out of scope:
- No `@Everyone` fanout implementation.
- No notification, collapse, sort, or history backfill rewrite.

Depends on:
- Canonical Milestone 2 start or explicit Teamlead parallel start decision

Blueprint anchors:
- `MR-1`: `docs/blueprint/next/migration-analysis.md` §3.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify `requireMention` supports inherit/on/off and cannot let a channel owner expand an external agent's attention or capability.

Parallelism:
- First task for this milestone after execution slot clears. Blocks fanout and client-control tasks.

Sensitive paths:
- auth, channel ACL, cross-org capability, privacy
