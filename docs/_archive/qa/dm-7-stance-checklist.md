# DM-7 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 DM-6/DM-5/AL-7/HB-5 stance 同模式)
> **目的**: DM-7 四段实施 (7.1 schema v=34 / 7.2 server / 7.3 client / 7.4 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off.
> **关联**: spec `dm-7-spec.md` + acceptance `acceptance-templates/dm-7.md` + content-lock `dm-7-content-lock.md`
> **content-lock 必锁** — EditHistoryModal title + 共 N 次编辑 + 时间戳 + 同义词反向.

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | schema migration v=34 ALTER messages ADD COLUMN edit_history TEXT NULL — 跟 AP-1.1+AP-3.1+AP-2.1+AL-7.1+HB-5.1+CHN-5.1 跨七 milestone ALTER ADD nullable 同模式 (NULL = 无历史; 老消息行 byte-identical 不动) | AL-7 #533 archived_at 立场承袭 + 跨六 milestone ADD COLUMN nullable 模式 | 反向 grep `migrations/dm_7_\d+\|ALTER messages.*edit_history` 在 v=33 后必有 1 hit (本 migration 单源) |
| ② | UpdateMessage SSOT — 改 content 前 SELECT old content 写入 edit_history JSON array; AL-1a `reasons.Unknown='unknown'` byte-identical (锁链第 18 处, AL-7 #15+AL-8 #16+HB-5 #17 承袭). UpdateMessage 路径单源 — DM-4 #553 调用方 byte-identical 不动 | AL-7 SweeperReason + HB-5 HeartbeatSweeperReason reason 锁链承袭 | 反向 grep `inline.*UPDATE.*messages.*content` 0 hit (DM-4 既有 path 不漂); UpdateMessage 内部 SELECT old → JSON append → UPDATE 单源 |
| ③ | owner-only ACL 锁链第 19 处 — user-rail GET sender = current user (别 user 403); admin readonly admin-rail GET (admin god-mode 不挂 PATCH/DELETE — ADM-0 §1.3 红线) | admin-model.md ADM-0 §1.3 + DM-6 #18 owner-only 立场承袭 | 反向 grep `admin.*edit_history.*PATCH\|admin.*edit_history.*DELETE` 0 hit + sender ≠ current user → 403 反向断言 unit |

边界:
- **④** PATCH 路径 byte-identical 不变 — DM-4 #553 既有 dm_4_message_edit.go production 0 行变更 (反向断言 git diff 仅命中 store + dm_7_*.go + client + docs); UpdateMessage 内部 SELECT/append/UPDATE 单源, 调用方 unchanged.
- **⑤** 文案 byte-identical 跟 content-lock §1 — EditHistoryModal title `编辑历史` 4 字 + count `共 N 次编辑` 5 字 + 时间戳 RFC3339 + diff view body; 同义词反向 reject (`history/changes/revisions/版本/修订/变更`).
- **⑥** AST 锁链延伸第 16 处 forbidden 3 token (`pendingEditHistory / editHistoryQueue / deadLetterEditHistory`) 在 internal/api 0 hit.

## §1 立场 ① schema v=34 ALTER ADD nullable (DM-7.1 守)

- [ ] migration v=34 ALTER messages ADD COLUMN edit_history TEXT NULL
- [ ] idempotent guard (hasColumn check 跟 AL-7.1 同模式)
- [ ] registry.go 加 dm71MessagesEditHistory 字面锁
- [ ] 老消息行 byte-identical 保留 (反向断言 NULL 不影响 GET / PATCH)
- [ ] 跨七 milestone ALTER ADD nullable 同模式 byte-identical 跟 AL-7.1

## §2 立场 ② UpdateMessage SSOT (DM-7.2 守)

- [ ] UpdateMessage 内部 SELECT old content 单源 (反向 grep inline UPDATE 0 hit)
- [ ] edit_history JSON array append `{old_content, ts, reason='unknown'}` byte-identical
- [ ] AL-1a reason 锁链第 18 处 — reasons.Unknown const 字面 byte-identical 跟 AL-7 SweeperReason / HB-5 HeartbeatSweeperReason 同源
- [ ] DM-4 #553 dm_4_message_edit.go production 0 行变更 (UpdateMessage 调用方 byte-identical)
- [ ] history append idempotent — 重复 PATCH 同 content 不重复入 history

## §3 立场 ③ owner-only sender + admin readonly (DM-7.2 守)

- [ ] GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history user-rail
- [ ] sender ≠ current user → 403 反向断言
- [ ] admin-rail GET /admin-api/v1/messages/{messageId}/edit-history readonly
- [ ] admin god-mode 不挂 PATCH/DELETE 反向断言 (双反向 grep 0 hit)
- [ ] owner-only ACL 锁链第 19 处一致 (DM-6 #18 承袭)

## §4 蓝图边界 ④⑤⑥ — 不漂

- [ ] DM-4 #553 既有 PATCH path byte-identical (production 0 行变更)
- [ ] EditHistoryModal title `编辑历史` 4 字 byte-identical
- [ ] count `共 N 次编辑` 5 字 byte-identical (N 计数动态)
- [ ] 时间戳 RFC3339 byte-identical (跟 CHN-1.2 archive system DM 同模式)
- [ ] 同义词反向 (`history/changes/revisions/版本/修订/变更`) 0 hit user-visible
- [ ] AST 锁链延伸第 16 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (5) + §2 (5) + §3 (5) + §4 (6) 全 ✅
- 反向 grep 5 项全 0 hit
- audit 5 字段链 DM-7 = 第 16 处
- AL-1a reason 锁链第 18 处一致
- AST 锁链延伸第 16 处
- owner-only ACL 锁链第 19 处一致
- 跨七 milestone ALTER ADD nullable byte-identical
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-DM7-001..006
