# Acceptance: Everyone Fanout ACL Rate Loop

## Segment A: Server-Computed Recipients

Acceptance checks:

- `@Everyone` recipients are computed from channel membership on the server.
- The sender is excluded.
- Deleted users and non-members are excluded.
- Computed recipients are persisted to `message_mentions`.

Negative checks:

- Client request bodies cannot supply recipient ids through `mentions`.
- A user literally named `Everyone` is not treated as a single display-name mention when the reserved token is used.

## Segment B: ACL And Privacy

Acceptance checks:

- Existing message-create channel visibility and membership gates still run before fanout.
- Cross-channel or cross-org delivery can only happen through channel membership/access rules.
- Offline agent fallback keeps the existing fixed owner system-DM body and does not forward raw message content.

Negative checks:

- No private channel body, file content, or hidden channel data is exposed by the broadcast expansion.

## Segment C: Rate And Loop Guards

Acceptance checks:

- Repeated `@Everyone` sends by the same sender in the same channel are rate-limited.
- Agent senders cannot trigger `@Everyone`.

Negative checks:

- No plugin/agent recursion path can turn an agent response into channel-wide broadcast fanout.

## Verification Evidence

Record fresh command evidence in `progress.md` before PR open:

- Focused RED and GREEN tests for store/API behavior.
- Affected package tests: `./internal/store ./internal/api`.
- Relevant full Go verification from `packages/server-go` with external `GOTMPDIR`.
- `git diff --check`.
