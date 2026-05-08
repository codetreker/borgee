# ADMIN-SPA-ARCHIVED-UI-FOLLOWUP content-lock v1 (≤40 行)

> 飞马/野马 · 2026-05-01 · v1 · 锚 spec brief §0..§4 + stance §1.1-§1.5 byte-identical
> 范围: client AdminAuditLogPage filter UI 段 (#633 D4-A 漏件闭环). UI 真补, 截屏非强制.

## §1 AdminAuditLogPage filter UI 字面 byte-identical

- `<label>View<select data-filter="archived">...</select></label>` 加在 Target User ID 之后, Filter button 之前
- `<option value="active">Active</option>` byte-identical 跟 server enum 同源
- `<option value="archived">Archived</option>` byte-identical 跟 server enum 同源
- `<option value="all">All</option>` byte-identical 跟 server enum 同源
- `value={filters.archived ?? 'active'}` 默认 active 跟 server byte-identical (反同义词 "default" / "blank" / "none")

## §2 AuditLogFilters interface SSOT

- `archived?: 'active' | 'archived' | 'all'` union 三态 byte-identical 跟 server `admin_endpoints.go::handleAdminAuditLog ?archived=` enum 字面单源
- `qs.set('archived', filters.archived)` URL param 名 byte-identical (反同义词 `archive` / `view` / `state`)

## §3 反约束 (黑名单 grep, 5 锚)

```bash
# 1) 反同义词 label 字面 (反 "View State" / "Filter Archived" / "Show:")
grep -nE '>\s*View\s*<' packages/client/src/admin/pages/AdminAuditLogPage.tsx  # ≥1 (label "View")
grep -nE 'View State|Filter Archived|Show:|视图' packages/client/src/admin/pages/AdminAuditLogPage.tsx  # 0 hit

# 2) 3 option value 字面 byte-identical 跟 server enum
grep -cE '<option value="(active|archived|all)">' packages/client/src/admin/pages/AdminAuditLogPage.tsx  # = 3

# 3) 反 URL param 名漂 (qs.set 'archived' 字面单源)
grep -nE "qs\.set\(\s*'(archive|view|state|filter|status)'" packages/client/src/admin/api.ts  # 0 hit (除 'archived')
grep -nE "qs\.set\(\s*'archived'" packages/client/src/admin/api.ts  # ≥1 hit

# 4) 反 union 漂 (interface 字面 byte-identical 跟 server)
grep -nE "archived\?:\s*string" packages/client/src/admin/api.ts  # 0 hit (反 string 漂)
grep -nE "archived\?:\s*'active'\s*\|\s*'archived'\s*\|\s*'all'" packages/client/src/admin/api.ts  # ≥1 hit

# 5) 反 cross-page contamination (admin-audit-row-archived 仅 AdminAuditLogPage 内)
grep -rnE 'admin-audit-row-archived' packages/client/src/admin/AdminApp.tsx  # 0 hit
grep -rnE 'admin-audit-row-archived' packages/client/src/admin/pages/  # 仅 AdminAuditLogPage.tsx
```

## §4 demo 截屏 (非强制 — 真补 UI, 5 vitest file-source 守门已够) + 跨 milestone 锁链

`docs/evidence/admin-spa-archived-ui-followup/audit-log-filter.png` 预备 (走 liema REG-ADM-05 翻牌时同步留证). AL-8 §0 立场 ③ + ADMIN-SPA-SHAPE-FIX #633 D4-A server sanitizer 真接 + ADM-2-FOLLOWUP #626 byte-identical 不破 + ADM-0 §1.3 红线 + spec/stance byte-identical.

| 2026-05-01 | 飞马/野马 | v1 content-lock — `View` label + 3 option (Active/Archived/All) + URL param `archived` 字面单源 + 5 黑名单 grep + 截屏非强制. #633 D4-A client filter UI 闭环全锁. |
