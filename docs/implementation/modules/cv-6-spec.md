# CV-6 spec brief — artifact 全文搜索 (Phase 5+ 续作)

> 战马C · 2026-04-30 · ≤80 行 spec lock (4 件套之一; canvas-vision §1.4 续作 — artifact 全文搜索 给 owner 大量 artifact 累积时定位)
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.4 (artifact 集合: 多类型 markdown / code / image_link / video_link / pdf_link, "首屏快读" 字面承袭 — 大量 artifact 时 owner 需 search 入口) + [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.3 主入口 + [`channel-model.md`](../../blueprint/current/channel-model.md) §1.4 channel.created_by owner 主权
> **关联**: CV-1 #348 markdown artifact + artifact_versions ✅ + CV-3 #408 三 kind enum + CV-2 v2 #517 五 kind enum (markdown/code/image_link/video_link/pdf_link) ✅ + CV-3 v2 #528 thumbnail_url ✅ + CV-1.2 #342 owner-only ACL (channel.created_by gate) + AP-1 #493 HasCapability SSOT + AP-3 #521 cross-org gate

> ⚠️ CV-6 是 **wrapper milestone** (跟 CV-2 v2 / CV-3 v2 / AL-5 / AP-2 / AP-3 wrapper 同模式) — 复用 SQLite FTS5 (内置 search infra, 不另起 elasticsearch / opensearch / typesense), **不裂新 search service**, 仅补 server-side FTS5 virtual table + GET /artifacts/search?q= owner-only endpoint + client SearchBox.

## 0. 关键约束 (3 条立场, 蓝图字面承袭)

1. **复用 SQLite FTS5 (现有 search infra, 不另起 elasticsearch)** (蓝图 §1.4 + 整体技术栈 SQLite SSOT 字面承袭): server-side 走 SQLite FTS5 `CREATE VIRTUAL TABLE artifacts_fts USING fts5(title, body, content=artifacts, content_rowid=id, tokenize='unicode61 remove_diacritics 2')`; FTS5 触发器同步 `artifacts INSERT/UPDATE/DELETE` → `artifacts_fts` (CV-1 + CV-3 v2 既有 artifact CRUD 路径自动入 index, 不改 endpoint); 反约束: 不引入 elasticsearch / opensearch / typesense / meilisearch / sonic / bleve client lib (反向 grep go.mod / package.json count==0); MATCH 查询走 FTS5 内置 query syntax + 高亮 snippet
2. **search owner-only (跟 owner-only ACL 6 处一致)** (跟 CV-1.2 commit + CV-2 v2 preview + CV-3 v2 thumbnail + CV-4 iterate + AL-5 recover + AP-3 cross-org owner-only 6 处 ACL 同模式): GET /api/v1/artifacts/search?q=`<query>`&channel_id=`<id>`(optional) — channel-scoped (channel.created_by gate) + cross-channel reject (跟 CV-1.2 立场 ① channel-scoped 同精神); reject without auth → 401; cross-channel non-member → 403 + `search.channel_not_member` 错码; cross-org user (走 AP-3 既有 cross-org gate) → 403; 反约束: 不暴露其他 owner artifact (反向 grep `search.*all_artifacts\|search.*cross_owner` count==0)
3. **反向 grep search_index_table 等 0 hit (不另起 schema)** (跟 CV-3.1 #396 立场 ① "enum 扩不裂表" + CV-2 v2 / CV-3 v2 五连 ALTER NULL 同精神 — 不裂表): 反向 grep `CREATE TABLE.*search_index\|artifact_search_results\|fts_documents` count==0; FTS5 virtual table 命名 `artifacts_fts` byte-identical (跟 SQLite FTS5 contentless 模式同模式), 三 trigger (insert/update/delete) byte-identical 命名 `artifacts_ai/au/ad`; 反约束: 不另起 search 表 / 不裂 search service / 不引入 cron 框架 reindex (FTS5 trigger 自动同步)

## 1. 拆段实施 (CV-6.1 / 6.2 / 6.3, ≤3 PR 同 branch 叠 commit, 一 milestone 一 PR 默认 1 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CV-6.1** server schema migration v=36 + endpoint | `internal/migrations/cv_6_1_artifacts_fts.go` v=36 — `CREATE VIRTUAL TABLE artifacts_fts USING fts5(title, body, tokenize='unicode61')` (contentless 模式 跟 artifacts 单源 SSOT, content_rowid='id') + 三 AFTER trigger (`artifacts_ai INSERT` / `artifacts_au UPDATE` / `artifacts_ad DELETE`) 自动同步 + initial backfill `INSERT INTO artifacts_fts(rowid, title, body) SELECT id, title, body FROM artifacts WHERE archived_at IS NULL` (legacy 行入 index); `internal/api/search.go::handleArtifactSearch` (GET /api/v1/artifacts/search?q=&channel_id=&limit=50) — channel-scoped owner ACL + FTS5 MATCH query + snippet highlight (`snippet(artifacts_fts, ...)` 50 字 ±20 字 context); 5 错码字面 (search.{not_owner / channel_not_member / query_empty / query_too_long / cross_org_denied}); 7 unit (TestCV61_CreatesFTS5VirtualTable + TriggerSync + BackfillExistingRows + SearchHappyPath + NonOwner403 + EmptyQuery400 + CrossOrgReject) | 待 PR (战马C) | 战马C |
| **CV-6.2** client SPA SearchBox + result list | `packages/client/src/components/SearchBox.tsx` — `<input type="search" debounce 300ms>` 入 GET /artifacts/search; `SearchResultList.tsx` 列结果 (title + snippet + ArtifactThumbnail 复用 CV-3 v2); kbd shortcut `/` focus + `Esc` clear; ChannelView 集成 (sidebar top); 反约束: 不引入 fuzzy-search / fuse.js / minisearch (HTML5 native + server SSOT, 跟 CV-2 v2 / CV-3 v2 立场 ② 同精神); 5 vitest case (debounce + http call + result list + kbd shortcut + 反 client-side fuzzy lib) | 待 PR (战马C) | 战马C |
| **CV-6.3** server full-flow integration + closure | server-side full-flow: insert markdown/code artifact w/ "Hello world" → search "hello" → 200 + result 含 artifact_id + snippet 高亮; cross-channel non-member reject 403; cross-org reject (走 AP-3 path) 403; archived_at IS NOT NULL 不出现; 反约束 grep 5 (反 elasticsearch import / 反 search_index 表 / 反 cross-owner / 反 hardcode error / 反 client-side fuzzy lib); registry §3 REG-CV6-001..N + acceptance + PROGRESS [x] CV-6 + docs/current sync (server/api/artifact-search.md + client/search-box.md, 跟 CV-2 v2 / CV-3 v2 双 docs 同模式) | 待 PR (战马C) | 战马C / 烈马 |

## 2. 留账边界 (不接 v2+)

- v2 message + DM 全文搜索 (留 DM-4+) — CV-6 仅 artifact-scoped, message search 是单独 milestone (走 messages 表 FTS5)
- BM25 ranking custom (留 v2+, FTS5 默认 ranking 够用 v0)
- 跨 channel global search (走 owner-only 多 channel 联查留 v2+, v0 单 channel-scoped 或 owner 全 artifact)
- agent 触发 search (留 v2+, BPP-frame 加 search action)
- search 历史 / saved query (留 v2+ 走 user_settings)
- search 高亮 client SPA marker (反约束: server-side `snippet()` 已带 ` <mark>...</mark>` 字面, client 直 dangerouslySetInnerHTML 走既有 markdown sanitize path)

## 3. 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) 不另起 search 表 (FTS5 contentless 跟 artifacts 单源 SSOT)
git grep -nE 'CREATE TABLE.*search_index|artifact_search_results|fts_documents' \
  packages/server-go/internal/migrations/  # 0 hit
# 2) 不引入 elasticsearch / opensearch / typesense / meilisearch / sonic / bleve
git grep -nE '"github\.com/[^"]*elasticsearch|opensearch|typesense|meilisearch|sonic|bleve|blevesearch"' \
  packages/server-go/  # 0 hit
# 3) 不引入 client-side fuzzy lib (HTML5 native + server SSOT)
grep -E '"fuse\.js"|"fuse"|"minisearch"|"fuzzysort"|"flexsearch"' \
  packages/client/package.json  # 0 hit
# 4) cross-owner search bypass (反向: search 走 owner-only ACL 同 6 处一致)
git grep -nE 'search.*all_artifacts|search.*cross_owner|skip.*owner.*search' \
  packages/server-go/internal/api/  # 0 hit
# 5) hardcode error code in handler (反 const 单源漂移, 跟 AP-1/AP-2/AP-3/CV-2 v2/CV-3 v2 const 单源同模式)
git grep -nE '"search\.(not_owner|channel_not_member|query_empty|query_too_long|cross_org_denied)"' \
  packages/server-go/internal/  # ≥5 hits (api/search.go const) + 0 hit hardcode in handler
```

## 4. 不在范围

- v2 message / DM 全文搜索 (留 DM-4+ 单独 milestone)
- BM25 custom ranking / 跨 channel global / agent search action / saved query (留 v2+)
- elasticsearch / opensearch / typesense / meilisearch 重 search service (蓝图 SQLite SSOT 字面承袭)
- search archived artifact (archived_at IS NOT NULL 反向断言不出现)

## 5. 跨 milestone byte-identical 锁

- 跟 CV-1 #348 + CV-3 #408 + CV-2 v2 #517 + CV-3 v2 #528 五 kind enum + artifacts 表 SSOT 同源 (CV-6 复用既有 schema, FTS5 contentless 不裂)
- 跟 CV-1.2 #342 owner-only ACL (channel.created_by gate) + CV-2 v2 + CV-3 v2 + AL-5 + AP-3 cross-org gate **6 处 owner-only ACL** 同精神 (改 = 改 ACL helper 一处)
- 跟 AP-1 HasCapability SSOT 同精神 (search endpoint 走 HasCapability + ChannelScopeStr; 反约束 endpoint 0 行改 ACL 路径)
- 跟 AP-3 #521 cross-org gate 同源 (search 路径自动经 abac.HasCapability cross-org gate)
- 跟 CV-3 v2 #528 + CV-2 v2 #517 thin recording shim + 错码字面单源 + 双向 docs 同模式
