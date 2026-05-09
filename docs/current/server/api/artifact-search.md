# CV-6 — artifact full-text search endpoint contract (server 单一来源)

> **Source-of-truth pointer.** Schema in
> `packages/server-go/internal/migrations/cv_6_1_artifacts_fts.go`
> (v=34). Handler in `packages/server-go/internal/api/search.go`.
> Wire-up via existing `ArtifactHandler.RegisterRoutes`.

## Why

CV-1 / CV-3 / CV-2 v2 / CV-3 v2 close the artifact CRUD + 5-kind enum +
preview/thumbnail loop. Once owners accumulate dozens / hundreds of
artifacts in a channel, 侧边栏 scroll alone won't cut it — they need a
search input. CV-6 closes that gap with **SQLite FTS5** (built-in,
zero-extra-process); no elasticsearch / opensearch / typesense /
meilisearch / sonic / bleve.

## 原则 (cv-6-spec.md §0)

- **① 复用 SQLite FTS5** — contentless virtual table tied to `artifacts`
  via `content='artifacts' content_rowid='rowid'`; three triggers
  (`artifacts_ai/au/ad`) auto-sync on INSERT/UPDATE/DELETE. No external
  search service.
- **② search owner-only** — channel-scoped (channel_id required); non
  member → 403 `search.channel_not_member`; cross-org → 403
  `search.cross_org_denied` (走 AP-3 `auth.HasCapability` 自动 enforce).
- **③ 反向: 不另建 search_index_table** — FTS5 contentless 跟 artifacts 单一来源;
  no separate schema, no cron reindex.

## Schema (v=34)

```sql
CREATE VIRTUAL TABLE artifacts_fts USING fts5(
    title, body,
    content='artifacts',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER artifacts_ai AFTER INSERT ON artifacts BEGIN
  INSERT INTO artifacts_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
END;
CREATE TRIGGER artifacts_ad AFTER DELETE ON artifacts BEGIN
  INSERT INTO artifacts_fts(artifacts_fts, rowid, title, body)
    VALUES('delete', old.rowid, old.title, old.body);
END;
CREATE TRIGGER artifacts_au AFTER UPDATE ON artifacts BEGIN
  INSERT INTO artifacts_fts(artifacts_fts, rowid, title, body)
    VALUES('delete', old.rowid, old.title, old.body);
  INSERT INTO artifacts_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
END;
```

Initial 回填 at migration time:

```sql
INSERT INTO artifacts_fts(rowid, title, body)
SELECT rowid, title, body FROM artifacts WHERE archived_at IS NULL;
```

**Build tag**: `mattn/go-sqlite3` 不默认编 FTS5 — 必须用
`-tags sqlite_fts5`. Makefile `GOTAGS := sqlite_fts5` 默认全套
build/test/run 自动带; CI 也带.

## Endpoint

```
GET /api/v1/artifacts/search?q=<query>&channel_id=<id>&limit=<n>
Authorization: <session cookie>
```

Bounds:

- `q` required, 1..256 chars (反 DoS 提前 reject 在 FTS5 之前).
- `channel_id` required v0 (跨 channel 全局 search 留 v2+).
- `limit` optional, default 50, max 200.

ACL gates:

- No auth user → **401 Unauthorized**.
- `q` empty → **400 `search.query_empty`**.
- `q` length > 256 → **400 `search.query_too_long`**.
- channel_id non-member → **403 `search.channel_not_member`**.
- cross-org user → **403 `search.cross_org_denied`** (走 AP-3
  `auth.HasCapability(ctx, ReadArtifact, channel:<id>)` 自动 enforce).

Result row shape:

```json
{
  "artifact_id": "<uuid>",
  "title": "Roadmap Q3",
  "snippet": "# <mark>Hello</mark> world plan",
  "kind": "markdown",
  "channel_id": "<uuid>",
  "current_version": 1
}
```

`snippet()` args byte-identical (跟 content-lock §1 + 原则 ⑧):

```
snippet(artifacts_fts, 1, '<mark>', '</mark>', '...', 32)
```

(col=1 is `body`; prefix/suffix `<mark>...</mark>` literal; ellipsis
`...`; window 32 tokens).

## Excluded from results (设计 ⑥)

- archived artifacts (`archived_at IS NOT NULL`) — CV-1 既有不变量.

## 错码字面单一来源 (跟 PreviewErrCode* + AP-1/AP-2/AP-3/CV-3 v2 const 同模式)

```go
SearchErrCodeNotOwner         = "search.not_owner"
SearchErrCodeChannelNotMember = "search.channel_not_member"
SearchErrCodeQueryEmpty       = "search.query_empty"
SearchErrCodeQueryTooLong     = "search.query_too_long"
SearchErrCodeCrossOrgDenied   = "search.cross_org_denied"
```

Drift caught by content-lock §4 双向 grep + 验收 §1.7 unit.

## 跨 milestone byte-identical 锁定

- 跟 CV-1 #348 / CV-3 #408 / CV-2 v2 #517 / CV-3 v2 #528 五 kind enum +
  artifacts 单一来源 同源 (FTS5 contentless 不拆表, 不动既有 schema).
- 跟 CV-1.2 #342 + CV-2 v2 + CV-3 v2 + CV-4 + AL-5 + AP-3 cross-org **6
  处 owner-only ACL** 同精神.
- 跟 AP-1 #493 `HasCapability` 单一来源 + AP-3 #521 cross-org gate 同源
  (search 路径自动经 cross-org gate).
- 错码字面单一来源 + content-lock 双向 docs 同模式 (跟 CV-2 v2 / CV-3 v2).

## 不在范围

- v2 message / DM 全文搜索 (留 DM-4+ 单独 milestone, 走 messages 表 FTS5).
- BM25 custom ranking / saved query / 跨 channel 全局 / agent search
  action — 留 v2+.
- elasticsearch / opensearch / typesense / meilisearch / sonic / bleve —
  蓝图 SQLite 单一来源原则一致.
