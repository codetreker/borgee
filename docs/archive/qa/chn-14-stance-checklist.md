# CHN-14 stance checklist (战马D v0)

战马D · 2026-04-30 · 立场守门 (3+3 边界). 跟 DM-7 #558 stance 同模式.
**关联**: spec `chn-14-spec.md` + acceptance + content-lock.

## §0 立场 3 项

- [x] **① schema migration v=44 ALTER channels ADD COLUMN
  description_edit_history TEXT NULL** — 跟 DM-7.1+AL-7.1+HB-5.1 +
  AP-1.1+AP-3.1+AP-2.1 跨七 milestone ALTER ADD nullable 同模式 (NULL
  = 无历史; 老 channel 行 byte-identical 不动). 反向 grep
  `migrations/chn_14_\d+\|ALTER channels.*description_edit_history` 必有 1 hit.
  不另起 `channel_description_history` 表.
- [x] **② UpdateChannelDescription SSOT** — 改 topic 前 SELECT old topic
  + edit_history → JSON append `{old_content, ts, reason='unknown'}` →
  UPDATE; AL-1a `reasons.Unknown='unknown'` byte-identical (锁链停在 HB-6
  #19, CHN-14 不引入新 reason). chn_10_description.go::handlePut 改调
  此包装代替泛通用 UpdateChannel; 既有 owner-only ACL + length cap 500
  路径 byte-identical.
- [x] **③ owner-only ACL 锁链第 21 处** (CHN-10 #20 + DM-7 #19 承袭) —
  user-rail GET sender = channel.CreatedBy (member 403); admin-rail
  readonly GET (admin god-mode 不挂 PATCH/DELETE — ADM-0 §1.3 红线).

## §0.边界 3 项

- [x] **④ 既有 PUT /channels/:id/description path byte-identical** —
  CHN-10 #561 chn_10_description.go::handlePut owner-only + length cap
  500 + UpdateChannel 单源不变 (反向 grep `chn_14` 在 handlePut block
  0 hit; 仅 UpdateChannel 调用 → UpdateChannelDescription 包装单字符串
  改). CHN-2 #406 既有 member-level PUT /topic path 不动.
- [x] **⑤ AL-1a reason 锁链不漂** (停在 HB-6 #19, 反向 grep
  `chn14.*reason\|description.*reason` 0 hit); description audit 走
  inline JSON 列, 不入 admin_actions (跟 DM-7 #558 立场 ⑤ 同精神).
- [x] **⑥ AST 锁链延伸第 22 处** — forbidden 3 token
  (`pendingDescriptionAudit / descriptionHistoryQueue /
  deadLetterDescriptionHistory`) 0 hit.

## §1 立场 ① schema v=44 ALTER ADD nullable (CHN-14.1 守)

- [x] migration v=44 ALTER channels ADD COLUMN description_edit_history TEXT NULL
- [x] idempotent guard (hasColumn check 跟 DM-7.1 / AL-7.1 同模式)
- [x] registry.go 加 chn141ChannelsDescriptionEditHistory 字面锁
- [x] 老 channel 行 byte-identical 保留 (NULL 不影响 GET / PUT)
- [x] 跨七 milestone ALTER ADD nullable 同模式 byte-identical 跟 DM-7.1

## §2 立场 ② UpdateChannelDescription SSOT (CHN-14.2 守)

- [x] UpdateChannelDescription 内部 SELECT old topic + edit_history 单源
  (反向 grep inline `UPDATE channels.*topic` 在 chn_10/chn_14 之外 0 hit)
- [x] edit_history JSON array append `{old_content, ts, reason='unknown'}`
- [x] AL-1a reason 锁链停在 HB-6 #19 (reasons.Unknown 字面 byte-identical)
- [x] CHN-10 #561 chn_10_description.go::handlePut 调用方 byte-identical
  (仅 UpdateChannel → UpdateChannelDescription 包装替换, owner-only +
  length cap 500 路径不变)
- [x] history append idempotent — same-content PUT 不重复入 history (跟
  DM-7 #558 idempotent 同精神)

## §3 立场 ③ owner-only sender + admin readonly (CHN-14.2 守)

- [x] GET /api/v1/channels/{channelId}/description/history user-rail
- [x] caller ≠ channel.CreatedBy → 403 反向断 (member-level reject)
- [x] admin-rail GET /admin-api/v1/channels/{channelId}/description/history readonly
- [x] admin god-mode 不挂 PATCH/DELETE 反向断 (双反向 grep 0 hit)
- [x] owner-only ACL 锁链第 21 处一致 (CHN-10 #20 承袭)

## §4 蓝图边界 ④⑤⑥ — 不漂

- [x] CHN-10 #561 既有 chn_10_description.go::handlePut owner-only +
  length cap 500 byte-identical (production 仅 UpdateChannel 调用单字符串改)
- [x] CHN-2 #406 既有 PUT /topic path 不动
- [x] DescriptionHistoryModal title `编辑历史` 4 字 byte-identical
- [x] empty `暂无编辑记录` 6 字 byte-identical
- [x] history 行 `{ts}: 修改了说明` (RFC3339) byte-identical
- [x] 同义词反向 (`history|log|audit|记录|日志|审计`) 0 hit user-visible
- [x] AST 锁链延伸第 22 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (5) + §2 (5) + §3 (5) + §4 (7) 全 ✅
- 反向 grep 6 项全 0 hit
- audit 5 字段链 CHN-14 = 第 17 处 (DM-7 #16 后顺位)
- AL-1a reason 锁链停在 HB-6 #19 (CHN-14 不动)
- AST 锁链延伸第 22 处
- owner-only ACL 锁链第 21 处一致
- 跨七 milestone ALTER ADD nullable byte-identical
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN14-001..006
