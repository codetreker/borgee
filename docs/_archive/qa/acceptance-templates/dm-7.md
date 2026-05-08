# Acceptance Template — DM-7: DM message edit history audit

> 蓝图 dm-model.md §3 audit + DM-4 #553 edit endpoint forward-only history. Spec `dm-7-spec.md` (战马D v0). Stance + content-lock. schema migration v=34 ALTER ADD edit_history nullable (跨七 milestone 同模式). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 DM-7.1 — schema v=34 + DM-7.2 server UpdateMessage SSOT

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 schema migration v=34 ALTER messages ADD COLUMN edit_history TEXT NULL (跟 AP-1.1+AP-3.1+AP-2.1+AL-7.1+HB-5.1+CHN-5.1 跨七 milestone 同模式; idempotent + 老消息行 byte-identical 不动) | unit (3 sub-case) | 战马D / 烈马 | `internal/migrations/dm_7_1_messages_edit_history_test.go::TestDM71_AddsEditHistoryColumn` (PRAGMA nullable) + `_VersionIs34` + `_Idempotent` |
| 1.2 UpdateMessage SSOT — 改 content 前 SELECT old content 写入 edit_history JSON array `[{old_content, ts, reason='unknown'}]`; AL-1a reason 锁链第 18 处 (AL-7 SweeperReason + HB-5 HeartbeatSweeperReason 承袭) | unit (3 sub-case) | 战马D / 烈马 | `_UpdateMessage_AppendsEditHistory` (改 content → edit_history 含 old) + `_UpdateMessage_MultipleEdits_AppendsAll` (改 N 次 → JSON array length=N + ts 单调递增) + `_UpdateMessage_ReasonByteIdentical` (reason='unknown' byte-identical) |
| 1.3 GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history user-rail — sender = current user 反向断言 (别 user → 403); 历史空时返 `[]`; HappyPath 返 JSON array | unit (4 sub-case) | 战马D / 烈马 | `_GetEditHistory_HappyPath` (sender 调 → 200 + array) + `_GetEditHistory_NonSenderRejected` (别 user → 403) + `_GetEditHistory_EmptyHistory` (未编辑 → 200 + []) + `_GetEditHistory_Unauthorized` (401) |
| 1.4 admin-rail GET /admin-api/v1/messages/{messageId}/edit-history readonly + admin god-mode 不挂 PATCH/DELETE 反向断言 (双反向 grep 0 hit) — admin god-mode ADM-0 §1.3 红线 | unit + grep | 战马D / 飞马 / 烈马 | `_GetEditHistoryAdmin_HappyPath` + `_NoAdminPatchDeletePath` (双反向 grep 0 hit) |
| 1.5 DM-4 #553 既有 dm_4_message_edit.go production 0 行变更 反向断言 (git diff 仅命中 store + dm_7_*.go + client + docs) | grep | 战马D / 飞马 / 烈马 | `_DM4ProductionByteIdentical` (反向 grep dm_7 在 dm_4*.go 0 hit + UpdateMessage 调用方 byte-identical) |

### §2 DM-7.3 — client EditHistoryModal + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 EditHistoryModal.tsx DOM byte-identical 跟 content-lock §1 (`<div data-testid="edit-history-modal">` + title `编辑历史` 4 字 + count `共 N 次编辑` 5 字 + 时间戳 RFC3339 + body diff view) | vitest (3 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/EditHistoryModal.test.tsx` (title 文案 + count 文案 + 时间戳格式 byte-identical) |
| 2.2 空 history (count===0) 不渲染 modal (return null) + lib/api.ts::getEditHistory 单源 | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_EmptyHistory_NoModal` + `_APIClientSingleSource` |
| 2.3 同义词反向 reject (`history/changes/revisions/版本/修订/变更` 0 hit user-visible text) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` |

### §3 DM-7.4 — closure + AST 锁链延伸第 16 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 16 处 forbidden 3 token (`pendingEditHistory / editHistoryQueue / deadLetterEditHistory`) 在 internal/api 0 hit | AST scan | 飞马 / 烈马 | `TestDM73_NoEditHistoryQueue` (AST scan 0 hit) |

## 边界

- AP-1.1+AP-3.1+AP-2.1+AL-7.1+HB-5.1+CHN-5.1+DM-7.1 跨七 milestone ALTER ADD nullable 同模式 / DM-4 #553 既有 PATCH path byte-identical 不动 / AL-7 SweeperReason + HB-5 HeartbeatSweeperReason reason 锁链承袭 (AL-1a 第 18 处) / ADM-0 §1.3 admin god-mode 不挂 / owner-only ACL 锁链 19 处一致 / audit 5 字段链第 16 处 / AST 锁链延伸第 16 处 / 文案 byte-identical 跟 content-lock + 同义词反向

## 退出条件

- §1 (5) + §2 (3) + §3 (1) 全绿 — 一票否决
- schema migration v=34 ALTER ADD nullable + idempotent
- DM-4 既有 unit 不破 (production byte-identical)
- audit 5 字段链 DM-7 = 第 16 处
- AL-1a reason 锁链第 18 处一致
- AST 锁链延伸第 16 处
- owner-only ACL 锁链 19 处一致
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-DM7-001..006
