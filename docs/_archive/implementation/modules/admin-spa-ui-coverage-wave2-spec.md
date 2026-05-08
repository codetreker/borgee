# admin-spa-ui-coverage-wave2 spec brief — 第二波 4 endpoint UI (≤80 行)

> 战马C · 2026-05-02 · post-#639 第一波合后 wave 2; 4 个 server 已挂 endpoint 但 admin SPA UI 缺
> 关联: ADM-0 §1.3 红线 / admin-spa-ui-coverage 第一波 #639 / AL-4.2 runtimes / HB-5 heartbeat-lag / CHN-5 archived / CHN-14 description-history

> ⚠️ Client-only milestone — **0 server / 0 endpoint / 0 schema / 0 routes 改** + ≤350 行 client 接 UI to existing server endpoint. 4 endpoint 真兑现 admin god-mode UI 第二波.

## 0. 关键约束 (3 条立场)

1. **0 server 改 + 复用 existing server endpoint** — 4 endpoint server 已挂:
   - GET `/admin-api/v1/runtimes` (runtimes.go:538) — 全 agent_runtimes 列表 (admin god-mode metadata read, 不含 last_error_reason 隐私 ADM-0 §1.3)
   - GET `/admin-api/v1/heartbeat-lag` (host_lag.go:52) — LagSnapshot {count, p50/p95/p99_ms, threshold_ms, at_risk, sampled_at, window_seconds, reason_if_at_risk?}
   - GET `/admin-api/v1/channels/archived` (channel_archived.go:44) — ChannelWithCounts[] 全 org archived
   - GET `/admin-api/v1/channels/{channelId}/description/history` (channel_history.go:48) — `[{old_content, ts, reason}]`
   反约束: `git diff origin/main -- packages/server-go/` 0 行
2. **admin god-mode 路径独立 (ADM-0 §1.3 红线)** — 4 page 仅访问 `/admin-api/*`, 不串 user-rail (`/api/v1/`) + 不 import user-rail `lib/api`. 反向 grep `fetch.*'/api/v1/'` + `from '../../lib/api'` 0 hit
3. **shape byte-identical 跟 server SSOT** — TS interface 字段名+类型跟 server JSON struct tag byte-identical (改 = 改两处)

## 1. 拆段实施 (3 段一 PR)

| 段 | 范围 |
|---|---|
| **ASUC2.1 api.ts 扩 helper (≤80 行)** | 4 interface (`AdminRuntime` / `LagSnapshot` / `AdminArchivedChannel` reuse `AdminChannel`+archived_at / `ChannelDescriptionHistoryEntry`) + 4 fetch helper (`fetchAdminRuntimes` / `fetchAdminHeartbeatLag` / `fetchAdminArchivedChannels` / `fetchAdminChannelDescriptionHistory`) byte-identical 跟 server endpoint shape |
| **ASUC2.2 4 page (≤270 行)** | RuntimesPage + HeartbeatLagPage + ArchivedChannelsPage + ChannelDescriptionHistoryPage 各 ≤70 行; 4 nav 入口加 sidebar; 12+ DOM 锚 `data-asuc2-*` byte-identical (跟 #639 `data-asuc-*` + ADMIN-SPA-SHAPE-FIX `data-asf-*` 模式承袭) |
| **ASUC2.3 vitest + 4 件套 + closure** | `admin-spa-ui-coverage-wave2.test.tsx` 7 case (4 api shape + 4 DOM 锚 + 中文文案 + 路径独立 + 立场承袭); REG-ASUC2-001..007 ⚪→🟢 + acceptance + 4 件套 |

## 2. 反向 grep 锚 (5 反约束)

```bash
# 1) 0 server 改
git diff origin/main -- packages/server-go/  # 0 行

# 2) admin god-mode 路径独立
grep -E "fetch\(['\"]\/api\/v1" packages/client/src/admin/pages/{Runtimes,HeartbeatLag,ArchivedChannels,ChannelDescriptionHistory}Page.tsx  # 0 hit
grep -E "from ['\"]\.\.\/\.\.\/lib\/api['\"]" packages/client/src/admin/pages/{Runtimes,HeartbeatLag,ArchivedChannels,ChannelDescriptionHistory}Page.tsx  # 0 hit

# 3) DOM 锚 byte-identical
grep -cE 'data-asuc2-' packages/client/src/admin/pages/{Runtimes,HeartbeatLag,ArchivedChannels,ChannelDescriptionHistory}Page.tsx  # ≥12 hit

# 4) 既有 vitest 不破
pnpm vitest run --testTimeout=10000  # ALL PASS

# 5) shape SSOT byte-identical (LagSnapshot 9 字段 / runtimeRow 7 字段 / history entry 3 字段)
grep -cE "p50_ms|p95_ms|p99_ms|at_risk|threshold_ms" packages/client/src/admin/api.ts  # ≥5 hit
```

## 3. 不在范围 (留账)

- ❌ POST/PATCH/DELETE on these 4 endpoint (server 都 readonly, ADM-0 §1.3 admin god-mode 不直接改)
- ❌ heartbeat-lag 历史 timeline / chart visualization (留 v2 admin observability milestone)
- ❌ archived channel 真 unarchive 入口 (admin god-mode readonly, owner 走 user-rail)
- ❌ description-history diff 视图 (高级 viz, 留 v2)

## 4. 跨 milestone byte-identical 锁链

- admin-spa-ui-coverage 第一波 #639 — `data-asuc-*` 模式承袭 → `data-asuc2-*`
- AL-4.2 #398 — agent_runtimes table SSOT (id/agent_id/endpoint_url/process_kind/status)
- HB-5 #408 — host_lag rolling window LagSnapshot SSOT
- CHN-5 #189 — archived channels admin god-mode readonly
- CHN-14 #429 — description_edit_history JSON 数组 schema
- ADM-0 §1.3 — admin/user 路径分叉红线 (admin SPA 仅 /admin-api/* 走)
