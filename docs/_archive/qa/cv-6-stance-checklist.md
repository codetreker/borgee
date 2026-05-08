# CV-6 立场反查清单 (战马C v0)

> 战马C · 2026-04-30 · 立场 review checklist (跟 CV-2 v2 / CV-3 v2 / AP-2 / AP-3 同模式)
> **目的**: CV-6 三段实施 (6.1 schema + server endpoint / 6.2 client SearchBox / 6.3 e2e + closure) PR review 时, 飞马 / 烈马按此清单逐立场 sign-off.
> **关联**: spec `docs/implementation/modules/cv-6-spec.md` (战马C v0, d2fe1f0). 复用 CV-1 / CV-3 / CV-2 v2 / CV-3 v2 五 kind enum + AP-1 HasCapability SSOT + AP-3 cross-org gate + 6 处 owner-only ACL.

## §0 立场总表 (3 立场 + 6 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | 复用 SQLite FTS5 (现有 search infra, 不另起 elasticsearch) | canvas-vision §1.4 + 整体技术栈 SQLite SSOT 字面承袭 | `CREATE VIRTUAL TABLE artifacts_fts USING fts5(title, body, content=artifacts, content_rowid=id, tokenize='unicode61')` contentless 模式; 三 trigger `artifacts_ai/au/ad` 自动同步; 反向 grep 7 keyword `elasticsearch\|opensearch\|typesense\|meilisearch\|sonic\|bleve\|blevesearch` 在 server-go count==0 |
| ② | search owner-only (跟 owner-only ACL 6+ 处一致) | CV-1.2 + CV-2 v2 + CV-3 v2 + CV-4 + AL-5 + AP-3 cross-org 6 处 | channel-scoped: `channel.created_by == user.ID` gate + non-member → 403; cross-org gate 走 AP-3 HasCapability 单源自动 enforce; 反向 grep `search.*all_artifacts\|search.*cross_owner\|skip.*owner.*search` 在 internal/api/ count==0 |
| ③ | 反向 grep search_index_table 等 0 hit (不另起 schema) | 跟 CV-3.1 立场 ① "enum 扩不裂表" + 五连 ALTER NULL 同精神 | FTS5 contentless 跟 artifacts 单源 SSOT; 反向 grep `CREATE TABLE.*search_index\|artifact_search_results\|fts_documents` count==0; 不引入 cron 框架 reindex (FTS5 trigger 自动同步) |
| ④ (边界) | 错码字面单源 (跟 AP-1/AP-2/AP-3/CV-2 v2/CV-3 v2 const 同模式) | const SSOT 同精神 | `SearchErrCode{NotOwner / ChannelNotMember / QueryEmpty / QueryTooLong / CrossOrgDenied}` const 字面单源; 反向 grep handler hardcode `"search\."` 字面 in non-const path count==0 |
| ⑤ (边界) | AP-3 cross-org gate 走 HasCapability 单源 (test 必 cover) | AP-3 #521 立场 ① + AP-1 SSOT 同精神 | search endpoint 经 `auth.HasCapability(ctx, "read_artifact", channel:<id>)` 路径自动经 AP-3 cross-org gate; 单测 cross-org user → 403 必 cover (TestCV63_CrossOrgDenied) |
| ⑥ (边界) | archived_at IS NOT NULL 不出现 | CV-1 archive 既有不变量 | search query 加 `WHERE archived_at IS NULL` 过滤; 反向断言 archived artifact 不出现在 result list |
| ⑦ (边界) | 不引入 client-side fuzzy lib (HTML5 native + server SSOT) | 跟 CV-2 v2 / CV-3 v2 立场 "不引入重 lib" 同精神 | 反向 grep `"fuse\.js"\|"fuse"\|"minisearch"\|"fuzzysort"\|"flexsearch"` 在 packages/client/package.json count==0 |
| ⑧ (边界) | server-side snippet `<mark>` 字面 + client 走既有 markdown sanitize | 跟 CV-1 markdown DOMPurify 同精神 | snippet() 函数 5 args 字面 byte-identical (`snippet(artifacts_fts, 1, '<mark>', '</mark>', '...', 32)` for body column 50 字 ±20 字 context); 反向: 不另起 client 高亮 lib |
| ⑨ (边界) | debounce 300ms client + kbd `/` focus + Esc clear | UX 标尺 — 跟既有 SearchBox / MentionList 同精神 | 反向: 不每键发 HTTP (debounce 300ms 必经过); 反 setTimeout/setInterval 缠绕 (用 useEffect cleanup 同模式 React) |

