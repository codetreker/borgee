# ADMIN-SPA-ARCHIVED-UI-FOLLOWUP spec brief — #633 D4-A client AdminAuditLogPage filter UI 兑现 (≤80 行)

> 飞马 · 2026-05-01 · v1 · post-#637 wave · zhanma-e impl
> **关联**: AL-8 §0 立场 ③ archived 三态 / ADM-2 #484 admin god-mode / ADM-2-FOLLOWUP #626 byte-identical / ADMIN-SPA-SHAPE-FIX #633 D4-A server sanitizer 真兑现
> **命名**: ADMIN-SPA-ARCHIVED-UI-FOLLOWUP = #633 D4-A 漏件 — server `?archived=` enum 三态已实施, client filter UI 与 AuditLogFilters.archived 字段当时未跟改, 此 PR 闭环.

> ⚠️ 🔴 **1 真漏件**:
> - #633 D4-A 加了 `sanitizeAdminAction` server-side `archived_at` surface + 加了 row `data-archived-state` + className 三态, **但 client 没加 filter UI** — `?archived=` URL param 真接 server query, server enum 仅 client 0 surface 等价无效. REG-ADM-05 e2e 真凿实 `grep archived AdminAuditLogPage.tsx` 当时 0 hit (filter UI 0 surface).

## 0. 关键约束 (3 条立场)

1. **0 server / 0 schema / 0 endpoint URL 改** (post-#633 闭环非新动作): client 单边 ~10 行真补; `git diff origin/main -- packages/server-go/` = 0 hit. 反约束: 0 migration / 0 routes / 0 cookie / 0 admin gate 改; #633 D4-A server-side sanitizer 完整保留不动.
2. **client 改最小补丁 + 3 option byte-identical 跟 server enum**: AuditLogFilters 加 `archived?: 'active'|'archived'|'all'`; AdminAuditLogPage 加 `<select data-filter="archived">` 3 option (`active` / `archived` / `all`) byte-identical 跟 server `admin_endpoints.go::handleAdminAuditLog ?archived=` enum 字面单源 (drift = 改两处). 反约束: row className/data-archived-state 不破 (#633 D4-A 已加, 此 PR 不动).
3. **vitest file-source content lock 5 case 守门**: `admin-spa-archived-ui-followup.test.ts` createRequire pattern (跟 admin-api-shape.test.ts 同模式), REG-ASAUI-001..005: select 3 option / row 三态不破 / interface union / qs.set / 反 cross-page contamination.

## 1. 拆段实施 (1 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **ASAUI.1 client filter UI 真补 (≤10 行 production + 5 vitest)** | `api.ts::AuditLogFilters` 加 `archived?: 'active'\|'archived'\|'all'` union; `fetchAdminAuditLog` 加 `if (filters.archived) qs.set('archived', filters.archived)`; `AdminAuditLogPage.tsx` 加 `<label>View<select data-filter="archived">3 option</select></label>` 在 Target User ID 之后; 加 `admin-spa-archived-ui-followup.test.ts` REG-ASAUI-001..005 file-source content lock |

## 2. 反向 grep 锚 (5 反约束)

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

## 3. 不在范围 (留账)

- ❌ server `?archived=` enum 改 / server sanitizeAdminAction 改 / server admin_actions schema 改 (#633 已实施保留不动)
- ❌ row className/data-archived-state 改 (#633 D4-A 已加, 此 PR 0 改)
- ❌ admin SPA 其它 page 加 archived filter (留 P3 admin-spa-ui-coverage)
- ❌ Playwright e2e (file-source vitest 守门已够; e2e 留 liema 跑 REG-ADM-05 翻牌)

## 4. 跨 milestone byte-identical 锁

AL-8 §0 立场 ③ archived 三态 + ADMIN-SPA-SHAPE-FIX #633 D4-A server sanitizer 真接 (此 PR client 闭环) + ADM-2 #484 admin god-mode + ADM-2-FOLLOWUP #626 AdminAuditLogPage data-attr 不破 + ADM-0 §1.3 红线 (admin/user 路径分叉 — 此 PR 仅 admin SPA)
