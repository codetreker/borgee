# Dev Design: requireMention Policy Model

## 1. Data Flow

Policy update flow:

```text
channel manager -> PUT /api/v1/channels/{channelId}/members/{userId}/require-mention
  -> user auth
  -> channel exists and non-DM
  -> caller has channel.manage_members for channel
  -> target is an agent channel member
  -> requested policy is inherit/on/off
  -> off checks target agent users.require_mention == false
  -> channel_members.require_mention_policy updated
  -> channel_updated/member event emitted
```

Message delivery-policy flow:

```text
message create -> explicit @target parser still validates mentioned targets
  -> store creates message and explicit mention rows
  -> message handler asks store for non-mention agent recipients allowed by policy
  -> those recipients are added to mention dispatch targets without adding message_mentions rows
  -> dispatcher uses existing online push / offline owner DM privacy behavior
```

The message integration is intentionally narrow: it gives non-mention delivery only to channel agent members whose effective policy resolves to off. Explicit `@agent` mention remains the primary route and keeps current cross-channel validation.

## 2. Data Model

Add `channel_members.require_mention_policy TEXT NOT NULL DEFAULT 'inherit'`.

Allowed values:

- `inherit`: resolve through `users.require_mention`.
- `on`: require explicit mention in this channel.
- `off`: allow non-mention delivery only when the agent owner's global setting has already opted into broader delivery.

Migration plan:

- Add a new forward-only migration after `helperJobs`.
- Add baseline compatibility in `store.applyColumnMigrations` for test/template and older boot paths.
- Add a GORM field to `store.ChannelMember` and a JSON field to `ChannelMemberInfo` so APIs can expose current state to later client-control work.

No existing rows are backfilled beyond the default `inherit` value.

## 3. API Contract

Endpoint:

```text
PUT /api/v1/channels/{channelId}/members/{userId}/require-mention
```

Request:

```json
{ "policy": "inherit" }
```

Response:

```json
{
  "channel_id": "...",
  "user_id": "...",
  "require_mention_policy": "inherit",
  "effective_require_mention": true
}
```

Status codes:

- `200`: policy updated.
- `400`: invalid policy, DM channel, human target, or owner-ceiling violation.
- `403`: caller lacks channel management authority.
- `404`: channel or target membership missing.

Error strings should be stable enough for tests but not introduced as UI copy locks; task 3 can decide presentation.

## 4. Edge Cases

- Missing or malformed JSON returns `400`.
- Unknown policy literal returns `400`.
- Target human member returns `400` because the policy is agent attention only.
- Target not in channel returns `404`.
- Caller is a member but lacks `channel.manage_members` returns `403`.
- `off` for an agent with global `require_mention=true` returns `400` and leaves stored policy unchanged.
- `inherit` for a legacy row with empty policy is treated as `inherit`.
- Explicit mentions still work even when policy is `on`.

## 5. Options Considered

Option A: store policy on `channel_members`.

- Pros: policy is naturally scoped to an agent's membership in one channel; no extra join table; easy to list with members.
- Cons: requires migration on an existing table.

Option B: store policy in a new `channel_member_attention_policies` table.

- Pros: clean separation from membership data.
- Cons: extra join/read path for every channel member list and message routing decision; more complex deletion semantics.

Chosen: Option A. The policy is membership-scoped, and `channel_members` already owns membership-level flags such as `silent`.

## 6. Integration Points

- Store models and queries: `packages/server-go/internal/store/models.go`, `queries.go`, and migration tests.
- API route: `packages/server-go/internal/api/channels.go`.
- Mention/message path: `packages/server-go/internal/api/messages.go` and `mention_dispatch.go` through existing dispatch behavior.
- Current docs: server data model, realtime/server behavior, security rail, and known gaps.

Task 6 Helper job transport owns different packages and docs (`helper_jobs`, `borgee-helper/internal/outbound`, Helper current docs). This task should not touch those surfaces.

## 7. Security / Privacy Threat Model

Sensitive assets:

- Agent attention/capability boundary.
- Channel membership and cross-org access.
- Private message bodies and offline owner fallback privacy.

Threats and controls:

- Channel owner broadens external agent attention: reject `off` unless the agent owner globally set `require_mention=false`.
- Non-manager changes policy: require `channel.manage_members` scoped to the channel.
- Cross-channel/cross-org update: target must be a member of the same channel and existing channel permission checks apply.
- Privacy leak through fallback: reuse existing dispatcher; do not add body-forwarding or new owner notification shape.
- Backfill leak: do not process historical messages or existing mention rows.
