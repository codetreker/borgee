# RT-4 stance checklist (战马D v0)

战马D · 2026-04-30 · 立场守门 (3+3 边界).

## §0 立场 3 项

- [x] **① 0 schema 改** — 复用 AL-3.1 #277 presence_sessions 既有表; 反向
  grep `migrations/rt_4_\d+\|ALTER presence_sessions` 0 hit.
- [x] **② 0 新 WS frame** — typing 走既有 RT-2 path; presence-change push
  留 v3; 反向 grep `presence_changed\|presenceChanged\|user_online_pushed`
  0 hit; 既有 ws/client.go::handleTyping byte-identical 不动.
- [x] **③ member-only ACL** — IsChannelMember 反向断 (非成员 403);
  admin god-mode 不挂 PATCH/POST/DELETE 在 admin-api/v1/.../presence
  (ADM-0 §1.3 红线 — admin 看不能改 presence).

## §0.边界

- [x] **④** 既有 typing path byte-identical (反向 grep rt_4 在
  ws/client.go::handleTyping block 0 hit).
- [x] **⑤** PresenceTracker.IsOnline 单源 (AL-3.2 SessionsTracker, RT-4
  不裂出独立 tracker).
- [x] **⑥** AST 锁链延伸第 18 处 forbidden 3 token 0 hit
  (`pendingPresenceQuery / presenceQueueRetry / deadLetterPresence`).

## §1 测试

- [x] REG-RT4-001 0 schema (`TestRT41_NoSchemaChange`).
- [x] REG-RT4-002 GET /presence HappyPath member 200 + online_user_ids
  长度 + non-member 403 + 401.
- [x] REG-RT4-003 既有 typing path byte-identical 不变 (`TestRT41_
  TypingPathByteIdentical` 反向 grep rt_4 在 ws/client.go::handleTyping
  block 0 hit).
- [x] REG-RT4-004 admin-rail 不挂 (`TestRT43_NoAdminPresencePath`).
- [x] REG-RT4-005 AST 锁链延伸第 18 处 (`TestRT43_NoPresenceQueue`).
- [x] REG-RT4-006 client ChannelPresenceList 文案 byte-identical
  (`当前在线 N 人` + `+N` overflow + 同义词反向 reject) + 5 vitest.

## §2 反约束 grep 锚

- 0 schema: `migrations/rt_4_\d+|ALTER presence_sessions` 0 hit.
- 0 新 WS frame: `presence_changed|presenceChanged|user_online_pushed` 0 hit.
- 既有 typing byte-identical: `rt_4` 在 ws/client.go::handleTyping block 0 hit.
- 同义词反向: `presence|typing|composing|在线状态|上线|在线人员` 在
  ChannelPresenceList user-visible 0 hit.
- AST 锁链延伸第 18 处: 3 forbidden token 0 hit.
