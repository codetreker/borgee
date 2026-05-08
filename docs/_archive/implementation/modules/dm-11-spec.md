# DM-11 spec brief — DM cross-DM message search (≤80 行)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`dm-model.md`](../../blueprint/dm-model.md) §3 future per-user search index v2 (本 PR v0 = LIKE %query% 跨 user DM channels). DM-11 给 user 加跨 DM channel 消息搜索 — 复用 messages.content 列 + LIKE; **0 schema 改** (跟 CHN-13 channel search / CV-12 既有 message search 同模式). 跟 CHN-13 #583 / DM-9 #585 / DM-10 #597 同节奏 (一 milestone 一 PR + 4件套).

## 0. 关键约束 (5 项立场)

1. **0 schema 改** — 复用 messages.content + LIKE %query% (跟 messages.go::handleSearchMessages #467 既有同模式). FTS5 已在 CV-6 #531 落 artifacts_fts 表但**不复用** — DM message search 不走 FTS5 避免跨表 join 复杂度, 留 v2. 反向断: `git diff origin/main -- packages/server-go/internal/migrations/` 0 production 行.

2. **DM-only scope** — store helper `SearchDMMessages` JOIN channels ON c.type='dm' 强制过滤; 反 cross-channel leak (跟 DM-10 #597 + dm_4_message_edit.go #549 DM-only path 同精神). 反向枚举: 公开 channel 同 query 消息 NOT 出现在 DM 搜索结果 (反向 leak 锁).

3. **channel-member ACL 复用 AP-4 + AP-5 模式** — store helper JOIN channel_members ON cm.user_id = caller (反 cross-user DM leak). 跟 AP-4 #551 reactions ACL + AP-5 #555 messages ACL 立场承袭 — 第三方 user 搜同 query → 0 results (无 DM 成员资格).

4. **q query param 反 DoS** — q trim + min 2 char + max 200 char; q 缺失 → 400 `dm_search.q_required` / 太短 → 400 `dm_search.q_too_short` / 太长 → 400 `dm_search.q_too_long`; limit clamp default 30 / max 50 (反 cursor 滥用).

5. **admin god-mode 不挂** — 反向 grep `admin.*dm.*search\|/admin-api/.*dm/search` 在 admin*.go 0 hit (ADM-0 §1.3 红线). cross-user DM search 永久不挂 admin (跟 DM-10 #597 + DM-7 edit history admin god-mode 红线锁链承袭).

## 1. 拆段实施 (单 PR 全闭)

| 段 | 文件 | 范围 |
|---|---|---|
| DM-11.1 store | `internal/store/dm_11_search_queries.go` (新, 1 helper) | SearchDMMessages — JOIN messages × users × channels (type='dm') × channel_members (cm.user_id = caller) + LIKE %query% + ORDER BY created_at DESC + clamp limit 50 + maskDeletedMessages (反 deleted leak) |
| DM-11.2 server | `internal/api/dm_11_search.go` (新, 1 endpoint) + server.go register | GET /api/v1/dm/search?q=&limit= + auth + q validation 3 字面错码 + limit clamp + 10 unit (happy/q-required/q-too-short/q-too-long/unauthorized/no-match/dm-only-excludes-public/non-member-no-leak/limit-clamp/deleted-hidden) |
| DM-11.3 closure | REG-DM11-001..006 + acceptance + content-lock + PROGRESS [x] | 5 立场 byte-identical 锁 + 反向 grep 4 锚 |

## 2. 错误码 byte-identical (3 字面)

- `dm_search.q_required` (400) — q 缺失
- `dm_search.q_too_short` (400) — q < 2 char
- `dm_search.q_too_long` (400) — q > 200 char (反 DoS)

**0 新错码外**: 401 "Unauthorized" 复用既有.

## 3. 反向 grep 锚 (DM-11 实施 PR 必跑)

```
git grep -nE 'dm_search_index|dm_search_table|dm_11_search_log' packages/server-go/internal/  # 0 hit (单源 messages.content 列)
git grep -nE 'admin.*dm.*search|/admin-api/.*dm/search' packages/server-go/internal/api/admin*.go  # 0 hit (ADM-0 §1.3)
git grep -nE 'fts5|MATCH.*dm_search|VIRTUAL TABLE.*dm' packages/server-go/internal/  # 0 hit (FTS5 不走留 v2)
git diff origin/main -- packages/server-go/internal/migrations/ | grep -c '^\+'  # 0 production 行 (0 schema 改)
```

## 4. 不在本轮范围 (deferred)

- ❌ FTS5 走 artifacts_fts 模式 (留 v2 — DM 消息量增长后再考虑跨表 join 复杂度)
- ❌ sort by relevance (留 v2 — 现版 ORDER BY created_at DESC 单源)
- ❌ admin god-mode cross-user search (永久不挂, ADM-0 §1.3)
- ❌ 跨 org search (复用 store.CrossOrg 既有, 留 AP-3 同期)
- ❌ search history persistence (留 v3)
- ❌ per-DM channel filter (现版跨所有 user's DM, ?channel_id= filter 留 v2)
- ❌ client UI (DM search bar 留 follow-up PR)
