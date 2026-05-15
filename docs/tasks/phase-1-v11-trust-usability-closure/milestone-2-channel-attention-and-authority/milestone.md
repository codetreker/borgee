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
| requireMention policy model | ACCEPTED | `task-1-requiremention-policy-model` | PR #949 (`c25ef60`) | complete |
| `@Everyone` fanout ACL/rate loop | ACCEPTED | `task-2-everyone-fanout-acl-rate-loop` | PR #951 (`3659ce1`) | complete |
| Client mention controls | ACCEPTED | `task-3-client-mention-controls` | PR #955 (`0dd35a9`) | complete |
| Channel management surface | ACCEPTED | `task-4-channel-management-surface` | PR #948 (`077cb8c`) | complete |
| Channel allowed-action rules | ACCEPTED | `task-5-channel-allowed-action-rules` | PR #953 (`6ae4604`) | complete |
| Channel authority checks | ACCEPTED | `task-6-channel-authority-checks` | PR #959 (`66c9a35`) | complete |
| Private indicator state inventory | ACCEPTED | `task-7-private-indicator-state-inventory` | Explicit parallel UI slot; merged PR #945 | no |
| Private indicator visual treatment | ACCEPTED | `task-8-private-indicator-visual-treatment` | PR #952 (`965fcd7`) | complete |
| Sidebar state collision regression | ACCEPTED | `task-9-sidebar-state-collision-regression` | PR #961 (`1e6d54c`) | complete |

Milestone 2 is accepted. This Task12 closure PR only records the state sync; it does not reopen channel attention or authority scope.

## Exit Gates

- `@Everyone` fanout is server-authoritative and never accepts client recipient IDs.
- Channel owners cannot broaden external-agent attention or capability beyond agent-owner authorization.
- Self-created or owned channels do not expose confusing leave actions.
- Private indicators do not hide unread, fault, presence, selection, or hover states.
