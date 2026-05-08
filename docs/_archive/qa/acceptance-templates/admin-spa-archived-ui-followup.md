# Acceptance Template — ADMIN-SPA-ARCHIVED-UI-FOLLOWUP (#633 D4-A client filter UI 闭环)

> Spec brief `admin-spa-archived-ui-followup-spec.md` (飞马 v1). Owner: 战马E 实施 / 飞马 review / 烈马 验收.
>
> **范围**: #633 D4-A 漏件 — server `?archived=` enum 三态已实施, client AdminAuditLogPage filter UI + AuditLogFilters.archived 字段闭环. client diff ≤10 行 production; 0 server / 0 schema / 0 endpoint URL / 0 cookie 改.

## 验收清单

### §1 行为不变量 (1 真漏件闭环)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 AdminAuditLogPage 加 `<select data-filter="archived">` 3 option byte-identical 跟 server enum (active/archived/all) | unit | `admin-spa-archived-ui-followup.test.ts::REG-ASAUI-001` PASS |
| 1.2 #633 D4-A row 三态 `data-archived-state` + `admin-audit-row-{active,archived}` className 不破 | unit | `_002` PASS (反向 grep 守) |
| 1.3 api.ts AuditLogFilters 加 `archived?: 'active'\|'archived'\|'all'` union 三态 byte-identical 跟 server enum | unit | `_003` PASS |
| 1.4 fetchAdminAuditLog 加 `qs.set('archived', filters.archived)` URL param 透传 server query | unit | `_004` PASS |
| 1.5 `admin-audit-row-archived` className 不漂入 AdminApp 其它 page (反 cross-page contamination) | grep | `_005` PASS |

### §2 数据契约 (0 server / 0 schema 改)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 client diff ≤10 行 production code (api.ts ~3 行 + AdminAuditLogPage.tsx ~17 行 select markup) | git diff | `git diff origin/main -- packages/client/src/admin/` ≤25 行 |
| 2.2 0 server 改 / 0 schema migration / 0 endpoint URL / 0 routes / 0 cookie 改 | grep | `git diff origin/main -- packages/server-go/` = 0 行 |
| 2.3 #633 D4-A server-side sanitizer (admin_endpoints.go) byte-identical 不动 | inspect | server diff 0 行 |

### §3 closure (REG + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 vitest 全绿不破 + 5 vitest 真测 | full test | 107/107 vitest PASS (含 5 新 REG-ASAUI) |
| 3.2 立场承袭 — AL-8 §0 立场 ③ archived 三态 + ADMIN-SPA-SHAPE-FIX #633 D4-A server sanitizer 真接 + ADM-2 #484 + ADM-2-FOLLOWUP #626 byte-identical + ADM-0 §1.3 admin/user 路径分叉 | inspect | spec §0+§4 byte-identical |
| 3.3 e2e-scenarios.md REG-ADM-05 翻牌 ✅ done (§3 总数表 + §3 v3 变更日志 + §4.2 闭环 + §5 退出条件) | inspect | docs/qa/e2e-scenarios.md diff 4 处 |

## REG-ASAUI-* (initial ⚪ → 🟢)

- REG-ASAUI-001 🟢 AdminAuditLogPage `data-filter="archived"` select + 3 option byte-identical 跟 server enum
- REG-ASAUI-002 🟢 #633 D4-A row 三态 data-archived-state + className 不破 (反向 grep)
- REG-ASAUI-003 🟢 api.ts AuditLogFilters `archived?` union 三态
- REG-ASAUI-004 🟢 fetchAdminAuditLog `qs.set('archived', ...)` URL param 透传
- REG-ASAUI-005 🟢 admin-audit-row-archived className 不漂入 AdminApp (反 cross-page contamination)

## 退出条件

- §1 (5) + §2 (3) + §3 (3) 全绿 — 一票否决
- 0 server / 0 schema / 0 endpoint URL / 0 cookie 改
- 5 vitest file-source content lock 真测
- REG-ADM-05 翻牌 ✅ done 同 PR 提交
- 登记 REG-ASAUI-001..005

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v1 — acceptance template. 立场承袭 AL-8 §0 立场 ③ + #633 D4-A server sanitizer 真接 (此 PR client 闭环) + ADM-2 #484 + ADM-2-FOLLOWUP #626 byte-identical + ADM-0 §1.3 红线. |
| 2026-05-01 | 战马E | v1 实施 — client api.ts AuditLogFilters.archived union + qs.set + AdminAuditLogPage `<select data-filter="archived">` 3 option + 5 vitest file-source content lock. 107/107 vitest PASS. server 0 改. REG-ADM-05 翻牌 ✅ done. REG-ASAUI-001..005 ⚪→🟢. |
