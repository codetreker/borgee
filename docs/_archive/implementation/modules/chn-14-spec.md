# CHN-14 spec brief — channel description edit history audit (战马D v0)

> Phase 6 channel description audit forward-only history — `channels`
> ALTER ADD COLUMN `description_edit_history TEXT NULL` (跟 DM-7 #558
> messages.edit_history 同模式 JSON array `{old_content, ts}`). CHN-10
> #561 既有 PUT 包装写入旧 topic JSON append (UpdateChannel 单源不漂).
> 新 owner-only GET + admin readonly GET history endpoint. Client
> `DescriptionHistoryModal.tsx` (跟 DM-7 EditHistoryModal 同模式).

## §0 立场 (3 + 3 边界)

- **①** 1 列 ALTER ADD COLUMN nullable v=**36** (跟 DM-7.1/AL-7.1/HB-5.1
  + AP-1.1+AP-3.1+AP-2.1 跨七 milestone 同模式). registry.go 字面锁.
  不另起 `channel_description_history` 表 (反向 grep
  `channel_description_history\|channel_history_log\|chn14_history` 0 hit).
- **②** owner-only ACL 锁链第 21 处 (CHN-10 #20 + DM-7 #19 承袭) — handler
  走 `channel.CreatedBy == user.ID` 反向断 member-level reject 403; admin
  god-mode readonly GET 挂 (ADM-0 §1.3 admin 看 audit 不直接改).
- **③** 文案 byte-identical 锁: modal title `编辑历史` 4 字 + empty
  `暂无编辑记录` 6 字 + 行 `{ts}: 修改了说明` (ts RFC3339); 同义词反向
  reject `history/log/audit/记录/日志/审计` 在 user-visible 0 hit.

边界:
- **④** 既有 CHN-10 #561 PUT byte-identical — owner-only ACL + length
  cap 500 + UpdateChannel 单源不变; 包装先 SELECT 旧 `topic +
  description_edit_history` JSON append `{old_content, ts}` → UPDATE.
  CHN-2 #406 既有 PUT /topic path 不动.
- **⑤** AL-1a reason 锁链不漂 (停在 HB-6 #19, 反向 grep `chn14.*reason\|
  description.*reason` 0 hit); description edit audit 走 inline JSON
  列, 不入 admin_actions (跟 DM-7 #558 同精神 立场 ⑤).
- **⑥** AST 锁链延伸第 22 处 forbidden 3 token (`pendingDescriptionAudit
  / descriptionHistoryQueue / deadLetterDescriptionHistory`) 0 hit.

## §1 拆段

**CHN-14.1 schema** v=44: `ALTER TABLE channels ADD COLUMN
description_edit_history TEXT NULL` (跟 DM-7.1 messages.edit_history
ALTER byte-identical). hasColumn 守 idempotent.

**CHN-14.2 server**:
- `store.UpdateChannelDescription(channelID, newDescription, ts)` 包装
  SELECT 旧 topic + edit_history → JSON append → UPDATE (DM-7
  UpdateMessage SSOT 同模式).
- `chn_10_description.go::handlePut` 改调此包装 (代替泛通用
  UpdateChannel; 同 owner-only/length cap 路径 byte-identical).
- 新 GET `/api/v1/channels/{id}/description/history` (owner-only).
- 新 GET `/admin-api/v1/channels/{id}/description/history` (admin readonly).

**CHN-14.3 client**:
- `lib/api.ts::getChannelDescriptionHistory(channelId)` thin wrapper.
- `components/DescriptionHistoryModal.tsx` (modal title `编辑历史` +
  history list RFC3339 + empty `暂无编辑记录`).
- `components/DescriptionEditor.tsx` 加历史按钮 (CHN-10 既有 byte-
  identical 不破).

**CHN-14.4 closure**: REG-CHN14-001..006 6 🟢. AST 锁链延伸第 22 处.

## §2 反约束 grep 锚

- 0 新表: `channel_description_history|channel_history_log|chn14_history` 0 hit.
- v=44 字面锁 (registry.go).
- 既有 chn_10_description.go::handlePut owner-only ACL + length cap 500
  byte-identical (反向 grep `chn_14` 在 handlePut block 0 hit).
- 既有 CHN-2 #406 PUT /topic path 不动 (反向 grep 锚).
- 同义词反向 (user-visible): `history|log|audit|记录|日志|审计` 0 hit
  (我们用 `编辑历史` / `暂无编辑记录` / `修改了说明`).
- AL-1a reason 锁链不漂: `chn14.*reason|description.*reason` 0 hit.
- AST 锁链延伸第 22 处: 3 forbidden token 0 hit.

## §3 不在范围

- 单条 history 删除 / 编辑 (forward-only 立场).
- description 之外的字段 audit (name / topic CHN-2 既有 path 不漂).
- 跨 org admin 全局 history (留 v3 — 仅同 org admin readonly).
- audit retention 自动清理 (留 v3 跟 AL-7 同期).
- diff render 新旧字符串对比 (留 v3 — v0 仅 raw old_content).
