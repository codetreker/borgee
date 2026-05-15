# Milestone 2: Channel Attention And Authority

## Capability Goal

Let users understand and control agent attention, broadcast mention behavior, channel membership/ownership actions, allowed actions, authority checks, and private/sidebar state meaning without hidden fanout or confusing owner leave behavior.

## Canonical Task Homes

This milestone is the only execution home for the channel attention and authority work. The former channel-control phase folder was never executed and has been removed to avoid presenting it as an available Phase.

The collapsed planning content is represented here as canonical tasks: mention delivery controls, channel management authority, and channel visual truth all belong inside this one milestone.

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

| Task | Status | Canonical path | Depends on | Parallel? |
|---|---|---|---|---|
| requireMention policy model | ACCEPTING | `task-1-requiremention-policy-model` | Milestone start | no |
| `@Everyone` fanout ACL/rate loop | ACCEPTING | `task-2-everyone-fanout-acl-rate-loop` | requireMention policy | yes, after policy |
| Client mention controls | ACCEPTING | `task-3-client-mention-controls` | policy and fanout behavior | no |
| Channel management surface | ACCEPTING | `task-4-channel-management-surface` | Milestone start | yes, if files are disjoint from mention policy |
| Channel allowed-action rules | TASKING | `task-5-channel-allowed-action-rules` | management surface | yes, after management surface |
| Channel authority checks | PLANNED | `task-6-channel-authority-checks` | management surface and allowed-action rules | no |
| Private indicator state inventory | ACCEPTED | `task-7-private-indicator-state-inventory` | Explicit parallel UI slot; merged PR #945 | no |
| Private indicator visual treatment | ACCEPTING | `task-8-private-indicator-visual-treatment` | state inventory accepted | no |
| Sidebar state collision regression | PLANNED | `task-9-sidebar-state-collision-regression` | visual treatment | no |

## Exit Gates

- `@Everyone` fanout is server-authoritative and never accepts client recipient IDs.
- Channel owners cannot broaden external-agent attention or capability beyond agent-owner authorization.
- Self-created or owned channels do not expose confusing leave actions.
- Private indicators do not hide unread, fault, presence, selection, or hover states.
