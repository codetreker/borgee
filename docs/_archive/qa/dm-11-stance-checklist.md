# DM-11 stance checklist — DM cross-DM message search

> 5 立场 byte-identical 跟 spec §0 (≤80 行).

## 1. 0 schema 改 (跟 CHN-13 / CV-12 既有同模式)

- [x] 复用 messages.content + LIKE %query% (跟 messages.go::handleSearchMessages #467 既有同模式)
- [x] FTS5 不走 — CV-6 #531 落 artifacts_fts 表但 DM message search 不复用 (跨表 join 复杂度留 v2)
- [x] 反向断: `git diff origin/main -- packages/server-go/internal/migrations/` 0 production 行 (0 schema 改)
- [x] 不另起 dm_search_index 表 (反向 grep `dm_search_index|dm_search_table|dm_11_search_log` 0 hit)

## 2. DM-only scope (跟 DM-10 + dm_4_message_edit.go 同精神)

- [x] store helper `SearchDMMessages` JOIN channels ON c.type='dm' 强制过滤
- [x] 反向枚举锁: 公开 channel 同 query 消息 NOT 出现在 DM 搜索结果 (`TestDM11_Search_DMOnly_ExcludesPublicChannel` PASS)
- [x] 跟 DM-10 #597 + dm_4_message_edit.go #549 DM-only path 同精神
- [x] 反向枚举: 公开/private/general channel pin/edit/search/list 全独立路径不跨

## 3. channel-member ACL (复用 AP-4 + AP-5 同模式)

- [x] store helper JOIN channel_members ON cm.user_id = caller (反 cross-user DM leak)
- [x] AP-4 #551 reactions ACL + AP-5 #555 messages ACL 立场承袭 — 第三方 user 搜同 query → 0 results
- [x] 反向枚举锁: 第三方 user (非 DM 成员) 搜 query → 0 results (`TestDM11_Search_NonMember_NoLeak` PASS)
- [x] 反 leak 路径: deleted message 不出现 (maskDeletedMessages helper 守, `TestDM11_Search_DeletedMessageHidden` PASS)

## 4. q 反 DoS + limit clamp

- [x] q trim + min 2 char (反 1 char query 全表扫) + max 200 char (反 DoS)
- [x] q 缺失 → 400 `dm_search.q_required` / 太短 → 400 `dm_search.q_too_short` / 太长 → 400 `dm_search.q_too_long` 字面锁
- [x] limit clamp default 30 / max 50 (反 cursor 滥用; 跟 messages.go::handleSearchMessages #467 既有同模式)
- [x] 4 反约束 unit PASS (QRequired + QTooShort + QTooLong + LimitClamp)

## 5. admin god-mode 不挂 (ADM-0 §1.3 红线)

- [x] 反向 grep `admin.*dm.*search|/admin-api/.*dm/search` 在 admin*.go 0 hit
- [x] cross-user DM search 永久不挂 admin (跟 DM-10 #597 + DM-7 edit history admin god-mode 红线锁链承袭)
- [x] PR #571 admin-godmode 总表 §2 模式延伸 — admin 不入 DM 业务路径

## 反约束

- ❌ FTS5 跨表 join (留 v2 复杂度过高)
- ❌ sort by relevance (留 v2)
- ❌ admin god-mode cross-user search (永久不挂)
- ❌ 跨 org search (复用 store.CrossOrg 既有, 留 AP-3 同期)
- ❌ search history persistence (留 v3)
- ❌ per-DM channel filter (现版跨所有 user's DM)
- ❌ client UI (DM search bar 留 follow-up)

## 跨 milestone byte-identical 锁链

- CV-12 #545 + CHN-13 #583 既有 search endpoint LIKE 模式 (per-channel)
- DM-10 #597 DM-only path (channel.Type='dm' filter 同精神)
- AP-4 #551 reactions ACL + AP-5 #555 messages ACL (channel-member helper 复用)
- dm_4_message_edit.go #549 DM-only scope
- ADM-0 §1.3 admin god-mode 红线
- PR #571 admin-godmode 总表 §2
- maskDeletedMessages helper (跟 SearchMessages #467 既有共享 — 反 deleted leak)
