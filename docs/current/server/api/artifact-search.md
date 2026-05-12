# CV-6 — artifact full-text search endpoint contract (server single source)

> **Single-source pointer.** Schema in
> `packages/server-go/internal/migrations/cv_6_1_artifacts_fts.go`
> (v=34). Handler in `packages/server-go/internal/api/search.go`.
> Route registration via existing `ArtifactHandler.RegisterRoutes`.

## Why

CV-1 / CV-3 / CV-2 v2 / CV-3 v2 cover artifact CRUD + 5-kind enum +
preview/thumbnail endpoints. Once owners accumulate dozens / hundreds of
artifacts in a channel, sidebar scrolling is insufficient — they need a
search input. CV-6 closes that gap with **SQLite FTS5** (built-in,
no extra process); no elasticsearch / opensearch / typesense /
meilisearch / sonic / bleve.

## Principles (cv-6-spec.md §0)

- **① Reuse SQLite FTS5** — contentless virtual table tied to `artifacts`
  via `content='artifacts' content_rowid='rowid'`; three triggers
  (`artifacts_ai/au/ad`) auto-sync on INSERT/UPDATE/DELETE. No external
  search service.
- **② search owner-only** — channel-scoped (channel_id required); non
  member → 403 `search.channel_not_member`; cross-org → 403
  `search.cross_org_denied` (enforced through AP-3 `auth.HasCapability`).
- **③ Do not add `search_index_table`** — FTS5 contentless stays tied to
  artifacts as the single source;
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

Initial backfill at migration time:

```sql
INSERT INTO artifacts_fts(rowid, title, body)
SELECT rowid, title, body FROM artifacts WHERE archived_at IS NULL;
```

**Build tag**: `mattn/go-sqlite3` does not compile FTS5 by default; use
`-tags sqlite_fts5`. Makefile `GOTAGS := sqlite_fts5` adds it to the default
build/test/run commands, and CI uses it too.

## Endpoint

```
GET /api/v1/artifacts/search?q=<query>&channel_id=<id>&limit=<n>
Authorization: <session cookie>
```

Bounds:

- `q` required, 1..256 chars (DoS guard; reject before calling FTS5).
- `channel_id` required v0 (cross-channel global search is left for v2+).
- `limit` optional, default 50, max 200.

ACL checks:

- No auth user → **401 Unauthorized**.
- `q` empty → **400 `search.query_empty`**.
- `q` length > 256 → **400 `search.query_too_long`**.
- channel_id non-member → **403 `search.channel_not_member`**.
- cross-org user → **403 `search.cross_org_denied`** (enforced through AP-3
  `auth.HasCapability(ctx, ReadArtifact, channel:<id>)`).

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

`snippet()` args are byte-identical with content-lock §1 + principle ⑧:

```
snippet(artifacts_fts, 1, '<mark>', '</mark>', '...', 32)
```

(col=1 is `body`; prefix/suffix `<mark>...</mark>` literal; ellipsis
`...`; window 32 tokens).

## Excluded from results (design ⑥)

- archived artifacts (`archived_at IS NOT NULL`) — existing CV-1 invariant.

## Error-code literals as the single source

Same pattern as `PreviewErrCode*` and AP-1/AP-2/AP-3/CV-3 v2 constants.

```go
SearchErrCodeNotOwner         = "search.not_owner"
SearchErrCodeChannelNotMember = "search.channel_not_member"
SearchErrCodeQueryEmpty       = "search.query_empty"
SearchErrCodeQueryTooLong     = "search.query_too_long"
SearchErrCodeCrossOrgDenied   = "search.cross_org_denied"
```

Mismatch is caught by content-lock §4 two-way grep and acceptance §1.7 unit coverage.

## Cross-Milestone Byte-Identical Locks

- Shares the same five-kind enum and artifact single-source rule as CV-1 #348 / CV-3 #408 / CV-2 v2 #517 / CV-3 v2 #528 (FTS5 contentless index, no split table, existing schema unchanged).
- Keeps the same owner-only ACL design constraint as CV-1.2 #342 + CV-2 v2 + CV-3 v2 + CV-4 + AL-5 + AP-3 cross-org paths.
- Uses the AP-1 #493 `HasCapability` single source and AP-3 #521 cross-org check (the search path automatically passes through the cross-org check).
- Uses the same error-code literal single-source + content-lock two-way docs pattern as CV-2 v2 / CV-3 v2.

## Out of Scope

- v2 message / DM full-text search (left for a separate DM-4+ milestone, using messages-table FTS5).
- BM25 custom ranking / saved query / cross-channel global search / agent search
  action — left for v2+.
- elasticsearch / opensearch / typesense / meilisearch / sonic / bleve —
  excluded to keep the blueprint's SQLite single-source rule.
