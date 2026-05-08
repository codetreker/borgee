# DM-4 spec brief — agent message edit 多端同步 (≤80 行)

> 战马D · Phase 5 · ≤80 行 · 蓝图 [`channels-dm-collab.md`](../../blueprint/channels-dm-collab.md) §3 (DM 编辑) + RT-3 #488 (多端 fan-out + thinking subject 5-pattern). 模块锚 [`dm-collab.md`](dm-collab.md) §DM-4. 依赖 RT-3 #488 fan-out 路径 + DM-3 #508 cursor 复用 RT-1.3 + AL-1 #492 5-state + REFACTOR-REASONS #496 6-dict + ADM-0 §1.3 红线.

## 0. 关键约束 (3 条立场, 蓝图 §3 + RT-3 字面)

1. **DM 编辑同步走 RT-3 既有 fan-out** — agent edit message → server PATCH /api/v1/channels/{dmID}/messages/{id} → 复用 channel.events backfill (跟 DM-3 cursor 同精神, edit 是 cursor 子集). **不另起** message 编辑 channel events stream, **不另起** WS frame, **不另起** sequence — RT-3 fan-out 自动覆盖多端 (BPP-1 #304 envelope reflect lint 自动锁). **反约束**: 反向 grep `dm_edit_event\|message_edit_channel\|edit_sync_frame` 0 hit (跟 DM-3 立场 ② 同精神, 不开 dm-only frame).

2. **edit 是 cursor 子集** — DM-4 编辑事件复用 RT-3 #488 fan-out 已建路径 (server.events 表 INSERT 一行带 op="edit"), client useDMSync (DM-3) 已订阅 channel events backfill, 直接 derive edit 状态. 多端 ≤3s 兜底跟 RT-1.2 #292 latency 同源 (RT-1.3 cursor monotonic 守门). **反约束**: 反向 grep `dm4.*sequence\|edit.*cursor.*= 0\|new.*resume.*dict` 0 hit (跟 DM-3 立场 ① + BPP-5 立场 ② 同精神, 不另起 sequence).

3. **thinking subject 5-pattern 反约束延伸** (RT-3 #488 + DM-3 第 2 处 + DM-4 第 3 处) — 编辑事件 body 反向断言 5-pattern (`processing|responding|thinking|analyzing|planning`) count==0 (改 = 改三处反向 grep). agent edit 不带"思考"语义 (蓝图 §3.2 立场 — edit 是机械修订, 不暴露 agent reasoning). **反约束**: 反向 grep 5-pattern 在 edit body / event payload count==0; 锁链第 3 处 (RT-3 第 1 + DM-3 第 2 + DM-4 第 3).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 DM-3 #508 + BPP-* 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| DM-4.1 server message edit endpoint | `internal/api/dm_4_message_edit.go` 新 (PATCH /api/v1/channels/{dmID}/messages/{id} owner-only ACL + body schema {content, edited_at} + DM-only 路径校验 reject channel.kind != "dm" → 403 `dm.edit_only_in_dm`) + `internal/api/dm_4_message_edit_test.go` 新 (5 unit: HappyPath / NonOwnerRejected / NonDMReject / 401 unauth / events 表 INSERT op="edit") | server PATCH endpoint 复用 messages 表 update_at + events INSERT op="edit", **不**新建表 |
| DM-4.2 client SPA edit hook | `packages/client/src/hooks/useDMEdit.ts` 新 (REST PATCH wrapper + optimistic update; **不**订阅 dm-only frame) + 4 vitest (HappyPath / 错误 toast / pre-edit cursor / multi-device sync 复用 useDMSync) | useDMSync (DM-3 #508) 已派 cursor sync; useDMEdit 仅做 PATCH + optimistic |
| DM-4.3 e2e + REG-DM4 + acceptance + PROGRESS [x] + closure | `packages/e2e/tests/dm-4-edit-multi-device.spec.ts` 新 (REST-driven 双 tab agent edit message → tab2 ≤3s 收 edit 反映) + REG-DM4-001..005 + acceptance/dm-4.md + docs/current sync (server/dm-message.md + client/hooks/useDMEdit.md) | RT-3 fan-out 兜底真测 — multi-device ≤3s |

## 2. 留账边界

- **edit history audit** (留 v2) — DM-4 仅 last-write-wins (messages.updated_at), 不挂编辑历史表. v2 加 `dm_message_edits` audit table (跟 forward-only 同精神).
- **conflict resolution / OT/CRDT** (留 v2 + 跨 milestone) — 多端同时 edit 同 message → last-write-wins 不报警. 真协作编辑留 collaborative editing v2 (跟 CV-2 锚点 immutability 不同维度).
- **edit window 时间限制** (留 v2) — DM-4 不挂"5 分钟内可编辑"业务规则, 蓝图 §3 字面没要求, server 不强制窗口. v2 可加 owner-side preference.
- **edit notification** (留 RT-3.2 follow-up) — edit 不触发 mention notify (跟 DL-4 #490 push gateway 反向, push 仅给 mention/agent_task). reflective 通知留 v2.

## 3. 反查 grep 锚 (Phase 5 验收 + DM-4 实施 PR 必跑)

```
git grep -nE 'PATCH.*/messages/' packages/server-go/internal/api/   # ≥ 1 hit (DM-4.1 endpoint 真挂)
git grep -nE 'op.*=.*"edit"|MessageEditEvent' packages/server-go/internal/api/   # ≥ 1 hit (events 表 op="edit" 写入)
# 反约束 (5 条 0 hit)
git grep -nE 'dm_edit_event|message_edit_channel|edit_sync_frame' packages/server-go/internal/   # 0 hit (复用 RT-3 fan-out, §0.1)
git grep -nE 'dm4.*sequence|edit.*cursor.*= 0|new.*resume.*dict' packages/server-go/   # 0 hit (复用 RT-1.3, §0.2)
git grep -nE '"processing"|"responding"|"thinking"|"analyzing"|"planning"' packages/server-go/internal/api/dm_4*.go   # 0 hit (5-pattern 反约束第 3 处, §0.3)
git grep -nE 'admin.*PATCH.*messages|admin.*DM.*edit' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'dm_message_edits|edit_history|edit_audit_log' packages/server-go/internal/store/   # 0 hit (留 v2, §2 留账)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ edit history audit table (留 v2, §2 留账)
- ❌ OT/CRDT conflict resolution / collaborative editing (跟 CV-2 锚点 immutability 不同维度)
- ❌ edit window 时间限制 (留 v2 owner preference)
- ❌ edit notification → push gateway (跟 DL-4 反向, push 仅 mention/agent_task)
- ❌ thinking subject 暴露在 edit body (5-pattern 反约束第 3 处, agent edit 是机械修订)
- ❌ admin god-mode 走 PATCH messages 路径 (ADM-0 §1.3 红线)
- ❌ DM 之外的编辑 (channel.kind != "dm" reject 403, scope 仅 DM 一对一/agent-DM)
