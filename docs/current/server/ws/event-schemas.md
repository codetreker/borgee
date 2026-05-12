# server-go — ws/event_schemas.go (RT-0 push frame schema)

> RT-0 (#237 server, #218 client) · blueprint `realtime.md §2.3` · CI lint ↔ `bpp/frame_schemas.go` (Phase 4) byte-identical

## 1. Scope

`internal/ws/event_schemas.go` is the **single source** for server → client push frames. Phase 2 uses the `/ws` hub; at Phase 4 BPP cutover, `bpp/frame_schemas.go` stays byte-identical with this file and client handlers do not change.

Field order is part of the contract. TS mirror lives in `packages/client/src/types/ws-frames.ts` (#218); JSON tag must equal TS field name. Adding a field requires adding it to both sides in the same PR, otherwise CI fails.

## 2. Frame List

| Frame | type string | Field order | Trigger |
|-------|------------|---------|--------|
| `AgentInvitationPendingFrame` | `agent_invitation_pending` | invitation_id, requester_user_id, agent_id, channel_id, created_at, expires_at | after `POST /api/v1/agent_invitations` writes to the database, push only to owner |
| `AgentInvitationDecidedFrame` | `agent_invitation_decided` | invitation_id, state, decided_at | `PATCH /api/v1/agent_invitations/{id}` pushes to requester + owner |
| `ArtifactUpdatedFrame` (RT-1.1 #269) | `artifact_updated` | type, cursor, artifact_id, version, channel_id, updated_at, kind | after CV-1 commit handler writes artifact row, push to all channel members; repeat `(artifact_id, version)` emit → same cursor (idempotent), hub does **not** double-push |
| `AnchorCommentAddedFrame` (CV-2.2 #360) | `anchor_comment_added` | type, cursor, anchor_id, comment_id, artifact_id, artifact_version_id, channel_id, author_id, author_kind, created_at | CV-2.2 anchor comment handler 写 `anchor_comments` 行后, 推 channel 全员; cursor 走 hub.cursors 同 RT-1.1 单调 sequence (反向约束: 不另起 anchor channel) |
| `MentionPushedFrame` (DM-2.2 #372) | `mention_pushed` | type, cursor, message_id, channel_id, sender_id, mention_target_id, body_preview, created_at | DM-2.2 mention dispatch handler `IsOnline(target)==true` → `BroadcastToUser(target_id, frame)` 单推 (反向约束: 不抄送 owner — owner 路径走 system DM fallback 不复用此 frame); body_preview 80 rune-safe 截断 (UTF-8 `utf8.RuneCountInString` 不切 CJK 字符, 隐私 §13 红线); cursor 走 hub.cursors 同 RT-1.1/CV-2.2 单调 sequence |
| `IterationStateChangedFrame` (CV-4.2 #409) | `iteration_state_changed` | type, cursor, iteration_id, artifact_id, channel_id, state, error_reason, created_artifact_version_id, completed_at | CV-4.2 iterate handler 写 `artifact_iterations` 行 + 每次 state machine 转移 (pending→running / pending→failed / running→completed / running→failed) → 推 channel 全员; cursor 走 hub.cursors 同 RT-1.1 / CV-2.2 / DM-2.2 单调 sequence (反向约束: 不另起 channel); state 4 态 byte-identical 跟 migration v=18 #405 CHECK 字面 + #380 文案锁定; error_reason / created_artifact_version_id / completed_at 在 pending/running 态零值始终序列化, 不挂 omitempty (反向约束 — 跟 AnchorComment resolved_at *T 指针模式不同) |

`expires_at = 0` is the sentinel (client TS `required: number`); `TestAgentInvitationPendingFrame_ZeroExpiresIsSentinel` locks wire parity.

## 3. Hub Push Entrypoints

| Method | Purpose |
|--------|------|
| `Hub.PushAgentInvitationPending(ownerUserID string, frame *AgentInvitationPendingFrame)` | push only to owner |
| `Hub.PushAgentInvitationDecided(userIDs []string, frame *AgentInvitationDecidedFrame)` | push to multiple users; POST path asserts direction with `got.UserID != frame.RequesterUserID` |
| `Hub.PushArtifactUpdated(artifactID string, version int64, channelID string, updatedAt int64, kind string) (cursor int64, sent bool)` | RT-1.1 — allocate monotonic cursor (no rollback after restart, seeded from `MAX(events.cursor)`) + `(artifact_id, version)` dedup; `sent=false` means repeated emit and hub already suppressed broadcast |
| `Hub.PushAnchorCommentAdded(...)` (CV-2.2 #360) | allocate monotonic cursor + push to all channel members; shares hub.cursors sequence with RT-1.1 |
| `Hub.PushMentionPushed(messageID, channelID, senderID, mentionTargetID, bodyPreview string, createdAt int64) (cursor int64, sent bool)` (DM-2.2 #372) | allocate monotonic cursor + `BroadcastToUser(mentionTargetID, frame)` single-target push (reverse constraint: target-only fanout, no owner copy); `sent=false` means hub.cursors was not configured (test seam); caller must truncate body_preview with `TruncateBodyPreview()` |
| `Hub.PushIterationStateChanged(iterationID, artifactID, channelID, state, errorReason string, createdArtifactVersionID, completedAt int64) (cursor int64, sent bool)` (CV-4.2 #409) | allocate monotonic cursor + push to all channel members (channelID==`""` → `BroadcastToAll` test fallback); `sent=false` only when hub.cursors is not configured (test seam); shares hub.cursors sequence with RT-1.1 / CV-2.2 / DM-2.2 |

### 3.1 Cursor Monotonicity Contract (RT-1.1)

- **Monotonic**: within the same origin server, cursor strictly increases; atomic int64 + CAS guarantees no duplicates across 100 concurrent calls (race detector test `TestCursorMonotonicUnderConcurrency`).
- **No rollback**: `NewCursorAllocator(s)` seeds in-memory head from `Store.GetLatestCursor()` (that is, `MAX(events.cursor)`), so the first cursor after restart is greater than the pre-restart maximum (`TestCursorNoRollbackAfterRestart`).
- **Idempotent**: repeated emit of the same `(artifact_id, version)` always returns the same cursor and `fresh=false` (`TestCursorIdempotentSameArtifactVersion` + `TestHubPushArtifactUpdatedDedup`); client RT-1.2 rendered-set dedup fails closed.
- **grep check** (RT-1 spec §3, 0 matches): `artifact_updated.*timestamp|sort.*ArtifactUpdated.*time` in `internal/ws/` (non-_test.go). client **must not** sort by `updated_at`; it must sort by `cursor`.

## 4. Out of Scope

- No ack/retry — RT-0 is best-effort; client disconnection relies on cursor replay (events table) as fallback.
- Complete BPP frame set (Phase 4) is added in sync with this file and is outside Phase 2 scope.
