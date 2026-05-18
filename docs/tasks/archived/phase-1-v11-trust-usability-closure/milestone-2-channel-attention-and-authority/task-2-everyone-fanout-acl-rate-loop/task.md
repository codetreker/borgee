# task-2-everyone-fanout-acl-rate-loop

Purpose:
- Add `@Everyone` broadcast without hidden or unauthorized fanout.

Scope:
- Implement server-authoritative recipient computation from membership/ACL, rate limits, and agent recursion prevention.
- Reject client-supplied recipient IDs and cross-channel/cross-org fanout outside access rules.

Out of scope:
- No offline fallback that forwards private message bodies to owners.
- No broad notification system rewrite.

Depends on:
- `task-1-requiremention-policy-model`

Blueprint anchors:
- `MR-1`: `docs/blueprint/next/migration-analysis.md` §3.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify `@Everyone` fanout is computed server-side, ACL-filtered, rate-limited, and cannot be recursively triggered by agents.

Parallelism:
- Can run after task 1. Can overlap with task 3 only after server behavior is testable enough for UI wiring.

Sensitive paths:
- auth, channel ACL, privacy, cross-org fanout
