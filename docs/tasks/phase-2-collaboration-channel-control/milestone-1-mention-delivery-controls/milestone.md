# Milestone 1: Mention Delivery Controls

## Capability Goal

Let channel context control agent attention and broadcast mentions without hidden fanout or capability expansion.

## Acceptance Boundary

Accepted by this milestone:

- Per-channel `requireMention` supports inherit/on/off semantics while preserving agent-owner authority.
- `@Everyone` is server-authoritative, ACL-filtered, rate-limited, and protected from agent recursion.

Rejected by this milestone:

- Client-supplied recipient IDs.
- Cross-channel or cross-org fanout outside server membership/ACL.
- Offline fallback that forwards private message bodies to an owner.

## Task-Split Trigger

Break down after phase-plan acceptance. Expected tasks should cover data/API, server fanout/ACL/rate guard, and client UI behavior separately.
