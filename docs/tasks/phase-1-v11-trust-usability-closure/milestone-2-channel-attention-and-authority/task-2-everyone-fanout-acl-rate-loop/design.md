# Design: Everyone Fanout ACL Rate Loop

## Server Flow

Message creation keeps the existing validation order: authenticated sender, channel existence, private-channel access, channel membership, readonly gate, explicit mention validation, artifact-comment validation, message insert, then best-effort mention dispatch.

Task2 extends the mention segment:

- Reject non-empty request-body `mentions` before content validation can create a message.
- Parse `@Everyone` with an exact reserved-token matcher.
- Reject agent senders before storing the message.
- Apply an in-memory throttle keyed by `(channel_id, sender_id)` with the same 5 minute window shape as the existing offline-agent fallback throttle.
- Resolve recipients from `channel_members JOIN users`, excluding the sender and soft-deleted users.
- Persist the computed recipients to `message_mentions` and dispatch them through `MentionDispatcher.Dispatch`.

## Store Helper

`Store.ListEveryoneMentionTargets(channelID, senderID)` is the single helper for the server-computed recipient set. It does not accept caller-supplied target ids. It orders by join time for deterministic behavior, then user id as a stable tie-breaker.

The legacy display-name parser treats `@Everyone` as a reserved token and does not resolve a user literally named `Everyone` through the older `mentions` table path. The broadcast history for Task2 is `message_mentions`, not the legacy `mentions` table.

## Error Semantics

- Non-empty client recipient ids: `400 mention.client_recipients_rejected`.
- Agent-originated broadcast: `400 mention.everyone_agent_sender_rejected`.
- Sender/channel rate limit: `429 mention.everyone_rate_limited`.
- Recipient helper errors: `500 Internal server error` with server log context.

## Privacy And Loop Controls

The feature reuses existing mention dispatch privacy behavior. Offline agent fallback sends the owner a fixed system-DM nudge and does not include the raw source message body. Human offline targets do not receive owner fallback. Agent senders cannot trigger `@Everyone`, preventing an agent response loop from becoming a channel-wide broadcast.

## Files

- `packages/server-go/internal/api/messages.go`
- `packages/server-go/internal/api/mention_dispatch.go`
- `packages/server-go/internal/store/require_mention_policy.go`
- `packages/server-go/internal/store/queries.go`
- Focused API/store regression tests for fanout, ACL, client-recipient rejection, rate limiting, and agent-loop rejection.
