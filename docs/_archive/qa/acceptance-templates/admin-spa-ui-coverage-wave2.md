# Acceptance Template — admin-spa-ui-coverage-wave2 第二波 (≤50 行)

> Spec: `admin-spa-ui-coverage-wave2-spec.md` (战马C v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收
>
> **范围**: client-only 4 page UI 接 4 个 existing server endpoint — runtimes / heartbeat-lag / archived channels / description-history. 0 server / 0 endpoint / 0 schema 改, ≤350 行 client.

## 验收清单

### §1 行为不变量 (shape SSOT + DOM 锚 + 路径独立)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 api.ts 加 4 helper (`fetchAdminRuntimes` / `fetchAdminHeartbeatLag` / `fetchAdminArchivedChannels` / `fetchAdminChannelDescriptionHistory`) + 4 interface byte-identical 跟 server endpoint shape | 数据契约 | `admin-spa-ui-coverage-wave2.test.tsx::REG-ASUC2-001` PASS (4 helper export + endpoint path) |
| 1.2 4 interface 字段 byte-identical 跟 server (LagSnapshot 9 / AdminRuntime 7 / archived 子集 / history entry 3) | 数据契约 | `_REG-ASUC2-002` PASS (字段 reverse-grep ≥15 hit) |
| 1.3 4 page DOM 锚 `data-asuc2-*` byte-identical (≥12 hit + 4 `data-page=admin-*`) | DOM grep | `_REG-ASUC2-003` PASS |
| 1.4 中文 UI 文案 byte-identical (运行时 / 心跳滞后 / 已归档频道 / 描述变更历史 + 按钮+空态字面) | content-lock | `_REG-ASUC2-004` PASS (文案 ≥10 字面 reverse-grep) |
| 1.5 admin god-mode 路径独立 — 4 page 仅 /admin-api/* 走 (反向 grep `/api/v1/` + `from '../../lib/api'` 0 hit) | grep | `_REG-ASUC2-005` PASS |

### §2 数据契约 + 反向 grep 锚 (server endpoint 不动)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 `git diff origin/main -- packages/server-go/` 0 行 (本 milestone client-only) | git diff | 0 行 ✅ |
| 2.2 4 nav 入口加 sidebar (Runtimes / Heartbeat Lag / Archived Channels / 描述历史 from ChannelsPage row) + AdminApp.tsx 4 Route 挂 | inspect | `_REG-ASUC2-006` PASS (4 Route + 3+ nav 入口) |

### §3 closure (REG + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有 client vitest 全绿不破 + 1 新 file 7 case PASS | full vitest | vitest run PASS |
| 3.2 立场承袭 admin-spa-ui-coverage 第一波 #639 模式 + ADM-0 §1.3 红线 + AL-4.2 + HB-5 + CHN-5 + CHN-14 server SSOT byte-identical | inspect | `_REG-ASUC2-007` PASS (spec §4 立场承袭锁链 byte-identical) |

## REG-ASUC2-* (initial ⚪ → 🟢 post-impl)

- REG-ASUC2-001 🟢 api.ts 4 helper export + endpoint path byte-identical (4 endpoint)
- REG-ASUC2-002 🟢 4 interface 字段 byte-identical 跟 server SSOT
- REG-ASUC2-003 🟢 4 page DOM 锚 data-asuc2-* SSOT (≥12 实测)
- REG-ASUC2-004 🟢 中文 UI 文案 byte-identical (content-lock §1.4)
- REG-ASUC2-005 🟢 admin god-mode 路径独立 (ADM-0 §1.3 红线 反 user-rail leak 0 hit)
- REG-ASUC2-006 🟢 4 Route + nav 入口挂 sidebar
- REG-ASUC2-007 🟢 立场承袭锁链 byte-identical (#639 + ADM-0 §1.3 + AL-4.2 + HB-5 + CHN-5 + CHN-14)

## 退出条件

- §1 (5) + §2 (2) + §3 (2) 全绿 — 一票否决
- vitest 7 case PASS + 既有 108 file 711 case 全绿不破
- 0 server / 0 endpoint / 0 schema 改
- 登记 REG-ASUC2-001..007

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-02 | 战马C | v0 实施 — admin-spa-ui-coverage-wave2 第二波 4 件套 byte-identical + 4 page UI ~270 行 client + api.ts 80 行 helper + 7 vitest PASS. REG-ASUC2-001..007 ⚪→🟢 全翻. 立场承袭 #639 + ADM-0 §1.3 + AL-4.2 + HB-5 + CHN-5 + CHN-14. |
