# ADM-3 v1 — admin SPA multi-source audit page (≤40 行)

> 落地: PR #619 feat/adm-3 · MultiSourceAuditPage 4 source badge + filter dropdown
> 蓝图锚: admin-model.md §1.4 来源透明 (人/agent/admin/混合)
> server 侧 [`docs/current/server/adm-3.md`](../server/adm-3.md) 保持一致 (4 source enum 字面值一致)

## 1. 文件清单 (admin SPA)

| 文件 | 行 | 角色 |
|---|---|---|
| `packages/client/src/admin/api.ts` 扩 | +35 | AUDIT_SOURCES 4-tuple 作为 source 枚举基准 + AuditSource type + MultiSourceAuditRow + fetchMultiSourceAudit |
| `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` | 150 | 4 source badge + filter dropdown + table view + DOM 标记 |
| `packages/client/src/admin/AdminApp.tsx` 扩 | +3 | nav 加 `/admin/audit-multi-source` route |
| `packages/client/src/__tests__/MultiSourceAuditPage.test.tsx` | 130 | 7 vitest (DOM 标记 + filter + 禁用同义词 reject) |

## 2. 4 source enum 单一来源 (跨层对齐)

`AUDIT_SOURCES = ['server', 'plugin', 'host_bridge', 'agent']` 与 server const + i18n SOURCE_LABEL 三处字面值一致. 改 = 改三处.

`SOURCE_LABEL`: `Server / Plugin / Host Bridge / Agent` 字面值一致 (禁用同义词 `hybrid / combined / multi_source / mixed_actor` 保持 0 hit, 并由 vitest negative assertion 覆盖).

## 3. DOM 锚

- `[data-page="admin-audit-multi-source"]` 页根
- `[data-filter="source"]` filter dropdown
- `[data-source-row]` 每行 (值=4 source 之一)
- `.audit-source-badge.audit-source-{server,plugin,host-bridge,agent}` badge class

## 4. 跟蓝图字面一致 (改一处=改两处)

- admin god-mode 路径独立 — 仅 `/admin-api/v1/audit/multi-source` (不接 user-rail 漂, 跟 ADM-0 §1.3 红线一致)
- 4 source filter dropdown (default = "All sources")
- 表格 ts DESC + actor / action / payload (audit forward-only readonly view)

## 5. 留账

- HB-1 host_bridge audit 表未落 v1, 该 source row 0 行 (留 HB-1 后续真接)
- audit FTS / sort by relevance / pagination cursor 全留 v3+
- 跨 source 反向追溯链 留 v3+
