# admin-spa-ui-coverage-wave2 stance checklist (≤80 行)

> 战马C · 2026-05-02 · 跟 spec brief 1:1 byte-identical, post-#639 第一波模式承袭

## 1. 立场单源 (5 立场)

- **立场 ①**: 0 server / 0 endpoint / 0 schema / 0 routes 改 — 4 endpoint server 已挂 (runtimes.go:538 / host_lag.go:52 / channel_archived.go:44 / channel_history.go:48), 仅 client 接 UI
- **立场 ②**: shape byte-identical 跟 server SSOT — TS interface 字段名+类型跟 server JSON struct tag byte-identical (LagSnapshot 9 字段 / runtimeRow 7 字段 / ChannelWithCounts archived 子集 / description history entry `{old_content, ts, reason}` 3 字段)
- **立场 ③**: admin god-mode 路径独立 (ADM-0 §1.3 红线) — 4 page 仅访问 `/admin-api/*`, 不串 user-rail (`/api/v1/`) + 不 import user-rail `lib/api`
- **立场 ④**: data-asuc2-* DOM 锚 byte-identical (跟 #639 `data-asuc-*` + ADMIN-SPA-SHAPE-FIX `data-asf-*` 模式承袭) + 中文 UI 文案 byte-identical
- **立场 ⑤**: readonly admin god-mode 视图 — 4 page 仅 GET, 0 mutation (server 都 readonly, ADM-0 §1.3 admin 看 audit 不直接改)

## 2. 反约束 (4 项)

- ❌ server endpoint 加 / shape 改 (本 milestone 仅 client UI)
- ❌ POST/PATCH/DELETE on 4 endpoint (server readonly, 反 admin god-mode 直接改红线)
- ❌ admin SPA 串 user-rail api (反 ADM-0 §1.3 红线)
- ❌ heartbeat-lag chart / timeline 高级 viz (反 scope 漂, 留 v2)

## 3. 跨 milestone 锁链 (5 处)

- admin-spa-ui-coverage 第一波 #639 — `data-asuc-*` + 中文文案 模式承袭
- AL-4.2 #398 agent_runtimes — 表 SSOT (id/agent_id/endpoint_url/process_kind/status)
- HB-5 #408 host_lag — LagSnapshot 9 字段 SSOT (p50/p95/p99/threshold/at_risk)
- CHN-5 #189 — archived channels admin god-mode readonly
- CHN-14 #429 — description_edit_history JSON 数组 schema (`{old_content, ts, reason}`)
- ADM-0 §1.3 admin/user 路径分叉红线

## 4. PM 拆死决策 (3 段)

- **runtimes scope 全 vs 按 agent 拆死** — server `/runtimes` 是全 agent_runtimes (admin god-mode 全视图), 不按 :id; client UI 也走全列表 (反向: 不发明 `/agents/:id/runtimes` 路径, 跟 server SSOT byte-identical)
- **archived channels admin vs user 拆死** — 走 `/admin-api/v1/channels/archived` (全 org), 不走 `/api/v1/me/archived-channels` (user-rail owner-only); admin god-mode 全 org readonly
- **description-history 入口拆死** — 从 ChannelsPage 行 click 跳页 (`/admin/channels/:id/description-history`); 不在 ChannelsPage 内嵌 modal (反 scope 漂; 跳页 readonly)

## 5. 用户主权红线 (4 项)

- ✅ admin god-mode 不串 user session 数据
- ✅ admin SPA UI 改 0 server 行为
- ✅ runtimes UI 不暴露 last_error_reason (server 已 omit, client 也不展示, ADM-0 §1.3 隐私守门)
- ✅ admin god-mode 永不挂 user-rail (ADM-0 §1.3 红线)

## 6. PR 出来 5 核对疑点

1. 0 server diff (`git diff origin/main -- packages/server-go/` 0 行)
2. shape SSOT byte-identical (LagSnapshot 9 字段 + runtimeRow 7 字段 + history entry 3 字段, 反向 grep ≥5 hit)
3. admin god-mode 路径独立 (`fetch.*'/api/v1/'` + `from '../../lib/api'` 在 4 page 0 hit)
4. 12+ DOM data-asuc2-* 锚 ≥12 hit (vitest reverse-grep 守)
5. 既有 client vitest 全绿 + 新 file 7 case PASS

| 2026-05-02 | 战马C | v0 stance checklist — 5 立场 byte-identical 跟 spec brief, 4 反约束 + 5 跨链 + 3 拆死决策 + 4 红线 + 5 PR 核对疑点. 立场承袭 admin-spa-ui-coverage 第一波 #639 + ADM-0 §1.3. |
