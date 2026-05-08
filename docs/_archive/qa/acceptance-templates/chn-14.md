# Acceptance Template — CHN-14: channel description edit history audit

> 蓝图 channel-model.md §3 audit + CHN-10 #561 description endpoint
> forward-only history. Spec `chn-14-spec.md` (战马D v0). Stance + content-lock.
> schema migration v=44 ALTER ADD description_edit_history nullable
> (跨八 milestone 同模式). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CHN-14.1 — schema v=44 + CHN-14.2 server UpdateChannelDescription SSOT

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 schema migration v=44 ALTER channels ADD COLUMN description_edit_history TEXT NULL (跟 DM-7.1+AL-7.1+HB-5.1+AP-1.1+AP-3.1+AP-2.1 跨七 milestone + 本 milestone 第八 同模式; idempotent + 老 channel 行 byte-identical 不动) | unit (3 sub-case) | 战马D / 烈马 | `internal/migrations/chn_14_1_channels_description_edit_history_test.go::TestCHN141_AddsDescriptionEditHistoryColumn` (PRAGMA nullable) + `_VersionIs44` + `_Idempotent` |
| 1.2 UpdateChannelDescription SSOT — 改 topic 前 SELECT old topic + edit_history → JSON append `{old_content, ts, reason='unknown'}` → UPDATE; AL-1a reason 锁链停在 HB-6 #19 byte-identical | unit (3 sub-case) | 战马D / 烈马 | `_UpdateChannelDescription_AppendsHistory` + `_MultipleEdits_AppendsAll` (改 N 次 → JSON array length=N + ts 单调递增) + `_SameContent_NoAppend` (idempotent — same-content PUT 不入 history) |
| 1.3 GET /api/v1/channels/{channelId}/description/history user-rail — caller = channel.CreatedBy 反向断 (member 403); 历史空时返 `[]`; HappyPath 返 JSON array | unit (4 sub-case) | 战马D / 烈马 | `_GetHistory_HappyPath` + `_NonOwnerRejected` + `_EmptyHistory` + `_Unauthorized` |
| 1.4 admin-rail GET /admin-api/v1/channels/{channelId}/description/history readonly + admin god-mode 不挂 PATCH/DELETE 反向断 | unit + grep | 战马D / 飞马 / 烈马 | `_GetHistoryAdmin_HappyPath` + `_NoAdminPatchDeletePath` (双反向 grep 0 hit) |
| 1.5 既有 chn_10_description.go::handlePut owner-only + length cap 500 byte-identical (production 仅 UpdateChannel → UpdateChannelDescription 包装单字符串改) | grep | 战马D / 飞马 / 烈马 | `_CHN10HandlePutByteIdentical` (反向 grep `chn_14` 在 chn_10_description.go::handlePut block 0 hit + 5 既有锚 must-contain) |

### §2 CHN-14.3 — client DescriptionHistoryModal + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 DescriptionHistoryModal.tsx DOM byte-identical 跟 content-lock §1 (`<div data-testid="description-history-modal">` + title `编辑历史` 4 字 + empty `暂无编辑记录` 6 字 + 时间戳 RFC3339 + 行 `{ts}: 修改了说明`) | vitest (3 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/DescriptionHistoryModal.test.tsx` (title + empty + ts 格式 byte-identical) |
| 2.2 lib/api.ts::getChannelDescriptionHistory 单源 + 空 history (count===0) 渲染 empty 文案 (不 return null — 跟 DM-7 EditHistoryModal 立场略别, 显式空态) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_EmptyHistory_ShowsEmptyState` + `_APIClientSingleSource` |
| 2.3 同义词反向 reject (`history/log/audit/记录/日志/审计` 0 hit user-visible text) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` |

### §3 CHN-14.4 — closure + AST 锁链延伸第 22 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 22 处 forbidden 3 token (`pendingDescriptionAudit / descriptionHistoryQueue / deadLetterDescriptionHistory`) 在 internal/api 0 hit | AST scan | 飞马 / 烈马 | `TestCHN143_NoDescriptionHistoryQueue` (AST scan 0 hit) |

## 边界

- AP-1.1+AP-3.1+AP-2.1+AL-7.1+HB-5.1+CHN-5.1+DM-7.1+CHN-14.1 跨八 milestone
  ALTER ADD nullable 同模式 / CHN-10 #561 既有 PUT /description path
  byte-identical 不动 / DM-7 #558 UpdateMessage SSOT 模式承袭 / ADM-0
  §1.3 admin god-mode 不挂 / owner-only ACL 锁链 21 处一致 / audit
  inline JSON 列模式 (跟 DM-7 #16 同精神) / AST 锁链延伸第 22 处 / 文案
  byte-identical 跟 content-lock + 同义词反向

## 退出条件

- §1 (5) + §2 (3) + §3 (1) 全绿 — 一票否决
- schema migration v=44 ALTER ADD nullable + idempotent
- CHN-10 既有 unit 不破 (handlePut path byte-identical)
- AL-1a reason 锁链停在 HB-6 #19
- AST 锁链延伸第 22 处
- owner-only ACL 锁链 21 处一致
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN14-001..006
