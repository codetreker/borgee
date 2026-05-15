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

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-requiremention-policy-model` | PLANNED | Add inherit/on/off channel mention policy without expanding agent-owner authority | Phase 1 execution slot or explicit parallel start | no | after dependency clears |
| `task-2-everyone-fanout-acl-rate-loop` | PLANNED | Add server-authoritative `@Everyone` fanout with ACL, rate, and loop guards | `task-1-requiremention-policy-model` | yes, after task 1 | no |
| `task-3-client-mention-controls` | PLANNED | Expose mention delivery controls and broadcast behavior truthfully in client UI | `task-1-requiremention-policy-model`, `task-2-everyone-fanout-acl-rate-loop` | no | no |

Dependency order: this task set can be reviewed now. Execution may run in parallel with Phase 1 only if Teamlead confirms no shared-file conflict and authority review capacity is available.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Channel policy, server fanout, and client controls keep channel owner authority separate from agent owner authority. |
| PM | LGTM | Attention-control value is split into policy, broadcast behavior, and truthful client controls. |
| QA | LGTM | Fanout, ACL, rate/loop guards, and no-backfill behavior are testable in the task slices. |
| Dev | LGTM | The model, server fanout loop, and client control tasks are one-PR sized with clear dependency order. |
| Security | LGTM | ACL fanout, cross-org boundaries, and private message body handling are marked for security review in execution. |
