# DM-11 content-lock — DM cross-DM search 文案 + DOM SSOT

> 改 = 改本文件一处. e2e + vitest + 实施代码三轨字面 byte-identical.

## 1. 错码字面 byte-identical (3 字面)

| 错码 | HTTP | 触发 |
|---|---|---|
| `dm_search.q_required` | 400 | q 缺失或全空 |
| `dm_search.q_too_short` | 400 | q < 2 char |
| `dm_search.q_too_long` | 400 | q > 200 char |

## 2. clamp 阈值 (反 DoS + cursor 滥用)

| Const | 值 |
|---|---|
| `dm11MinQueryLen` | 2 |
| `dm11MaxQueryLen` | 200 |
| `dm11DefaultLimit` | 30 |
| `dm11MaxLimit` | 50 |

## 3. Endpoint shape

```
GET /api/v1/dm/search?q=<query>&limit=<N>
→ 200 {"messages": [...], "count": N}
→ 400 {"code": "dm_search.q_required"}
→ 400 {"code": "dm_search.q_too_short"}
→ 400 {"code": "dm_search.q_too_long"}
→ 401 "Unauthorized"
```

## 4. Client UI 文案 (留 follow-up PR)

| 用途 | 字面 (建议, follow-up PR 锁) |
|---|---|
| input placeholder | `搜索 DM 消息` |
| empty state | `未找到匹配` |
| count label | `共 ${N} 条消息` |
| toast on error | `搜索失败, 请稍后重试` |

## 5. 反约束 grep (DM-11 实施 PR 必跑)

```
git grep -nE 'dm_search_index|dm_search_table|dm_11_search_log' packages/server-go/internal/  # 0 hit
git grep -nE 'admin.*dm.*search|/admin-api/.*dm/search' packages/server-go/internal/api/admin*.go  # 0 hit
git grep -nE 'fts5|MATCH.*dm_search|VIRTUAL TABLE.*dm' packages/server-go/internal/  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/ | grep -c '^\+'  # 0
```
