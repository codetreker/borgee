# server-go — ws/event_schemas.go (RT-0 push frame schema)

> RT-0 (#237 server, #218 client) · 蓝图 `realtime.md §2.3` · CI lint ↔ `bpp/frame_schemas.go` (Phase 4) byte-identical

## 1. 适用范围

`internal/ws/event_schemas.go` 是 server → client push frame 的 **source of truth**. Phase 2 走 `/ws` hub, Phase 4 BPP cutover 时 `bpp/frame_schemas.go` 跟此 file 字节相同, 客户端 handler 0 改.

字段顺序是契约的一部分. TS 镜像在 `packages/client/src/types/ws-frames.ts` (#218); JSON tag 必须 = TS field name. 加字段 = 两端同 PR 加, 否则 CI 红.

## 2. Frame 清单

| Frame | type 字符串 | 字段顺序 | 触发点 |
|-------|------------|---------|--------|
| `AgentInvitationPendingFrame` | `agent_invitation_pending` | invitation_id, requester_user_id, agent_id, channel_id, created_at, expires_at | `POST /api/v1/agent_invitations` 写库后, 推 owner 单端 |
| `AgentInvitationDecidedFrame` | `agent_invitation_decided` | invitation_id, state, decided_at | `PATCH /api/v1/agent_invitations/{id}` 双推 (requester + owner) |
| `ArtifactUpdatedFrame` (RT-1.1 #269) | `artifact_updated` | type, cursor, artifact_id, version, channel_id, updated_at, kind | CV-1 commit handler 写 artifact 行后, 推 channel 全员; 同 `(artifact_id, version)` 重发 → 同 cursor (idempotent), hub **不**双推 |
| `AnchorCommentAddedFrame` (CV-2.2 #360) | `anchor_comment_added` | type, cursor, anchor_id, comment_id, artifact_id, artifact_version_id, channel_id, author_id, author_kind, created_at | CV-2.2 anchor comment handler 写 `anchor_comments` 行后, 推 channel 全员; cursor 走 hub.cursors 同 RT-1.1 单调 sequence (反约束: 不另起 anchor channel) |
| `MentionPushedFrame` (DM-2.2 #372) | `mention_pushed` | type, cursor, message_id, channel_id, sender_id, mention_target_id, body_preview, created_at | DM-2.2 mention dispatch handler `IsOnline(target)==true` → `BroadcastToUser(target_id, frame)` 单推 (反约束: 不抄送 owner — owner 路径走 system DM fallback 不复用此 frame); body_preview 80 rune-safe 截断 (UTF-8 `utf8.RuneCountInString` 不切 CJK 字符, 隐私 §13 红线); cursor 走 hub.cursors 同 RT-1.1/CV-2.2 单调 sequence |
| `IterationStateChangedFrame` (CV-4.2 #409) | `iteration_state_changed` | type, cursor, iteration_id, artifact_id, channel_id, state, error_reason, created_artifact_version_id, completed_at | CV-4.2 iterate handler 写 `artifact_iterations` 行 + 每次 state machine 转移 (pending→running / pending→failed / running→completed / running→failed) → 推 channel 全员; cursor 走 hub.cursors 同 RT-1.1 / CV-2.2 / DM-2.2 单调 sequence (反约束: 不另起 channel); state 4 态 byte-identical 跟 migration v=18 #405 CHECK 字面 + #380 文案锁; error_reason / created_artifact_version_id / completed_at 在 pending/running 态零值始终序列化, 不挂 omitempty (反约束 — 跟 AnchorComment resolved_at *T 指针模式不同) |

`expires_at = 0` 是 sentinel (client TS `required: number`); `TestAgentInvitationPendingFrame_ZeroExpiresIsSentinel` 锁 wire parity.

## 3. Hub 推送入口

| Method | 用途 |
|--------|------|
| `Hub.PushAgentInvitationPending(ownerUserID string, frame *AgentInvitationPendingFrame)` | 单推 owner |
| `Hub.PushAgentInvitationDecided(userIDs []string, frame *AgentInvitationDecidedFrame)` | 多推, POST 路径双推方向断言 `got.UserID != frame.RequesterUserID` |
| `Hub.PushArtifactUpdated(artifactID string, version int64, channelID string, updatedAt int64, kind string) (cursor int64, sent bool)` | RT-1.1 — 分配单调 cursor (重启不回退, 由 `events.cursor` MAX 种子) + `(artifact_id, version)` dedup; `sent=false` 表示重发, hub 已抑制广播 |
| `Hub.PushAnchorCommentAdded(...)` (CV-2.2 #360) | 分配单调 cursor + 推 channel 全员; 跟 RT-1.1 共 hub.cursors sequence |
| `Hub.PushMentionPushed(messageID, channelID, senderID, mentionTargetID, bodyPreview string, createdAt int64) (cursor int64, sent bool)` (DM-2.2 #372) | 分配单调 cursor + `BroadcastToUser(mentionTargetID, frame)` 单推 (反约束: target-only fanout, 不抄 owner); `sent=false` 表示 hub.cursors 未配 (test seam); body_preview 调用方需 `TruncateBodyPreview()` 截断 |
| `Hub.PushIterationStateChanged(iterationID, artifactID, channelID, state, errorReason string, createdArtifactVersionID, completedAt int64) (cursor int64, sent bool)` (CV-4.2 #409) | 分配单调 cursor + 推 channel 全员 (channelID==`""` → `BroadcastToAll` test fallback); `sent=false` 仅当 hub.cursors 未配 (test seam); 跟 RT-1.1 / CV-2.2 / DM-2.2 共 hub.cursors sequence |

### 3.1 Cursor 单调契约 (RT-1.1)

- **单调**: 同 origin server 内 cursor 严格递增, atomic int64 + CAS 保 100 并发无重复 (race detector 单测 `TestCursorMonotonicUnderConcurrency`).
- **不回退**: `NewCursorAllocator(s)` 从 `Store.GetLatestCursor()` (即 `MAX(events.cursor)`) 种子 in-memory head, 重启后第一个 cursor > 重启前最大值 (`TestCursorNoRollbackAfterRestart`).
- **Idempotent**: 同 `(artifact_id, version)` 重 emit 必然返回同 cursor 且 `fresh=false` (`TestCursorIdempotentSameArtifactVersion` + `TestHubPushArtifactUpdatedDedup`); client RT-1.2 已渲染集 dedup fail-closed.
- **grep 检查项** (RT-1 spec §3, 0 命中): `artifact_updated.*timestamp|sort.*ArtifactUpdated.*time` in `internal/ws/` (非 _test.go). client **不可**按 `updated_at` 排序, 必须按 `cursor`.

## 4. 不在范围

- 不带 ack/retry — RT-0 走 best-effort, 客户端断线靠 cursor replay (events 表) 兜底.
- BPP frame 完整集 (Phase 4) 跟此 file 同步加, 不在 Phase 2 范围.
