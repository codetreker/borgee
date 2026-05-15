# Phase 2: Collaboration Channel Control

## Source Anchors

- `MR-1`: Mention routing granularity and `@Everyone` broadcast.
- `CH-1`: Channel authority and user-side channel management.
- `PS-1`: Privacy/security guardrail for ACL and rail separation.
- Source issues: gh#674, gh#693, gh#685, gh#688, gh#690, gh#654.

## Value Loop

A user can understand and control who receives attention in a channel, what channel actions are allowed, and what private-channel indicators mean, without hidden fanout or confusing owner leave behavior.

## Milestones

| Milestone | Goal | Status | Task-split trigger |
|---|---|---|---|
| `milestone-1-mention-delivery-controls` | Add per-channel mention delivery controls and `@Everyone` fanout with server-authoritative ACL/rate/loop guards | PLANNED | Break down after Phase 1 planning is stable or in parallel if no shared files conflict |
| `milestone-2-channel-management-authority` | Add user-side channel management for membership/ownership actions and clarify owner cannot leave self-owned channels | PLANNED | Break down with mention controls if UI/API surfaces are independent enough |
| `milestone-3-channel-visual-truth` | Reduce private-channel lock weight and prevent collisions with unread, fault, or presence indicators | PLANNED | Break down with channel-management UI if design surface is shared |

This Phase has 3 user-facing milestones, within the project default.

## Exit Gates

Strict checks:

- `@Everyone` fanout is server-authoritative and computed from membership/ACL; client never supplies recipient IDs.
- Per-channel `requireMention` cannot let channel owners broaden external-agent attention or capability beyond agent-owner authorization.
- Owner/self-created channels do not expose confusing leave actions.
- Private indicators do not collide with unread/fault/presence states.

User-perceivable checks:

- Users can see and understand channel mention/attention behavior.
- Users can find channel membership/ownership actions.
- Private channels are identifiable without dominating the sidebar or hiding more important state.

Carry-over checks:

- Notification, collapse, and sort controls may stay existing behavior unless a task explicitly scopes them.
- Accessibility/quality backlog issues remain out unless explicitly pulled into this Phase.
