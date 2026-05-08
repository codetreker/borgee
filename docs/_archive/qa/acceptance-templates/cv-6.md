# Acceptance Template — CV-6: artifact 全文搜索 wrapper

> Spec: `docs/implementation/modules/cv-6-spec.md` (战马C v0, d2fe1f0)
> 蓝图: `canvas-vision.md` §1.4 + `auth-permissions.md` §1.3 + 6 处 owner-only ACL
> 前置: CV-1 #348 + CV-3 #408 + CV-2 v2 #517 + CV-3 v2 #528 (五 kind enum + thumbnail) + AP-1 #493 HasCapability SSOT + AP-3 #521 cross-org gate + CV-1.2 owner-only ACL

## 验收清单

### CV-6.1 server schema migration v=36 + endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 schema migration v=36 — `CREATE VIRTUAL TABLE artifacts_fts USING fts5(title, body, content=artifacts, content_rowid=id, tokenize='unicode61 remove_diacritics 2')` contentless + 三 trigger `artifacts_ai/au/ad` 自动同步 + initial backfill | unit | 战马C / 烈马 | `internal/migrations/cv_6_1_artifacts_fts_test.go::TestCV61_CreatesFTS5VirtualTable` + `TestCV61_TriggerSyncOnInsert` + `TestCV61_TriggerSyncOnUpdate` + `TestCV61_TriggerSyncOnDelete` + `TestCV61_BackfillExistingRows` + `TestCV61_RegistryHasV36` + `TestCV61_Idempotent` |
| 1.2 GET /api/v1/artifacts/search?q=&channel_id=&limit= happy path — markdown / code 全文搜索, 200 + result list (artifact_id + title + snippet `<mark>` highlight) | unit | 战马C / 烈马 | `internal/api/search_test.go::TestCV62_SearchHappyPath_MarkdownBody` + `TestCV62_SearchHappyPath_CodeTitle` |
| 1.3 立场 ② owner-only ACL — non-owner / non-member → 403 + `search.channel_not_member`; admin god-mode → 401 (跟 6 处 owner-only ACL 同精神) | unit | 战马C / 烈马 | `TestCV62_NonOwner403` + `TestCV62_Admin401` |
| 1.4 立场 ⑤ AP-3 cross-org gate — cross-org user → 403 (走 AP-3 HasCapability 自动经); 单测必 cover (AP-3 #521 立场 ① 同源) | unit | 战马C / 烈马 | `TestCV63_CrossOrgDenied` (org-A user search org-B channel → 403) |
| 1.5 query empty / too long → 400 `search.query_empty` / `search.query_too_long` (max 256 字符); reject before FTS5 query (反 DoS) | unit | 战马C / 烈马 | `TestCV62_QueryEmpty400` + `TestCV62_QueryTooLong400` |
| 1.6 立场 ⑥ archived_at IS NOT NULL 不出现 (CV-1 archive 既有不变量) | unit | 战马C / 烈马 | `TestCV62_ArchivedNotInResults` (archive 1 row, search 后不出现) |
| 1.7 5 错码字面单源 const (跟 AP-1/AP-2/AP-3/CV-2 v2/CV-3 v2 const 同模式) | unit | 战马C / 烈马 | `TestCV62_ErrCodeConstByteIdentical` (5 const 字面 byte-identical) |

### CV-6.2 client SPA SearchBox + result list

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `SearchBox.tsx` `<input type="search">` debounce 300ms + kbd `/` focus + `Esc` clear; ChannelView sidebar 集成 | vitest | 战马C | `packages/client/src/__tests__/SearchBox.test.tsx::debounces 300ms` + `kbd / focuses input` + `Esc clears query` |
| 2.2 `SearchResultList.tsx` 列结果 — title + snippet (server `<mark>` 字面, client 走既有 markdown sanitize path 不另起) + ArtifactThumbnail 复用 CV-3 v2 (markdown/code) / MediaPreview 复用 CV-2 v2 (image/video/pdf) | vitest | 战马C | `SearchResultList.test.tsx::renders title + snippet HTML` + `kind dispatch 跟 ArtifactThumbnail/MediaPreview 兼容` |
| 2.3 立场 ⑦ 反约束 — 不引入 fuse.js / minisearch / fuzzysort / flexsearch (HTML5 native + server SSOT) | grep | 烈马 | `grep -E "fuse\.js\|fuse\|minisearch\|fuzzysort\|flexsearch" packages/client/package.json` count==0 |
| 2.4 5 错码文案 (跟 server const byte-identical) — error toast 显: 用户搜索时遇 `search.query_empty` → "请输入搜索词"; `search.query_too_long` → "搜索词太长 (最长 256 字符)"; `search.channel_not_member` → "无权访问此频道"; `search.cross_org_denied` → "跨组织搜索被禁"; `search.not_owner` → "需要频道所有者权限" | vitest | 战马C / 野马 | `SearchBox.test.tsx::toast 5 错码文案 byte-identical` |

### CV-6.3 e2e + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 server-side full-flow integration — insert markdown "Hello world" → search "hello" → 200 + snippet 高亮 `<mark>Hello</mark> world` (server-side snippet() 5 args byte-identical) | http unit | 战马C / 烈马 | `internal/api/cv_6_3_search_integration_test.go::TestCV63_FullFlow_HighlightsHelloMarkdownBody` |
| 3.2 cross-channel non-member 403 + cross-org 403 (AP-3 path) + archived 不出现 + 反向 grep 5 pattern 全 count==0 | unit | 烈马 | `TestCV63_CrossOrgDenied` + `TestCV63_NonMemberRejected` + `TestCV63_ArchivedSkipped` + `TestCV63_ReverseGrep_5Patterns_AllZeroHit` |
| 3.3 closure: registry §3 REG-CV6-001..006 + acceptance + PROGRESS [x] CV-6 + docs/current sync (server/api/artifact-search.md + client/search-box.md, 跟 CV-2 v2 / CV-3 v2 双 docs 同模式) | docs | 战马C / 烈马 | registry + PROGRESS + 4 件套全闭 |

## 不在本轮范围 (spec §4)

- v2 message / DM 全文搜索 (留 DM-4+)
- BM25 custom ranking / 跨 channel global / agent search action / saved query (留 v2+)
- elasticsearch / opensearch / typesense / meilisearch (蓝图 SQLite SSOT 字面承袭)
- search archived artifact (反向断言不出现)

## 退出条件

- CV-6.1 1.1-1.7 (schema FTS5 + endpoint + ACL + AP-3 cross-org cover + query bounds + archived skip + 错码 const) ✅
- CV-6.2 2.1-2.4 (SearchBox debounce/kbd + ResultList kind dispatch + 反约束 lib + 5 错码文案) ✅
- CV-6.3 3.1-3.3 (full-flow + 反向 grep + closure) ✅
- 现网回归不破: AP-1/AP-2/AP-3/CV-2 v2/CV-3 v2 路径零变 (FTS5 contentless 不动 artifacts 既有 schema)
- REG-CV6-001..006 落 registry + 5 反约束 grep 全 count==0
- 4 件套全闭 (spec ✅ + stance ✅ + acceptance ✅ + content-lock ✅ — 5 错码文案锁)

## 更新日志

- 2026-04-30 — 战马C v0 acceptance template (4 件套第二件): 3 段实施 (1.1-1.7 / 2.1-2.4 / 3.1-3.3) + 4 不在范围 + 6 项退出条件. 联签 CV-6.1/.2/.3 三段同 branch 同 PR (一 milestone 一 PR 协议默认 1 PR).
