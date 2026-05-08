# DM-6 spec brief — DM thread reply UI (战马D v0)

> Phase 6 DM thread reply — 用户在 DM 频道中对单条消息发起 thread reply,
> 跟既有 channel message reply 同模式. **0 schema 改 / 0 server production
> code** — messages.reply_to_id 列由 existing migration 已落 + POST
> /api/v1/channels/{channelId}/messages 既有接受 reply_to_id (反向断言).
> 跟 DM-5 #549 / CV-7 #535 / CV-9..12 一系列 0-server 模式承袭.

## §0 立场 (3 + 3 边界)

- **①** **0 schema 改 / 0 server production code** — messages.reply_to_id
  既有列 + 既有 POST /channels/{id}/messages reply_to_id 入参 byte-identical
  不动. 反向 grep `migrations/dm_6_\d+|ALTER TABLE messages.*reply\|
  reply.*new.*endpoint` 在 internal/migrations+api/ 0 hit.
  生产 server git diff 仅含新 _test.go 反断 + docs (反向断言 + 留账锚).
- **②** owner-only ACL 锁链第 18 处 (CHN-9 #17 承袭) — DM thread reply
  走既有 message ACL (channel.member 必传, 跟 DM-3 #508 既有 path 同源
  byte-identical 不动); admin god-mode 不挂 PATCH/POST DM thread (反向
  grep `admin.*dm.*thread\|admin.*reply` 在 admin*.go 0 hit).
- **③** thinking 5-pattern 锁链第 9 处 (DM-5 第 8 处承袭) — DM thread
  reply 不豁免 thinking 反向断言: agent 写 reply body 仍走既有 thinking
  validation (反向 grep `<thinking>\|<thought>\|<reasoning>\|<reflection>\|
  <internal>` 在 dm_6 production *.go 0 hit, 跟 DM-3/RT-3/DM-4/DM-5 反约束
  延伸).

边界:
- **④** thread depth 1 层强制 — reply_to_id 指向的 parent message 不能
  本身有 reply_to_id (反向断言: `parent.reply_to_id != null` → reject
  400 `dm.thread_depth_exceeded` byte-identical 文案锁). 此 validation 走
  既有 server CreateMessage 路径 — 现网行为零变 (反向 grep 既有 server 代码
  0 行变更).
- **⑤** 文案 byte-identical 跟 content-lock §1 — DMThread 折叠 toggle
  `▼ 隐藏 N 条回复` (展开态) / `▶ 显示 N 条回复` (折叠态) byte-identical
  + reply input placeholder `回复...` 2 字; data-testid `dm6-thread-toggle`
  锚 + 同义词反向 reject (`reply/comment/discussion/讨论/评论/评论区`).
- **⑥** AST 锁链延伸第 15 处 forbidden 3 token (`pendingDMThread /
  dmThreadQueue / deadLetterDMThread`) 在 internal/api 0 hit.

## §1 拆段

**DM-6.1 — server 0 production code**:
- 0 新 production *.go 文件; 既有 messages.go 反向断言 不动.
- `internal/api/dm_6_thread_test.go` (新 _test.go ONLY) — 4 unit:
  - `TestDM61_NoServerCodeAdded` (filepath.Walk internal/api 反向 grep
    `dm_6` 在 production *.go 0 hit, 仅 _test.go).
  - `TestDM61_ReplyToIDColumnExists` (PRAGMA messages 反向断言 reply_to_id
    列 existing).
  - `TestDM61_DMThreadReply_HappyPath` (POST DM channel message with
    reply_to_id → 200 + persisted, 走既有 path).
  - `TestDM63_NoDMThreadQueue` (AST scan forbidden tokens).

**DM-6.2 — client**: `lib/api.ts` 不动 (既有 sendMessage already accepts
reply_to_id). `components/DMThread.tsx` 折叠 thread + reply input + DOM
byte-identical 跟 content-lock. vitest 5 case (toggle 折叠 / reply
submit / 文案锁 / 同义词反向 / 空 thread 不渲染).

**DM-6.3 — closure**: REG-DM6-001..006 6 🟢 + AST scan 反向 + 5-pattern
锁链延伸第 9 处.

## §2 反约束 grep 锚

- 0 server prod: `dm_6_*.go production` 在 internal/api/ 0 hit (仅 _test.go).
- 0 schema: `migrations/dm_6_\d+|ALTER TABLE messages.*reply` 0 hit.
- admin 不挂: `admin.*dm.*thread\|admin.*reply` 在 admin*.go 0 hit.
- thinking 5-pattern: `<thinking>\|<thought>\|<reasoning>\|<reflection>\|
  <internal>` 在 dm_6 production *.go 0 hit (锁链第 9 处).
- AST 锁链延伸第 15 处 forbidden 3 token 0 hit.

## §3 不在范围

- thread depth >1 (永久不挂 — 1 层强制保留 UX 简单).
- thread reply push notification (RT-3.2 follow-up, 留 v3).
- admin god-mode thread override (永久不挂 ADM-0 §1.3).
- thread reply audit (audit 5 字段链复用既有 messages 表, 不另起).
- cross-channel thread (永久不挂 — thread 局限单 channel).
