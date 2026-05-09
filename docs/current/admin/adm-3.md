# ADM-3 v1 — admin SPA multi-source audit page (≤40 行)

> 落地: PR #619 feat/adm-3 · MultiSourceAuditPage 4 source badge + filter dropdown
> 蓝图锚: admin-model.md §1.4 来源透明 (人/agent/admin/混合)
> server 侧 [`docs/current/server/adm-3.md`](../server/adm-3.md) 同源 (4 source enum byte-identical)

## 1. 文件清单 (admin SPA)

| 文件 | 行 | 角色 |
|---|---|---|
| `packages/client/src/admin/api.ts` 扩 | +35 | AUDIT_SOURCES 4-tuple SSOT + AuditSource type + MultiSourceAuditRow + fetchMultiSourceAudit |
| `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` | 150 | 4 source badge + filter dropdown + table view + DOM 锚 |
| `packages/client/src/admin/AdminApp.tsx` 扩 | +3 | nav 加 `/admin/audit-multi-source` route |
| `packages/client/src/__tests__/MultiSourceAuditPage.test.tsx` | 130 | 7 vitest (DOM 锚 + filter + 反同义词 reject) |

## 2. 4 source enum SSOT (跨层锁)

`AUDIT_SOURCES = ['server', 'plugin', 'host_bridge', 'agent']` byte-identical 跟 server const + i18n SOURCE_LABEL 三处. 改 = 改三处.

`SOURCE_LABEL`: `Server / Plugin / Host Bridge / Agent` byte-identical (反同义词 `hybrid / combined / multi_source / mixed_actor` 0 hit, vitest 反向断言守).

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
