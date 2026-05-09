# server-go — realtime push 路径

> RT-0 (#40) · Phase 2 临时载体 `/ws` hub · Phase 4 BPP cutover 不动 schema

## 1. 适用范围

CM-4 流程 B 邀请回路的 push 通知:

- `agent_invitation_pending` — owner 端: 有人想拉你的 agent 进 channel
- `agent_invitation_decided` — 跨 client 同步 (owner 在另一端 approve/reject, 或 server 标 expired)

替代 60s 轮询; 落 ≤ 3s 端到端 latency (G2.4 hardline).

## 2. 服务端 API

文件:

| 文件 | 角色 |
|------|------|
| `internal/ws/event_schemas.go` | typed frame structs + discriminator 常量 (source of truth) |
| `internal/ws/hub.go` | `Hub.PushAgentInvitationPending(userID, *Frame)` / `Hub.PushAgentInvitationDecided(userID, *Frame)` |
| `internal/api/agent_invitations.go` | `AgentInvitationPusher` interface + handler 调用点 (POST + PATCH) |
| `internal/server/server.go` | `agentInvitationHandler.Hub = s.hub` 装配 |

行为:

- `userID == ""` 或 `frame == nil` → 静默 no-op (handler 一侧也 nil-guard, Hub 一侧二次 nil-guard).
- `userID` 没有 live session → 静默 drop. Persisted invitation 行是 source of truth, 客户端 reconnect / bell-poll fallback 自我对齐.
- 每次 push 后 `Hub.SignalNewEvents()` 唤醒 `/events` long-poll waiter, 与 `BroadcastEventTo*` 行为一致.

## 3. handler 触发点

POST `/api/v1/agent_invitations` 创建后, push pending → `agent.OwnerID`.

PATCH `/api/v1/agent_invitations/{id}` (approve/reject) 落库后, push decided → 双方 (`inv.RequestedBy` + `agent.OwnerID`); 多设备并发 (realtime.md §1.4 A 全推默认).

## 4. schema 锁

字段顺序 + JSON tag 必须与 `packages/client/src/types/ws-frames.ts` (PR #218) 字面对齐:

```
pending : type, invitation_id, requester_user_id, agent_id, channel_id, created_at, expires_at
decided : type, invitation_id, state, decided_at
```

`expires_at` 客户端类型为 `number` (required), 服务器无 row-level expiry 时也必须 emit (sentinel `0`, 非 `omitempty`).

`internal/ws/push_agent_invitation_test.go::TestAgentInvitationPendingFrame_WireSchema` 是 schema 红线: 加/删字段不在两边同 PR 同步会 CI red.

## 5. Phase 4 BPP cutover

调用方 (`agent_invitations.go` handler) 不动. `Hub.PushAgentInvitation*` 实现内部把 `BroadcastToUser` 换成 `bpp.SendFrame` 即可. struct 字段顺序锁让 `bpp/frame_schemas.go` 与 `ws/event_schemas.go` 可 type-alias 共用 (G2.6 CI lint 兜底).

## 6. Phase 3 fanout 路径扩展 (CV-1/CV-2/DM-2 同 hub 多 frame)

> 跟 §1 CM-4 邀请 push 同 hub.cursors 单调 sequence, 不另起 channel — 详见 `ws/event-schemas.md`.

### 6.1 ArtifactUpdated (RT-1.1 #290 + CV-1.2 #342)

CV-1 commit/rollback handler 写 `artifacts` 行后, `Hub.PushArtifactUpdated` 推 channel 全员 (channel-scoped fanout); 同 `(artifact_id, version)` 重发 → 同 cursor (idempotent dedup), 重启不回退 (cursor MAX seed)。client `wsClient.ts` switch type='artifact_updated' → mutate `/api/v1/artifacts/:id` 拉 body (envelope 仅信号, 设计 ⑤ 反约束 envelope 不带 body)。

### 6.2 AnchorCommentAdded (CV-2.2 #360)

CV-2.2 anchor comment handler 写 `anchor_comments` 行后, push channel 全员 (channel ACL 同源, 跟 CV-1.2 #342 cross-channel 403 同 RollbackOwnerOnly 模式)。10 字段 envelope (含 author_kind/artifact_version_id) 跟 RT-1.1 7 字段 / DM-2.2 8 字段共 hub.cursors 单调发号 — BPP-1 #304 envelope CI lint reflect 比对自动闸位。**反约束**: agent 只能在 human-anchored thread 接龙 reply, agent-only thread 反 → 403 (蓝图 §1.6 锚点 = 人审 agent 产物字面禁 AI 自循环)。

### 6.3 MentionPushed (DM-2.2 #372)

DM-2.2 mention dispatch handler `parser regex @([0-9a-f-]{36})` 落 `message_mentions` 行后, 走 `IsOnline(target)` 真接 AL-3 #310 SessionsTracker:
- **在线** → `Hub.PushMentionPushed` `BroadcastToUser(target_id, frame)` 单推 (反约束: target-only fanout, 不抄送 owner — 设计 ③ 蓝图 §4 字面)
- **离线** → `enqueueOwnerSystemDM` 写 owner ↔ agent 内置 DM 一行 `messages.type='system'` + 5min/(agent, channel) 节流 (clock fixture, 跟 G2.3 节流模式同源); 文案 byte-identical `"{agent_name} 当前离线，#{channel} 中有人 @ 了它，你可能需要处理"` (跟 #314 文案锁 + #293 §2.2 acceptance 同源)

8 字段 envelope `body_preview` 80 rune-safe 截断 (`utf8.RuneCountInString`, 不切 CJK 字符) — 隐私 §13 红线 (完整 body 走 `new_message` event channel ACL 授权路径, 不通过此 frame)。**反约束** (DM-2.2 自查 4 锚 0 hit): `mention.*owner_id` / `cc.*owner` / `notify.*owner_id` / `system.*DM.*body`; `@channel` 0 hit (留 DM-3)。

### 6.4 IterationStateChanged (CV-4.2 #409)

CV-4.2 iterate handler 写 `artifact_iterations` 行后 + 每次 state machine 转移 (pending → running / pending → failed / running → completed / running → failed) 触发 `Hub.PushIterationStateChanged` 推 channel 全员 (channel-scoped fanout 跟 RT-1.1 / CV-2.2 同模式)。9 字段 envelope: type / cursor / iteration_id / artifact_id / channel_id / state / error_reason / created_artifact_version_id / completed_at — cursor 走 hub.cursors 同 RT-1.1 / CV-2.2 / DM-2.2 单调 sequence (反约束: 不另起 channel)。**反约束**: error_reason / created_artifact_version_id / completed_at 在 pending/running 态时为零值 (string="" / int64=0), 始终序列化, 不挂 omitempty (跟 AnchorComment resolved_at 模式不同); state CHECK 4 态 byte-identical 跟 #380 文案锁 + migration v=18 字面; AL-4 stub fail-closed → state='failed' + error_reason='runtime_not_registered' (跟 AL-1a #249 6 reason 同源, 设计 ⑤)。

### 6.5 cursor 共序契约 (跨 RT-1 / CV-2 / DM-2 / CV-4)


四 frame (ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9) 共一根 `hub.cursors` 单调 sequence (RT-1.1 atomic int64 + CAS 保 100 并发无重复, 重启从 `MAX(events.cursor)` seed)。client 不可按 `created_at`/`updated_at` 排序, 必须按 `cursor` (RT-1 反约束)。BPP-1 #304 envelope CI lint reflect 比对 `bpp/frame_schemas.go` ↔ server-go 端 frame struct 字段顺序自动闸位 — 改字段顺序 = lint fail = PR 卡。
