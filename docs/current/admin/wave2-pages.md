# Admin SPA — Wave 2 4 page (admin-spa-ui-coverage-wave2)

> 2026-05-02 · admin-spa-ui-coverage-wave2 milestone (战马C). 一个 milestone 一个 PR. 0 server / 0 endpoint / 0 schema 改, 仅 client ≤350 行 接 既有 server endpoint UI.

## 0. 设计沿用

- **ADM-0 §1.3 admin god-mode 路径独立** — 4 page 仅访问 `/admin-api/*`, 不串 user-rail
- **shape 与 server response 保持一致** — TS interface 字段名+类型跟 server JSON struct tag 保持一致 (改 = 改两处)
- **admin-spa-ui-coverage 第一波 #639** — `data-asuc-*` 模式沿用 → `data-asuc2-*`
- **readonly admin god-mode** — 4 page 仅 GET, 0 mutation (server 都 readonly)

## 1. 文件 + 范围

| 文件 | 改动 |
|---|---|
| `packages/client/src/admin/api.ts` | 加 4 interface (`AdminRuntime` / `LagSnapshot` / `AdminArchivedChannel` / `ChannelDescriptionHistoryEntry`) + 4 fetch helper (`fetchAdminRuntimes` / `fetchAdminHeartbeatLag` / `fetchAdminArchivedChannels` / `fetchAdminChannelDescriptionHistory`) |
| `packages/client/src/admin/pages/RuntimesPage.tsx` | 全 agent_runtimes 列表 (admin god-mode metadata read; last_error_reason 不展示, 隐私 ADM-0 §1.3) |
| `packages/client/src/admin/pages/HeartbeatLagPage.tsx` | LagSnapshot 9 字段 (count/p50/p95/p99/threshold/at_risk/sampled_at/window_seconds + reason_if_at_risk?) |
| `packages/client/src/admin/pages/ArchivedChannelsPage.tsx` | 全 org archived 列表 + 行内 "查看" 跳描述历史页 |
| `packages/client/src/admin/pages/ChannelDescriptionHistoryPage.tsx` | description_edit_history JSON `[{old_content, ts, reason}]` 渲染 |
| `packages/client/src/admin/AdminApp.tsx` | 4 Route 挂 (runtimes / heartbeat-lag / channels-archived / channels/:id/description-history) + 3 nav 入口 |

## 2. server endpoint (既有, 不动)

| Method | Path | Handler | shape |
|---|---|---|---|
| GET | `/admin-api/v1/runtimes` | `AdminRuntimeHandler.handleListRuntimes` (runtimes.go:538) | `{runtimes: AdminRuntime[]}` (last_error_reason OMITTED) |
| GET | `/admin-api/v1/heartbeat-lag` | `HostLagHandler.handleGet` (host_lag.go:52) | `LagSnapshot` (9 字段) |
| GET | `/admin-api/v1/channels/archived` | `ChannelHandler.handleAdminListArchivedChannels` (channel_archived.go:44) | `{channels: ChannelWithCounts[]}` |
| GET | `/admin-api/v1/channels/{id}/description/history` | `ChannelDescriptionHistoryHandler.handleAdminGet` (channel_history.go:48) | `{history: [{old_content, ts, reason}]}` |

## 3. 中文 UI 文案 (≥10 字面 byte-identical, content-lock §1.4)

运行时 / 心跳滞后 / 已归档频道 / 描述变更历史 / 样本数 / 阈值 / 归档时间 / 暂无运行时 / 暂无归档频道 / 暂无变更历史 / 刷新 / 返回 / 查看

## 4. DOM 锚 (≥12 hit, data-asuc2-*)

- `data-page="admin-runtimes" / "admin-heartbeat-lag" / "admin-archived-channels" / "admin-channel-description-history"` (4)
- `data-asuc2-runtimes-list / -runtime-row / -runtimes-refresh` (3)
- `data-asuc2-lag-card / -lag-refresh / -lag-reason?` (2-3)
- `data-asuc2-archived-list / -archived-row / -archived-refresh / -history-link` (4)
- `data-asuc2-history-list / -history-row` (2)

## 5. grep 检查项 (REG-ASUC2-001..007)

```bash
# REG-ASUC2-005 — admin god-mode 路径独立 (ADM-0 §1.3 红线)
grep -E "fetch\(['\"]\/api\/v1" packages/client/src/admin/pages/{Runtimes,HeartbeatLag,ArchivedChannels,ChannelDescriptionHistory}Page.tsx  # 0 hit
grep -E "from ['\"]\.\.\/\.\.\/lib\/api['\"]" packages/client/src/admin/pages/{Runtimes,HeartbeatLag,ArchivedChannels,ChannelDescriptionHistory}Page.tsx  # 0 hit

# REG-ASUC2-002 — shape 单一来源 字段 byte-identical
grep -cE "p50_ms|p95_ms|p99_ms|threshold_ms|at_risk" packages/client/src/admin/api.ts  # ≥5 hit
```

## 6. 不在范围

- POST/PATCH/DELETE on these 4 endpoint (server 都 readonly, ADM-0 §1.3 admin god-mode 不直接改)
- heartbeat-lag chart / timeline (留 v2 admin observability)
- archived channel 真 unarchive 入口 (admin readonly, owner 走 user-rail)
- description-history diff 视图 (留 v2)
