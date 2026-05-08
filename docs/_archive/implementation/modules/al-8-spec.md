# AL-8 spec brief — audit log query API filter 扩 (战马D v0)

> Phase 6 audit retention 互补 — admin 看 active vs archived audit; 既有
> GET /admin-api/v1/audit-log filter 加 since/until/archived/actions, **0
> schema / 0 新 endpoint** (复用 AL-7 #533 archived_at + ADM-2.2 既有 path).
> 跟 AL-7 一对 —— 写戳 (AL-7 archive) + 读视 (AL-8 query). admin-rail only,
> client SPA dashboard 留 v3 (跟 AL-7 同精神).

## §0 立场 (3 + 4 边界)

- **①** 不另起 endpoint — GET /admin-api/v1/audit-log 路径单源, ADM-2.2
  既有 (3-filter actor_id/action/target_user_id) 字面 byte-identical 不动,
  加 since/until/archived/actions 4 filter additive (反向 grep `audit-log/
  query\|audit-log/search\|admin-api/v1/audit/.*` 0 hit).
- **②** admin-rail only — ADM-0 §1.3 红线 (反向 grep `/api/v1/.*audit-log`
  user-rail 0 hit, 跟 ADM-2.2 立场承袭).
- **③** archived filter 三态 — `active` (archived_at IS NULL, 默认) /
  `archived` (archived_at IS NOT NULL) / `all` (无 WHERE); 反向 reject
  spec 外值 → 400 `audit_log.archived_view_invalid`. 跟 AL-7 archived_at
  字段 sparse idx 同源 (sparse idx 走 archived 视图, 现网零开销).

边界:
- **④** since/until int64 ms epoch — clamp 反 negative / non-int → 400
  `audit_log.time_range_invalid`; since>until → 400 `audit_log.time_range_
  inverted` (反 0/负/字符串/反向区间 4 case).
- **⑤** actions 多值 — `?action=a&action=b` 走 r.URL.Query()["action"]
  Slice; 单值 backward-compat (ADM-2.2 既有 ?action=foo 字面 byte-identical),
  无值默认全可见.
- **⑥** limit clamp 跟 ADM-2.2 既有 default 100/max 500 字面单源 (parseLimit
  helper byte-identical 不动).
- **⑦** AST 锁链延伸第 8 处 forbidden 3 token (`pendingAuditQuery /
  auditQueryRetryQueue / deadLetterAuditQuery`) 在 internal/auth +
  internal/api production 0 hit (跟 BPP-4/5/6/7/8/HB-3 v2/AL-7 同模式).

## §1 拆段

**AL-8.1 — schema 0 行**: 复用 admin_actions + AL-7.1 archived_at 列 +
sparse idx. 反向 grep `ALTER TABLE admin_actions\|CREATE INDEX.*audit_log
` 在 internal/migrations/ 0 hit. registry.go 不动.

**AL-8.2 — server filter 扩**:
- store: AdminActionListFilters 加 Since *int64 / Until *int64 /
  ArchivedView string / Actions []string (既有 3 字段不动); ListAdminActions
  ForAdmin Where 链按 nil-safe 加 (since/until BETWEEN; archived 三态 switch;
  actions IN slice). 既有 ListAdminActionsForTargetUser 不动 (user-rail 不
  漂; 不挂 archived filter — user 只见自己, 立场 ⑤).
- api: ADM2Handler.handleAdminAuditLog 加 query parse — sinceMs / untilMs
  int64 + archivedView string + actions []string; 反向校验 (400 错码 byte-
  identical 跟 §0 立场 ③④); 既有 3 filter byte-identical 不动.
- 4 unit + 3 反约束 unit (active 视图默认 / archived 视图 archived_at IS
  NOT NULL / since-until 区间 / 多 action / time_range_invalid 反断 / archived_
  view_invalid 反断 / user-rail 0 挂).

**AL-8.3 — client + closure**: client SPA admin dashboard 留 v3 (跟 AL-7
同精神). REG-AL8-001..006 6 🟢 + AST scan 反向 + audit 5 字段链第 8 处.

## §2 留账 (v3+)

- admin SPA dashboard list 视图 + filter form (留 v3).
- audit log export CSV / NDJSON (留 v3).
- archived row search by metadata JSON path (留 v4).
- audit log Prometheus metric exporter (留 v3).

## §3 反约束 grep 锚

- 不开 user-rail variant: `/api/v1/.*audit-log` user-rail 0 hit.
- 不另起 endpoint: `audit-log/query\|audit-log/search\|/admin-api/v1/audit/` 0 hit (除 ADM-2.2 单源).
- 不裂表: `audit_query_table\|audit_log_index\|al8_query_log` 0 hit.
- AST 锁链延伸第 8 处 forbidden 3 token 0 hit.
- audit 5 字段链第 8 处不漂 — admin_actions 字段集不动, 仅 query filter 加.

## §4 不在范围

- admin SPA dashboard UI (留 v3) / audit export CSV/NDJSON (留 v3).
- audit metadata JSON path search (留 v4) / Prometheus exporter (留 v3).
- audit cross-org filter (留 v3 跟 AP-3 同期).
- user-rail audit search (永久不挂 ADM-0 §1.3 红线).
- audit row write/update/delete (forward-only AL-7 锁).
- audit retention runtime hot-mutate (留 v3).
