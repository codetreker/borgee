# DM Search — cross-DM message search REST endpoint

> **Canonical pointer.** Code at
> `packages/server-go/internal/api/message_search.go` (handler) +
> `packages/server-go/internal/store/dm_11_search_queries.go` (store
> helper). Route registration at server boot in
> `packages/server-go/internal/server/server.go`.
> Spec brief at
> [`docs/implementation/modules/dm-11-spec.md`](../../implementation/modules/dm-11-spec.md).
> Content-lock at [`docs/qa/dm-11-content-lock.md`](../../qa/dm-11-content-lock.md).

## Why

Blueprint
[`channel-model.md`](../../blueprint/current/channel-model.md) §1.2 + §3.2
mark DM as a concept that **reuses the channel table** (`type='dm'`) but
must stay UX-isolated. DM-11 adds user-scoped cross-DM message
search via `LIKE %q%` over `messages.content`. The implementation
reuses the `messages.content` column (no new schema) and stays scoped
to DM-only channels with channel-member ACL — admin-wide access is
permanently off (蓝图 §1.2 + ADM-0 §1.3 红线).

## Principles (blueprint channel-model.md §1.2 + §3.2 source text)

- **No schema change** — reuse `messages.content` + LIKE %q% (same pattern as
  messages search #467 + CHN-13 channel search #583). FTS5 already exists in
  the CV-6 #531 `artifacts_fts` table but is **not reused** here; leave it for
  v2 to avoid cross-table join complexity.
- **DM-only scope** — store helper `SearchDMMessages` JOIN
  `channels ON c.type='dm'` enforces filtering and prevents cross-channel leaks
  (same design constraint as DM-10 #597 + dm_4_message_edit.go #549 DM-only path).
- **channel-member ACL reuses the AP-4 + AP-5 pattern** — store helper JOIN
  `channel_members ON cm.user_id = caller` prevents cross-user DM leaks and
  follows AP-4 #551 reactions ACL + AP-5 #555 messages ACL.
- **q query param DoS guard** — q trim + min 2 char + max 200 char;
  3 literal error-code checks; limit clamp default 30 / max 50.
- **admin-wide cross-user search is permanently unregistered** — grep check
  `admin.*dm.*search|/admin-api/.*dm/search` in `admin*.go` 0 hit
  (ADM-0 §1.3 boundary; cross-user DM search does not register an admin route,
  matching the DM-10 + DM-7 edit-history admin-wide access boundary).
- **Exclude deleted_at IS NOT NULL rows** — `maskDeletedMessages` helper
  enforces this to prevent deleted-message leaks.

## Endpoint

| Method | Path                              | Purpose                         | ACL                  |
|--------|-----------------------------------|---------------------------------|----------------------|
| GET    | `/api/v1/dm/search?q=<q>&limit=N` | Cross-DM message search         | authenticated user (auth middleware) |

Query params:

| Param   | Required | Validation                                                                      |
|---------|----------|---------------------------------------------------------------------------------|
| `q`     | yes      | trim → `2 ≤ len ≤ 200` (`dm11MinQueryLen` / `dm11MaxQueryLen` const)            |
| `limit` | no       | default 30, clamped to max 50 (`dm11DefaultLimit` / `dm11MaxLimit` const)       |

## Response body

```
200 {"messages": [...], "count": N}
400 {"code": "dm_search.q_required"}    # q 缺失或全空
400 {"code": "dm_search.q_too_short"}   # len(q) < 2
400 {"code": "dm_search.q_too_long"}    # len(q) > 200
401 "Unauthorized"
500 (search failure, no body code)
```

## Error code byte-identical (3 literals)

| Error code                 | HTTP | Trigger               | content-lock §1 |
|----------------------------|------|-----------------------|-----------------|
| `dm_search.q_required`     | 400  | q missing or blank     | ✅              |
| `dm_search.q_too_short`    | 400  | `len(q) < 2`          | ✅              |
| `dm_search.q_too_long`     | 400  | `len(q) > 200`        | ✅              |

Literals are locked with `internal/api/message_search.go`; changing them
requires updating two places (handler + content-lock §1).

## Validation order (handler)

1. Auth — `mustUser(w, r)` (auth middleware returns 401).
2. q query param required + 2..200 char (DoS guard; avoid empty-query full table scans).
3. limit clamp default 30 / max 50.
4. `Store.SearchDMMessages(user.ID, q, limit)` — DM-only + channel-
   member ACL JOIN.

## Reverse-grep checks (required for the DM-11 implementation PR)

```
git grep -nE 'dm_search_index|dm_search_table|dm_11_search_log' packages/server-go/internal/  # 0 hit (messages.content 列)
git grep -nE 'admin.*dm.*search|/admin-api/.*dm/search'         packages/server-go/internal/api/admin*.go  # 0 hit (ADM-0 §1.3)
git grep -nE 'fts5|MATCH.*dm_search|VIRTUAL TABLE.*dm'          packages/server-go/internal/  # 0 hit (FTS5 不走留 v2)
git diff origin/main -- packages/server-go/internal/migrations/ | grep -c '^\+'                  # 0 production 行
```

## Tests

- `internal/api/dm11search_test.go` — 10 unit tests (success /
  q-required / q-too-short / q-too-long / unauthorized / no results /
  dm-only-excludes-public / non-member-no-leak / limit-clamp /
  deleted messages hidden).
- `internal/store/dm_11_search_queries_test.go` (sibling) — store
  helper level: DM-only JOIN + channel-member ACL + ORDER BY
  `created_at DESC` + maskDeletedMessages.

Regression rows: `REG-DM11-001..006` in
[`docs/qa/regression-registry.md`](../../qa/regression-registry.md).

## Not in scope (remaining items)

- ❌ FTS5 via the `artifacts_fts` pattern — leave for v2, after DM message volume
  warrants revisiting cross-table join complexity.
- ❌ Sort by relevance — leave for v2. Current `ORDER BY created_at DESC` remains
  the shared ordering and keeps the same design constraint as messages search #467.
- ❌ Admin-wide cross-user search — permanently unregistered (ADM-0 §1.3 boundary).
- ❌ Cross-org search — leave for the AP-3 timeframe, reusing existing store.CrossOrg.
- ❌ Search history persistence — 留 v3.
- ❌ Per-DM channel filter (`?channel_id=`) — leave for v2; current version searches
  across all of the user's DMs.
- ❌ Client UI (DM search bar / dropdown) — leave for a follow-up PR after the
  server contract is fixed; content-lock §4 lists suggested literals.
