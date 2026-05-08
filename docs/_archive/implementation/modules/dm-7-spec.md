# DM-7 spec brief — DM message edit history audit (战马D v0)

> Phase 6 DM message edit history — DM-4 #553 既有 PATCH edit endpoint
> 当前 overwrites content 字面 (旧 content 丢失). 给 message edit 加历史
> 留痕回放: edit_history JSON 列追加旧 content + ts + reason. 跟 AL-7
> #536 archived_at + AP-1.1/AP-3.1/AP-2.1/HB-5.1/CHN-5.1 ALTER ADD COLUMN
> nullable **跨七 milestone** 同模式. owner-only sender + admin readonly
> (跟 AL-8 query API 同精神).

## §0 立场 (3 + 3 边界)

- **①** schema migration v=34 ALTER messages ADD COLUMN edit_history TEXT
  NULL (跟 AP-1.1+AP-3.1+AP-2.1+AL-7.1+HB-5.1+CHN-5.1 跨七 milestone
  ALTER ADD nullable 同模式; NULL = 无历史 / 现网行为零变 / 老消息行
  byte-identical 不动). 反向 grep `migrations/dm_7_\d+|ALTER messages.*
  edit_history` 在 hb_5_1 后必有 1 hit (本 migration 单源).
- **②** UpdateMessage SSOT 路径加 history append — 在改 content 前 SELECT
  old content 写入 edit_history JSON array (`[{old_content, ts, reason}]`,
  reason 复用 AL-1a `reasons.Unknown='unknown'` byte-identical 跟 AL-7
  SweeperReason / HB-5 HeartbeatSweeperReason 同源, AL-1a reason 锁链
  第 18 处 — HB-5 #17 + AL-8 #16 + AL-7 #15 承袭). UpdateMessage 路径
  单源 — 反向 grep `Update.*content.*edited_at` 0 行 inline UPDATE byte-
  identical 单源 (DM-4 既有 path 不漂).
- **③** owner-only ACL 锁链第 19 处 — GET /api/v1/channels/{channelId}/
  messages/{messageId}/edit-history user-rail (sender = current user
  反向断言, 别 user 调 → 403); admin readonly admin-rail GET
  /admin-api/v1/messages/{messageId}/edit-history (admin god-mode 不挂
  PATCH/DELETE — ADM-0 §1.3 红线 admin 看不能改; 反向 grep `admin.*
  edit_history.*PATCH\|admin.*edit_history.*DELETE` 0 hit).

边界:
- **④** PATCH 路径 byte-identical 不变 — DM-4 #553 既有 dm_4_message_edit.go
  反向断言 line count + signature byte-identical (production 0 行变更);
  history append 走 store layer wrap UpdateMessage 单源 (UpdateMessage
  内部 SELECT old content 后写, 调用方 unchanged).
- **⑤** 文案 byte-identical 跟 content-lock §1 — EditHistoryModal `编辑
  历史` 4 字 title + `共 N 次编辑` 5 字 + 时间戳 RFC3339 + body diff
  view; 同义词反向 reject (`history/changes/revisions/版本/修订/变更`).
- **⑥** AST 锁链延伸第 16 处 forbidden 3 token (`pendingEditHistory /
  editHistoryQueue / deadLetterEditHistory`) 在 internal/api 0 hit.

## §1 拆段

**DM-7.1 — schema migration v=34**: ALTER messages ADD COLUMN edit_history
TEXT NULL (跟 AL-7.1 archived_at 跨七 milestone 同模式; idempotent
guard).

**DM-7.2 — server**:
- `internal/store/queries.go::UpdateMessage` wrap — 改 content 前 SELECT
  old content + 写入 edit_history JSON array; AL-1a reason 锁链第 18 处.
- `internal/api/dm_7_edit_history.go` GET endpoint owner-only sender +
  admin readonly admin-rail. user-rail 路径 GET 走既有 channel.member ACL
  + sender == current user 反向断言.

**DM-7.3 — client**: `lib/api.ts::getEditHistory` 单源 +
`components/EditHistoryModal.tsx` 文案 byte-identical 跟 content-lock.
vitest 5 case (`编辑历史` title + 共 N 次编辑 + 空 history null +
时间戳 RFC3339 + 同义词反向).

**DM-7.4 — closure**: REG-DM7-001..006 6 🟢 + AST scan + audit 5 字段链
第 16 处.

## §2 反约束 grep 锚

- ALTER ADD COLUMN nullable 跨七 milestone (AP-1.1+AP-3.1+AP-2.1+AL-7.1+
  HB-5.1+CHN-5.1+DM-7.1).
- admin god-mode 不挂 PATCH/DELETE: `admin.*edit_history.*PATCH\|
  admin.*edit_history.*DELETE` 0 hit.
- UpdateMessage SSOT: 反向 grep `inline.*UPDATE.*messages.*content` 0 hit
  (DM-4 既有 path 不漂).
- AL-1a reason 锁链第 18 处 — `runtime_recovered\|dm7_specific_reason` 0 hit.
- AST 锁链延伸第 16 处 forbidden 3 token 0 hit.

## §3 不在范围

- edit history 全文搜 (留 v3, content search 跟 CV-6 FTS 同期).
- edit history retention sweeper (留 v3, 跟 AL-7 retention 同精神).
- admin god-mode edit override (永久不挂 ADM-0 §1.3).
- diff syntax highlight (留 v3 client only).
- delete history (永久不挂 — forward-only 跟 AL-1 同精神).