## §1 立场 ① 复用 SQLite FTS5 (CV-6.1 守)

**蓝图字面源**: canvas-vision §1.4 + 整体技术栈 SQLite SSOT

**反约束清单**:

- [ ] migration v=36 `cv_6_1_artifacts_fts.go` — `CREATE VIRTUAL TABLE artifacts_fts USING fts5(title, body, content=artifacts, content_rowid=id, tokenize='unicode61 remove_diacritics 2')` contentless 模式
- [ ] 三 trigger byte-identical 命名 `artifacts_ai` (AFTER INSERT) / `artifacts_au` (AFTER UPDATE) / `artifacts_ad` (AFTER DELETE)
- [ ] initial backfill `INSERT INTO artifacts_fts(rowid, title, body) SELECT id, title, body FROM artifacts WHERE archived_at IS NULL`
- [ ] 反向 grep `elasticsearch\|opensearch\|typesense\|meilisearch\|sonic\|bleve\|blevesearch` 在 server-go go.mod / go.sum count==0 (除 lock 文件版本号自动撞匹配)

## §2 立场 ② search owner-only (CV-6.2 守)

**蓝图字面源**: 6 处 owner-only ACL 同精神 (CV-1.2 commit + CV-2 v2 preview + CV-3 v2 thumbnail + CV-4 iterate + AL-5 recover + AP-3 cross-org)

**反约束清单**:

- [ ] handler `handleArtifactSearch` 走 `auth.UserFromContext` + channel.created_by gate (跟 CV-1.2 rollback 同 path); 调 `auth.HasCapability(ctx, auth.ReadArtifact, auth.ChannelScopeStr(channelID))` 单源
- [ ] cross-channel non-member → 403 + `search.channel_not_member`
- [ ] cross-org user → 403 (走 AP-3 HasCapability 自动经 cross-org gate)
- [ ] no auth user → 401
- [ ] 反向 grep `search.*all_artifacts\|search.*cross_owner` count==0

## §3 立场 ③ + ⑤ 反向 grep + AP-3 cross-org test 必 cover (CV-6.3 守)

**蓝图字面源**: 跟 CV-3.1 立场 ① + 五连 ALTER NULL + AP-3 #521 cross-org gate 同精神

**反约束清单**:

- [ ] 5 grep pattern 全 count==0: `CREATE TABLE.*search_index|artifact_search_results|fts_documents` / `elasticsearch|opensearch|typesense|meilisearch|sonic|bleve|blevesearch` / `"fuse\.js"|fuse|minisearch|fuzzysort|flexsearch` / `search.*all_artifacts|search.*cross_owner|skip.*owner.*search` / handler hardcode `"search\."` 非 const
- [ ] full-flow integration: insert markdown w/ "Hello world" → search "hello" → 200 + result + snippet `<mark>Hello</mark> world`; cross-channel non-member → 403; **cross-org user → 403** (TestCV63_CrossOrgDenied 必 cover, 跟 AP-3 #521 立场 ① 同源)
- [ ] archived_at IS NOT NULL 反向断言不出现
- [ ] registry §3 REG-CV6-001..006 + acceptance + PROGRESS [x] CV-6 + docs/current sync

## §4 联签清单 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐): _(签)_
- [ ] 烈马 (反向 grep + 单测覆盖率 ≥84% + 5 反约束全 count==0 + AP-3 cross-org test cover): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 9 项全过): _(签)_
