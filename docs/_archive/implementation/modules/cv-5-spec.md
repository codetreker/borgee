# CV-5 spec brief — artifact comments (canvas vision, agent ↔ comment 单源)

> 战马E · Phase 5 收尾 · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 字面 "心智更接近 Linear issue + comment，而不是 Miro 白板" + DM-2.2 #372 mention 同源 fan-out + RT-3 #488 cursor 共序锚 + AL-1b/BPP-2.2 thinking subject 5-pattern 反约束链.

## 0. 关键约束 (3 项立场, 蓝图字面)

1. **artifact comments 走 messages 表单源, channel_id 复用 `artifact:<id>` namespace** (蓝图 L24 字面 "Linear issue + comment" + DM-2.2 #372 mention router 同精神 — comment / message 同表同语义不裂): 不另起 `artifact_comments` 表; comment row 落 `messages` (#293) + `channel_id = "artifact:<artifact_id>"` (虚拟 channel namespace, 跟 DM-2 #312 user_id pair 同 dm: namespace 模式承袭). **反约束**: 不开 `artifact_comments` 新表 / 不裂 `comment_id` 不同生命周期 / 不开新 schema migration; 反向 grep `CREATE TABLE.*artifact_comments|artifact_comments.*PRIMARY` 在 `internal/migrations/` count==0.

2. **comment 走 RT-3 既有 fan-out, cursor 共序** (跟 RT-3 #488 AgentTaskStateChangedFrame + AL-2b #481 + DM-2.2 #372 + BPP-2 #485 + BPP-3.1 #494 共一根 hub.cursors sequence): comment frame **复用 MentionPushedFrame 模式** (8 字段, type=`artifact_comment_added`, channel_id=`artifact:<id>`, sender_id=agent/user, body_preview=80 rune 截断 — 跟 DM-2.2 #372 隐私 §13 同 cap), 不另起 plugin-only 通道. **反约束**: 不另开 cursor sequence; 不挂 admin god-mode 抄送 (ADM-0 §1.3 红线); 反向 grep `comment.*hub\.cursors\.NextCursor.*[^"]plugin|admin.*comment` count==0.

3. **agent comment 必带 thinking subject 反约束 (5-pattern 第 4 处链)** (蓝图 realtime §1.1 ⭐ "thinking 必须带 subject" + RT-3 #488 + BPP-2.2 #485 + AL-1b #482 同链承袭, CV-5 是第 4 处): server 接 agent POST /comments 时, 若 sender_role==`agent` 且 body 含 `thinking` literal 但无 subject context (比如 "@thinking" 占位 / "AI is thinking" 字面无信息), reject 400 `comment.thinking_subject_required`. **反向 grep 守门**: `body.*"thinking"$|comment.*defaultSubject|comment.*fallbackSubject|"AI is thinking"|comment.*subject\s*=\s*""` 在 `internal/api/` 排除 _test.go count==0. 5-pattern 改 = 改 4 处 (RT-3 + BPP-2.2 + AL-1b + CV-5).

## 1. 拆段实施 (3 段, 一 milestone 一 PR 协议)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-5.1 server | `internal/api/artifact_comments.go` (新) + `messages.go::handleCreateMessage` 改 (虚拟 channel namespace 路由) + `channels.go::ResolveChannel` 改 (识别 `artifact:` prefix) | POST `/api/v1/artifacts/:id/comments` body `{body, agent_id?}` → 复用 MessageHandler.handleCreateMessage 注入 `channel_id = "artifact:<id>"` + 走 message_mentions 已有 fan-out + RT-3 cursor 共序 push `artifact_comment_added` frame; ACL 复用 channel_members (artifact channel 自动加 channel.created_by + agent_id owner); thinking 反约束 validator (sender_role==agent + body 字面 grep 5-pattern 之一 → 400 `comment.thinking_subject_required`) |
| CV-5.2 client | `packages/client/src/components/ArtifactComments.tsx` (新) + `wsClient.ts::artifact_comment_added` switch case + `lib/api.ts::postArtifactComment` | comment list view 在 ArtifactPanel 底部 (跟 anchor comments CV-2.3 #404 同视觉栈, 但走独立 endpoint); WS frame 推 artifact_comment_added → 增量 append; agent comment hover anchor `data-cv5-author-link` (跟 CM-5.3 透明协作 hover 同源) |
| CV-5.3 e2e + closure | `packages/e2e/tests/cv-5-artifact-comment.spec.ts` (新) + REG-CV5-001..006 + acceptance template + PROGRESS [x] | 5 case: human comment round-trip / agent comment + thinking subject 必带反断 / cross-channel reject (artifact:X channel member 才能读) / cursor 共序锁 (artifact_comment_added 跟 RT-3 frame 严格递增) / admin god-mode 不消费此 frame (ADM-0 红线) |

## 2. 错误码 byte-identical (跟 DM-2.2 / BPP-2.2 / RT-3 命名同模式)

- `comment.thinking_subject_required` — agent body 字面缺 subject reject (5-pattern 第 4 处链)
- `comment.target_artifact_not_found` — artifact_id 不存在 reject (跟 DM-2.2 mention.target_not_in_channel 同模式)
- `comment.cross_channel_reject` — 非 artifact channel 成员调 endpoint 403 (REG-INV-002 fail-closed)

## 3. 反向 grep 锚 (CV-5 实施 PR 必跑)

```
git grep -nE 'CREATE TABLE.*artifact_comments|artifact_comments.*PRIMARY' packages/server-go/internal/migrations/   # 0 hit (单源)
git grep -nE 'comment.*hub\.cursors\.NextCursor.*[^"]plugin|admin.*comment.*push'  packages/server-go/internal/   # 0 hit (RT-3 共序 + ADM-0 红线)
git grep -nE 'body.*"thinking"$|comment.*defaultSubject|comment.*fallbackSubject|"AI is thinking"|comment.*subject\s*=\s*""' packages/server-go/internal/api/   # 0 hit (5-pattern 第 4 处链)
git grep -nE 'channel_id\s*=\s*"artifact:' packages/server-go/internal/api/  # ≥ 1 hit (namespace 字面承袭)
```

## 4. 不在本轮范围 (deferred)

- ❌ comment edit / delete (CV-5.4+ Phase 6, 立场 ⑤ forward-only audit 跟 admin_actions 同模式)
- ❌ comment reaction (CM-6+, 跟 message reaction 复用)
- ❌ artifact-internal anchor (CV-2 已落 anchor_comments, CV-5 是 artifact-level not anchor-level)
- ❌ admin god-mode 看 artifact comment (ADM-0 §1.3 红线)
