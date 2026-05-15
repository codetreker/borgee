# Spec: Everyone Fanout ACL Rate Loop

## Source Alignment

- Task: `task-2-everyone-fanout-acl-rate-loop`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Depends on: Task1 `requireMention policy model`, merged in PR #949 at `c25ef60b1b1b3ccf71ba1997e70523e34b73ca34`
- Blueprint anchors: `docs/blueprint/next/migration-analysis.md` section 3.3 and section 6.1

## In Scope

- Recognize literal `@Everyone` in message content on the server message-create path.
- Compute recipients from current channel membership, excluding the sender and deleted users.
- Persist computed `@Everyone` recipients to `message_mentions` so the broadcast is reviewable as mention history.
- Dispatch through the existing mention fanout path so online users, push notification seams, and offline-agent owner fallback keep the existing privacy behavior.
- Reject non-empty client-supplied `mentions` arrays on message create.
- Rate-limit repeated `@Everyone` broadcasts per sender/channel.
- Reject `@Everyone` from agent senders so agents cannot recursively broadcast to a channel.

## Out Of Scope

- No channel management UI, Settings UI, or client mention controls.
- No private indicator or sidebar state work.
- No broad notification system rewrite.
- No schema migration.
- No owner transfer, leave, archive, delete, or allowed-action rule changes.

## Boundary

`@Everyone` is a server-derived mention target expansion, not a client recipient list. The client may send message content containing the reserved token, but it may not send recipient ids. Channel ACL remains membership-based: only members of the target channel are included, and the sender must already pass the existing message-send membership and visibility gates.

Agent-originated `@Everyone` is rejected before message creation. This keeps agent-to-agent loops and plugin-generated broadcast recursion out of the v1 surface.
