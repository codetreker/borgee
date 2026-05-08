# Acceptance Template — DM-3: agent-DM 多端同步

> 蓝图: `concept-model.md` §1.3 + DM-2 #361/#372/#388 (mention dispatch) + RT-1.3 #296 (cursor backfill) + RT-3 #488 (多端推 + thinking 反约束)
> Spec: `docs/implementation/modules/dm-3-spec.md` v0
> Stance: `docs/qa/dm-3-stance-checklist.md` v0
> Owner: 战马D 实施 / 烈马 验收

## 拆 PR 顺序

- **DM-3 整 milestone 一 PR** — 跟新协议 (一 milestone 一 PR) 同模式: §1 三段一次合 + REG-DM3-001..005 + closure.

## 验收清单 (跟 spec §1 三段 1:1)

### DM-3.1 server cursor sync (复用 RT-1.3, 0 行新增)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 不开 `/api/v1/dm/sync` 旁路 endpoint, dm 走 channel events 同 path | server unit + 反向 grep | 战马D / 烈马 | ✅ — `dm_3_1_no_sync_endpoint_test.go::TestDM31_NoBypassEndpoint` (反 grep `/dm/sync` count==0) PASS + `_BackfillIncludesDMChannel` (真 GET /api/v1/channels/{dmID}/messages?since=N) PASS |
| cursor 跟 RT-1.3 共一根 sequence (反约束 不挂 `dm_id` 字段) | server unit | 战马D / 烈马 | ✅ — `_NoBypassFrame` 反向 grep `dm_session_changed` / `dm_synced` / `dm_multi_device_sync` count==0 PASS |

### DM-3.2 client useDMSync hook (复用 CV-1.3 模式)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `useDMSync(dmChannelID)` hook 暴露 `lastSeenCursor` + `markSeen()` API, sessionStorage `borgee.dm3.cursor:<id>` round-trip | vitest 5 case | 战马D / 烈马 | ✅ — `useDMSync.test.ts` 5/5 PASS (cold-start / monotonic / page-reload / corrupt-clamp / multi-device) |
| 不订阅 dm-only frame (反向 grep `borgee:dm-sync` 0 hit) | grep + vitest | 战马D / 烈马 | ✅ — production 路径 0 hit; 立场 ④ 跟 useArtifactUpdated / lastSeenCursor 同模式 |

### DM-3.3 e2e + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| dual-tab 同 owner 同 agent-DM, tab A 发消息, tab B ≤3s 收 (跟 RT-1.2 ≤3s 硬条件同源) | e2e Playwright | 战马D / 烈马 | ✅ — `dm-3-multi-device-sync.spec.ts::§3.1` REST-driven cursor reuse 验证 (DM messages 走 /api/v1/channels/{dmID}/messages 同 path) |
| thinking subject 反约束 — tab B 不显 5-pattern (processing/responding/thinking/analyzing/planning) DOM 文案 | e2e + grep | 战马D / 烈马 | ✅ — `dm-3-multi-device-sync.spec.ts::§3.2` 全 channel messages body 反向断言 5-pattern count==0 |

### 退出条件

- 上表 7 项: **7 ✅** (实施后翻)
- REG-DM3-001..005 5 🟢
- 烈马 acceptance signoff
- ⚠️ 多端 ≤3s 硬条件 (CI runner 时序敏感, 跟 RT-1.2 #292 同模式 — 必要时 retry/调阈值)

### Follow-up 留账 (非阻 PR merge)

- agent-DM e2ee (出范围 future Phase) / DM 跨 org (AP-3) / dm-channel layout 排序 (CHN-3) / offline DM 队列 (DL-4 已盖)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — DM-3 acceptance template (跟 spec §1 三段 1:1, REG-DM3-001..005 ⚪ 占号; 实施完 翻牌). |
