# DM Search — cross-DM message search REST endpoint

> **Source-of-truth pointer.** Code at
> `packages/server-go/internal/api/message_search.go` (handler) +
> `packages/server-go/internal/store/dm_11_search_queries.go` (store
> helper). Wire-up at server boot in
> `packages/server-go/internal/server/server.go`.
> Spec brief at
> [`docs/implementation/modules/dm-11-spec.md`](../../implementation/modules/dm-11-spec.md).
> Content-lock at [`docs/qa/dm-11-content-lock.md`](../../qa/dm-11-content-lock.md).

## Why

Blueprint
[`channel-model.md`](../../blueprint/current/channel-model.md) §1.2 + §3.2
mark DM as a concept that **reuses the channel table** (`type='dm'`) but
must stay UX-isolated. DM-11 closes the user-rail gap: cross-DM message
search via `LIKE %q%` over `messages.content`. The implementation
reuses the `messages.content` column (no new schema) and stays scoped
to DM-only channels with channel-member ACL — admin god-mode is
permanently off (蓝图 §1.2 + ADM-0 §1.3 红线).

## Stance (蓝图 channel-model.md §1.2 + §3.2 字面)

- **0 schema 改** — 复用 `messages.content` + LIKE %q% (跟 messages
  search #467 + CHN-13 channel search #583 既有同模式). FTS5 已在
  CV-6 #531 落 `artifacts_fts` 表但**不复用** — 留 v2 (避免跨表 join
  复杂度).
- **DM-only scope** — store helper `SearchDMMessages` JOIN
  `channels ON c.type='dm'` 强制过滤; 反 cross-channel leak (跟
  DM-10 #597 + dm_4_message_edit.go #549 DM-only path 同精神).
- **channel-member ACL 复用 AP-4 + AP-5 模式** — store helper JOIN
  `channel_members ON cm.user_id = caller` (反 cross-user DM leak,
  AP-4 #551 reactions ACL + AP-5 #555 messages ACL 立场承袭).
- **q query param 反 DoS** — q trim + min 2 char + max 200 char;
  3 字面错码守门; limit clamp default 30 / max 50.
- **admin god-mode 永久不挂** — 反向 grep
  `admin.*dm.*search|/admin-api/.*dm/search` in `admin*.go` 0 hit
  (ADM-0 §1.3 红线; cross-user DM search 永久不挂 admin, 跟 DM-10 +
  DM-7 edit history admin god-mode 红线锁链承袭).
- **不返 deleted_at IS NOT NULL 行** — `maskDeletedMessages` helper 守
  (反 deleted leak).

## Endpoint

| Method | Path                              | Purpose                         | ACL                  |
|--------|-----------------------------------|---------------------------------|----------------------|
| GET    | `/api/v1/dm/search?q=<q>&limit=N` | Cross-DM message search         | user-rail (auth Mw)  |

Query params:

| Param   | Required | Validation                                                                      |
|---------|----------|---------------------------------------------------------------------------------|
| `q`     | yes      | trim → `2 ≤ len ≤ 200` (`dm11MinQueryLen` / `dm11MaxQueryLen` const)            |
| `limit` | no       | default 30, clamped to max 50 (`dm11DefaultLimit` / `dm11MaxLimit` const)       |

## Response shape

```
200 {"messages": [...], "count": N}
400 {"code": "dm_search.q_required"}    # q 缺失或全空
400 {"code": "dm_search.q_too_short"}   # len(q) < 2
400 {"code": "dm_search.q_too_long"}    # len(q) > 200
401 "Unauthorized"
500 (search failure, no body code)
```

## Error code byte-identical (3 字面)

| 错码                       | HTTP | 触发                  | content-lock §1 |
|----------------------------|------|-----------------------|-----------------|
| `dm_search.q_required`     | 400  | q 缺失或全空           | ✅              |
| `dm_search.q_too_short`    | 400  | `len(q) < 2`          | ✅              |
| `dm_search.q_too_long`     | 400  | `len(q) > 200`        | ✅              |

字面跟 `internal/api/message_search.go` 锁; 改 = 改两处 (handler +
content-lock §1).

## Validation order (handler)

1. Auth — `mustUser(w, r)` (auth Mw 守 401).
2. q query param required + 2..200 char (反 DoS, 反空查询全表扫).
3. limit clamp default 30 / max 50.
4. `Store.SearchDMMessages(user.ID, q, limit)` — DM-only + channel-
   member ACL JOIN.

## Reverse-grep 锚 (DM-11 实施 PR 必跑)

```
git grep -nE 'dm_search_index|dm_search_table|dm_11_search_log' packages/server-go/internal/  # 0 hit (单源 messages.content 列)
git grep -nE 'admin.*dm.*search|/admin-api/.*dm/search'         packages/server-go/internal/api/admin*.go  # 0 hit (ADM-0 §1.3)
git grep -nE 'fts5|MATCH.*dm_search|VIRTUAL TABLE.*dm'          packages/server-go/internal/  # 0 hit (FTS5 不走留 v2)
git diff origin/main -- packages/server-go/internal/migrations/ | grep -c '^\+'                  # 0 production 行
```

## Tests

- `internal/api/dm11search_test.go` — 10 unit tests (happy /
  q-required / q-too-short / q-too-long / unauthorized / no-match /
  dm-only-excludes-public / non-member-no-leak / limit-clamp /
  deleted-hidden).
- `internal/store/dm_11_search_queries_test.go` (sibling) — store
  helper level: DM-only JOIN + channel-member ACL + ORDER BY
  `created_at DESC` + maskDeletedMessages.

Regression rows: `REG-DM11-001..006` in
[`docs/qa/regression-registry.md`](../../qa/regression-registry.md).

## Not in scope (留账)

- ❌ FTS5 走 `artifacts_fts` 模式 — 留 v2 (DM 消息量增长后再考虑跨表
  join 复杂度).
- ❌ Sort by relevance — 留 v2 (现版 `ORDER BY created_at DESC` 单源,
  跟 messages search #467 同精神).
- ❌ Admin god-mode cross-user search — 永久不挂 (ADM-0 §1.3 红线).
- ❌ Cross-org search — 留 AP-3 同期 (复用 store.CrossOrg 既有).
- ❌ Search history persistence — 留 v3.
- ❌ Per-DM channel filter (`?channel_id=`) — 留 v2 (现版跨所有 user's
  DM).
- ❌ Client UI (DM search bar / dropdown) — 留 follow-up PR (server
  contract 先固化, content-lock §4 列建议字面).
