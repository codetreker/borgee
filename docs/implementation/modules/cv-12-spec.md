# CV-12 spec brief — artifact comment search (CV-5..CV-11 续, client only)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 + CV-5..CV-11 单源延伸 + CV-9/10/11 client-only 同模式 + thinking 5-pattern 第 9 处链 (无新增, 但锁链不漂). CV-12 让 artifact comment 内容可搜索 — 复用既有 `GET /api/v1/channels/{channelId}/messages/search?q=` (CV-5 既有 message-search endpoint), channelId 走 artifact: namespace channel UUID. **0 server production code + 0 schema 改 + 0 新 endpoint + 0 新 lib**.

## 0. 关键约束 (4 项立场, 跨链承袭)

1. **search 走既有 `GET /api/v1/channels/{channelId}/messages/search?q=` 单源** — comment 既然落 messages 表 + namespace channel (CV-5 #530 立场 ①), 既有 message-search endpoint 自动覆盖 comment-search. **反约束**: 不开 `/api/v1/artifacts/:id/comments/search` 别名 endpoint / 不另起 FTS5 表 / 不另写 search query 路径. 反向 grep `/comments/search\|comment_fts\|artifact_search.*PRIMARY` 在 internal/ count==0.

2. **owner-only ACL byte-identical 14+ 处一致, admin god-mode 不挂** — message-search 既有 readPerm + private channel access check (messages.go::handleSearchMessages 既有 ACL); admin god-mode 不入 user rail (跟 ADM-0 §1.3 + CV-5..CV-11 同源). **反向 grep**: `admin.*comment.*search\|admin.*search.*comment` 在 admin*.go count==0.

3. **thinking 5-pattern 锁链不漂** — search 是 read-side, 不解 markdown 不评 thinking; 5-pattern 仍是 server CV-7/CV-8 既有 hook (write-side gate). **client search input 不预判 thinking** (反向断). 5-pattern 锁链 8 处不变 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9 + CV-11).

4. **client UI: search input + 结果 DOM 锚 + 文案 byte-identical** (content-lock): input `data-cv12-search-input="<artifactId>"` 锚 + 结果 list `data-cv12-search-result-id="<msgId>"` 锚; 空查询 不调 API (反约束 不打 server); 0 results 文案 "未找到匹配评论" byte-identical. **反向 grep**: `data-cv12-search-input\|data-cv12-search-result-id` ≥2 hit; `未找到匹配评论` ≥1 hit.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-12.1 server | (无 server 实施) + `internal/api/cv_12_search_namespace_test.go` 1 unit 反向断 既有 message-search endpoint 在 `artifact:<id>` namespace channel 上工作 byte-identical to text channel | 1 unit PASS; **0 行 production code** |
| CV-12.2 client | `packages/client/src/lib/api.ts::searchArtifactComments` (新 thin wrapper) + `packages/client/src/components/ArtifactCommentSearchBox.tsx` (新) + content-lock | hook 调既有 `/api/v1/channels/${ch}/messages/search?q=`; component: input + 结果 list + 0 result 文案; 4 vitest case |
| CV-12.3 e2e + closure | `packages/e2e/tests/cv-12-comment-search.spec.ts` (3 case, REST-driven) + REG-CV12-001..005 + acceptance + PROGRESS [x] | seed 3 comments → search "needle" → 1 hit / search "absent" → 0 hit / cross-channel reject |

## 2. 错误码 (0 新 — 沿用 CV-5..CV-11 既有)

CV-12 复用既有 message-search response shape; 0 错误码新增.

## 3. 反向 grep 锚 (CV-12 实施 PR 必跑)

```
git grep -nE '/comments/search|comment_fts|artifact_search.*PRIMARY' packages/server-go/internal/  # 0 hit (单源 message-search)
git grep -nE 'admin.*comment.*search|admin.*search.*comment' packages/server-go/internal/api/admin  # 0 hit (ADM-0 §1.3)
git grep -nE 'data-cv12-search-input|data-cv12-search-result-id' packages/client/src/  # ≥ 2 hit (DOM 锚)
git grep -nE '未找到匹配评论' packages/client/src/  # ≥ 1 hit (文案 byte-identical)
git grep -nE 'cv12.*fts|cv12.*new.*search' packages/server-go/internal/  # 0 hit (反约束新 search 路径)
```

## 4. 不在本轮范围 (deferred)

- ❌ FTS5 引入 (LIKE 既有 search 够用; FTS5 留 CV-13+ scale 真要时)
- ❌ search highlight (留 v2 — 命中词 marker)
- ❌ admin god-mode comment search (ADM-0 §1.3 红线)
- ❌ schema migration (0 schema 改)
- ❌ cross-artifact search aggregator (留 v2 — search 仅 per-artifact namespace)
