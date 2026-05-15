# Milestone 2: Channel Attention And Authority

## Capability Goal

Let users understand and control agent attention, broadcast mention behavior, channel membership/ownership actions, allowed actions, authority checks, and private/sidebar state meaning without hidden fanout or confusing owner leave behavior.

## Remapped Prior Structure

This milestone replaces the old Phase 2 milestone split:

- `phase-2-collaboration-channel-control/milestone-1-mention-delivery-controls`
- `phase-2-collaboration-channel-control/milestone-2-channel-management-authority`
- `phase-2-collaboration-channel-control/milestone-3-channel-visual-truth`

Old Phase 2 was an execution slot, not a prerequisite or integration boundary. Those folders remain detailed task homes; this file is the authoritative coarse milestone grouping.

## Acceptance Boundary

Accepted by this milestone:

- Per-channel `requireMention` policy with inherit/on/off semantics that cannot broaden external-agent capability beyond agent-owner authorization.
- Server-authoritative `@Everyone` fanout computed from membership/ACL with rate and loop guards.
- Client mention controls that truthfully expose attention behavior.
- Channel management, allowed-action rules, and authority checks for membership/ownership actions.
- Private indicator treatment and sidebar state collision regression for private/unread/fault/presence states.

Rejected by this milestone:

- Client-supplied recipient IDs or cross-channel/cross-org fanout outside server membership/ACL.
- Owner transfer, hard delete, notification rewrite, collapse/sort rewrite, or broad visual redesign unless a task explicitly scopes it.
- Any privacy/security boundary change hidden as a UI treatment.

## Task Index

| Task | Status | Prior path | Depends on | Parallel? |
|---|---|---|---|---|
| requireMention policy model | PLANNED | `phase-2-collaboration-channel-control/milestone-1-mention-delivery-controls/task-1-requiremention-policy-model` | Milestone start | no |
| `@Everyone` fanout ACL/rate loop | PLANNED | `phase-2-collaboration-channel-control/milestone-1-mention-delivery-controls/task-2-everyone-fanout-acl-rate-loop` | requireMention policy | yes, after policy |
| Client mention controls | PLANNED | `phase-2-collaboration-channel-control/milestone-1-mention-delivery-controls/task-3-client-mention-controls` | policy and fanout behavior | no |
| Channel management surface | PLANNED | `phase-2-collaboration-channel-control/milestone-2-channel-management-authority/task-1-channel-management-surface` | Milestone start | yes, if files are disjoint from mention policy |
| Channel allowed-action rules | PLANNED | `phase-2-collaboration-channel-control/milestone-2-channel-management-authority/task-2-channel-allowed-action-rules` | management surface | yes, after management surface |
| Channel authority checks | PLANNED | `phase-2-collaboration-channel-control/milestone-2-channel-management-authority/task-3-channel-authority-checks` | management surface and allowed-action rules | no |
| Private indicator state inventory | PLANNED | `phase-2-collaboration-channel-control/milestone-3-channel-visual-truth/task-1-private-indicator-state-inventory` | Milestone start or alongside management surface | yes, if UI ownership is clean |
| Private indicator visual treatment | PLANNED | `phase-2-collaboration-channel-control/milestone-3-channel-visual-truth/task-2-private-indicator-visual-treatment` | state inventory | no |
| Sidebar state collision regression | PLANNED | `phase-2-collaboration-channel-control/milestone-3-channel-visual-truth/task-3-sidebar-state-collision-regression` | visual treatment | no |

## Exit Gates

- `@Everyone` fanout is server-authoritative and never accepts client recipient IDs.
- Channel owners cannot broaden external-agent attention or capability beyond agent-owner authorization.
- Self-created or owned channels do not expose confusing leave actions.
- Private indicators do not hide unread, fault, presence, selection, or hover states.
