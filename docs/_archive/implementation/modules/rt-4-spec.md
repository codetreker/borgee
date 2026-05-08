# RT-4 spec brief — channel presence indicator (战马D v0)

> Phase 6 channel presence indicator — 谁在 channel 当前 online + typing
> indicator 闭环. Typing 走既有 WS frame `typing` (ws/client.go 既有 path,
> client TypingIndicator.tsx 既有, 5s timeout 自动消失) byte-identical
> 不动. 本 milestone 收尾: GET /api/v1/channels/{channelId}/presence
> 返回 channel 成员里当前 online 的 user_id list (复用 AL-3.1 #277
> presence_sessions 表 + IsOnline 谓词单源) + client PresenceIndicator.tsx
> 头部头像列表展示 + ChannelPresenceList 折叠区. 0 schema 改 / 0 新
> WS frame.

## §0 立场 (3 + 3 边界)

- **①** 0 schema 改 (复用 AL-3.1 #277 presence_sessions 既有表 + idx_
  presence_sessions_user_id). 反向 grep `migrations/rt_4_\d+\|ALTER
  presence_sessions` 在 internal/migrations/ 0 hit (本 milestone 无 schema
  段).
- **②** 0 新 WS frame — typing 走既有 ws/client.go `typing` event
  byte-identical 不动 (5s timeout 既有 RT-2 path); presence-change push
  留 v3 (反向 grep `presence_changed\|presenceChanged\|user_online_pushed`
  在 internal/ws + internal/api 0 hit). 本 milestone 仅同步 GET 拉取.
- **③** member-only ACL (channel.member 既有 IsChannelMember 反向断 —
  非 channel 成员 GET 403); admin god-mode 不挂 PATCH/POST/DELETE 在
  admin-api/v1/.../presence (ADM-0 §1.3 红线 — admin 看不能改 presence).
  AL-1a reason 锁链不漂 — RT-4 read-only 不引入新 reason (反向 grep
  `rt4.*reason\|presence_reason` 0 hit; 锁链停在 HB-6 #19).

边界:
- **④** 既有 typing WS path byte-identical — ws/client.go::handleTyping
  + 既有 multi_device_test.go 不动 (反向 grep `rt_4` 在 ws/client.go
  既有 typing 行 0 hit); RT-4 仅加新 GET endpoint, 不改 hub fan-out.
- **⑤** PresenceTracker.IsOnline 单源 — 已有 presence.IsOnline(userID)
  谓词 (AL-3.2 #310 SessionsTracker 既有), RT-4 reload SQL 走 presence.
  IsOnlineForChannel 新加但内部走 IsOnline + ChannelMembers 拼装 (跟
  既有 SessionsTracker 接口承袭, 不裂出独立 tracker).
- **⑥** AST 锁链延伸第 18 处 forbidden 3 token (`pendingPresenceQuery /
  presenceQueueRetry / deadLetterPresence`) 在 internal/api 0 hit.

## §1 拆段

**RT-4.1 — schema**: 0 行 (复用 presence_sessions).

**RT-4.2 — server**: `internal/api/rt_4_presence.go::RT4PresenceHandler`
GET /api/v1/channels/{channelId}/presence — 取 channel.members ∩
presence.IsOnline → 返回 `{online_user_ids: [...], counted_at: nowMs}`;
member-only ACL (IsChannelMember 反向断); server.go 加
rt4PresenceHandler.RegisterUserRoutes (admin-rail 不挂).

**RT-4.3 — client**: `lib/api.ts::getChannelPresence` thin wrapper +
`components/ChannelPresenceList.tsx` 头部头像列表 (≤5 显示 + 多余 `+N`
overflow + 文案 byte-identical `当前在线 N 人`); 现有 TypingIndicator.tsx
byte-identical 不动 (RT-2 既有 path).

**RT-4.4 — closure**: REG-RT4-001..006 6 🟢.

## §2 反约束 grep 锚

- 0 schema: 反向 grep `migrations/rt_4_\d+\|ALTER presence_sessions` 0 hit.
- 0 新 WS frame: 反向 grep `presence_changed\|presenceChanged\|user_online_pushed`
  0 hit + 既有 typing path byte-identical (反向 grep `rt_4` 在 ws/client.go
  typing block 0 hit).
- member-only ACL: PUT/POST 在 /presence path 0 hit (read-only) + admin-rail
  反向 0 hit.
- 同义词反向 reject (client UI): `presence/online/typing/composing /
  在线状态 / 上线 / 在线人员` 在 ChannelPresenceList user-visible 0 hit
  (我们用 `当前在线 N 人` 字面单源).
- AL-1a reason 锁链不漂: `rt4.*reason\|presence_reason` 0 hit (停在 HB-6 #19).
- AST 锁链延伸第 18 处 forbidden 3 token 0 hit.

## §3 不在范围

- presence-change WS push frame (留 v3 — RT-3.2 fan-out 同期).
- per-device presence (留 v3 — multi-device 算一个 user 已 online).
- typing indicator 改 (既有 RT-2 path byte-identical 不动).
- presence 历史回放 (留 v3 — DM-7 edit history 不延伸).
- admin god-mode presence override (永久不挂 ADM-0 §1.3).
- cross-org presence isolation (留 AP-3 同期).
- last-seen-at 列展示 (留 v3, 当前仅 online 二态).
