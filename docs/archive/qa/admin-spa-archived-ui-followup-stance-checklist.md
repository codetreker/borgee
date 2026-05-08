# ADMIN-SPA-ARCHIVED-UI-FOLLOWUP stance checklist v1 (≤80 行)

> 野马/飞马联拟 · 2026-05-01 · v1 · post-#637 wave
> **关联**: `admin-spa-archived-ui-followup-spec.md` byte-identical / AL-8 §0 立场 ③ / ADMIN-SPA-SHAPE-FIX #633 D4-A server sanitizer
> **立场**: #633 D4-A client filter UI 漏件闭环 — server `?archived=` enum 三态已实施, client AuditLogFilters.archived + filter UI 当时未跟改

## 1. 立场 (5 项)

- [ ] **§1.1 0 server / 0 schema / 0 endpoint URL / 0 cookie 改** (post-#633 闭环非新动作): `git diff origin/main -- packages/server-go/` = 0 行; client 单边 ~10 行真补; #633 D4-A server-side sanitizer 完整保留不动
- [ ] **§1.2 client AuditLogFilters.archived union 三态 byte-identical 跟 server enum**: `archived?: 'active'|'archived'|'all'` 跟 `admin_endpoints.go::handleAdminAuditLog ?archived=` enum 字面单源, drift = 改两处 (client interface + server enum)
- [ ] **§1.3 fetchAdminAuditLog 透传 ?archived= URL param**: `if (filters.archived) qs.set('archived', filters.archived)` 真接 server query
- [ ] **§1.4 AdminAuditLogPage filter UI 加 `<select data-filter="archived">` 3 option byte-identical**: `<option value="active">Active</option>` / `<option value="archived">Archived</option>` / `<option value="all">All</option>` 三态 byte-identical 跟 server enum; `value={filters.archived ?? 'active'}` 默认 active 跟 server byte-identical
- [ ] **§1.5 #633 D4-A row 三态不破 + 反 cross-page contamination**: `data-archived-state={row.archived_at != null ? 'archived' : 'active'}` + `admin-audit-row-{active,archived}` className 当时已加, 此 PR 不动; `admin-audit-row-archived` 仅 AdminAuditLogPage 内 0 漂入 AdminApp 别 page

## 2. 黑名单 grep (反约束 5 锚)

```bash
# 1) data-filter="archived" select + 3 option byte-identical 跟 server enum
grep -nE 'data-filter="archived"' packages/client/src/admin/pages/AdminAuditLogPage.tsx  # ≥1
grep -cE '<option value="(active|archived|all)">' packages/client/src/admin/pages/AdminAuditLogPage.tsx  # = 3

# 2) AuditLogFilters union + qs.set('archived', ...)
grep -nE "archived\?:\s*'active'\s*\|\s*'archived'\s*\|\s*'all'" packages/client/src/admin/api.ts  # ≥1
grep -nE "qs\.set\(\s*'archived'" packages/client/src/admin/api.ts  # ≥1

# 3) #633 D4-A row 三态不破 (data-archived-state + admin-audit-row-{active,archived})
grep -nE "data-archived-state" packages/client/src/admin/pages/AdminAuditLogPage.tsx  # ≥1
grep -nE "admin-audit-row-(active|archived)" packages/client/src/admin/pages/AdminAuditLogPage.tsx  # ≥2

# 4) 反 cross-page contamination — admin-audit-row-archived 仅 AdminAuditLogPage 内
grep -rnE "admin-audit-row-archived" packages/client/src/admin/AdminApp.tsx  # 0 hit

# 5) 0 server / 0 schema / 0 endpoint URL / 0 cookie 改
git diff origin/main -- packages/server-go/  # 0 行
git diff origin/main -- packages/server-go/internal/migrations/  # 0 行
```

## 3. 不在范围 (跟 spec §3 留账 byte-identical)

- ❌ server `?archived=` enum 改 / sanitizeAdminAction 改 / admin_actions schema 改 (#633 保留)
- ❌ row className/data-archived-state 改 (#633 D4-A 已加)
- ❌ admin SPA 其它 page 加 archived filter (留 P3 admin-spa-ui-coverage)
- ❌ Playwright e2e (file-source vitest 守门已够; e2e 走 liema REG-ADM-05 翻牌)
- ❌ 跨端字面 (用户端 Settings/AdminActionsList 中文动词不动)

## 4. 验收挂钩 (跟 spec §1 ASAUI.1 closure byte-identical)

- [ ] **REG-ASAUI-001** AdminAuditLogPage 加 `data-filter="archived"` select + 3 option byte-identical 跟 server enum (active/archived/all)
- [ ] **REG-ASAUI-002** row 三态 data-archived-state attr + className 不破 (#633 D4-A 已加, 此 PR 反向 grep 守)
- [ ] **REG-ASAUI-003** api.ts AuditLogFilters 加 `archived?: 'active'|'archived'|'all'` union 三态
- [ ] **REG-ASAUI-004** fetchAdminAuditLog 加 `qs.set('archived', filters.archived)` URL param 透传
- [ ] **REG-ASAUI-005** admin-audit-row-archived className 不漂入 AdminApp 其它 page (反 cross-page contamination)

## 5. v0 → v1 transition + 跨 milestone 锁链

v1 直转正 (post-#637 wave 合后启动). AL-8 §0 立场 ③ + ADMIN-SPA-SHAPE-FIX #633 D4-A server sanitizer 真接 (此 PR client 闭环) + ADM-2 #484 + ADM-2-FOLLOWUP #626 byte-identical + ADM-0 §1.3 红线

| 2026-05-01 | 飞马/野马 | v1 stance — #633 D4-A client filter UI 闭环. 5 立场 + 5 黑名单 grep + 5 REG-ASAUI. |
