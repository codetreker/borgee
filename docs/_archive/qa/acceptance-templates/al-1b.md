# Acceptance Template — AL-1b busy/idle (BPP 同期)

> 蓝图: `docs/blueprint/current/agent-lifecycle.md` §2.3 (5-state) · Implementation: `docs/implementation/modules/al-1b-spec.md` (战马C v0)
> 前置: AL-1a (#249) ✅ + AL-3 (#310/#317/#324) ✅ + AL-4 (#398) ✅ + BPP-2 frame schema 留账 · Owner: 战马C 三段全做 / 验收 烈马 / 文案 野马
> 拆 PR: **AL-1b.1** schema (本 PR v=21) + **AL-1b.2** server endpoint + state machine (待 PR) + **AL-1b.3** client SPA dot UI (待 PR)

## 验收清单

### 数据契约 (AL-1b.1 schema v=21 — 立场 ① 拆三路径)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 agent_status 表三轴 (agent_id PK / state NOT NULL / last_task_id + last_task_started_at + last_task_finished_at nullable / created_at + updated_at NOT NULL) | migration test | 战马C / 烈马 | `internal/migrations/al_1b_1_agent_status_test.go::TestAL1B1_CreatesAgentStatusTable` |
| 1.2 state CHECK ('busy','idle') 2 态 byte-identical + 反约束枚举外 reject (AL-1a 三态 online/offline/error + AL-4 4 态 + 同义词漂 active/working/idling) | migration test | 战马C / 烈马 | `TestAL1B1_AcceptsBusyIdleEnum` (2 态 PASS + 11 反约束 reject) |
| 1.3 反约束 NoDomainBleed 9 列反向 (is_online / presence / last_error_reason / endpoint_url / process_kind / source / set_by / cursor — 立场 ①② 拆 AL-3/AL-4/反人工伪造) | migration test | 战马C / 烈马 | `TestAL1B1_NoDomainBleed` |
| 1.4 v=21 sequencing + idempotent (CV-2.1 v=14 / DM-2.1 v=15 / AL-4.1 v=16 / CV-3.1 v=17 / CV-4.1 v=18 / CHN-3.1 v=19 / CHN-4.1 v=20 / **AL-1b.1 v=21**) + IF NOT EXISTS 守 | migration test | 战马C / 烈马 | `TestAL1B1_Idempotent` + `registry.go` 字面锁 |
| 1.5 INDEX idx_agent_status_state busy 列表 lookup 热路径 + NoCascadeDelete (蓝图 §2.3 保留状态历史 + 跟 al_3_1 / al_4_1 同逻辑 FK 模式) | migration test | 战马C / 烈马 | `TestAL1B1_HasStateIndex` + `TestAL1B1_NoCascadeDelete` |

### 状态机不变量 (AL-1b.2 server — 5-state 合并 + BPP frame state machine)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 GET /api/v1/agents/:id/status 5-state 合并优先级 (error > busy > idle > online > offline) | unit | 战马C / 烈马 | `internal/api/al_1b_2_status_test.go::TestAL1B2_GetStatus_NoRowFallsBackToOnlineOffline` (no-row → AL-1a 退化) + `TestAL1B2_GetStatus_BusyFromAgentStatusRow` (busy 优先 online) + `TestAL1B2_GetStatus_IdleFromAgentStatusRow` (idle 优先 online) |
| 2.2 BPP `task_started` frame → state=busy + last_task_id + last_task_started_at | unit | 战马C / 烈马 | `store/agent_status_queries.go::SetAgentTaskStarted` upsert ON CONFLICT(agent_id) + `TestAL1B2_GetStatus_BusyFromAgentStatusRow` (last_task_id="task-foo" + last_task_started_at byte-identical assert) |
| 2.3 BPP `task_finished` frame → state=idle + last_task_finished_at | unit | 战马C / 烈马 | `SetAgentTaskFinished` upsert + `TestAL1B2_GetStatus_IdleFromAgentStatusRow` (started → finished pair, last_task_finished_at=t0+30s) |
| 2.4 5min 无 frame → 自动 idle (单 const `IdleThreshold = 5*time.Minute`) | unit | 战马C / 烈马 | `ReapStaleBusyToIdle` + `TestAL1B2_ReapStaleBusyToIdle` (T+1min 0 行 + T+6min 1 行 reap, IdleThreshold const al_1b_2_status.go:line 单源) |
| 2.5 PATCH /api/v1/agents/:id/status admin god-mode 拒绝 (立场 ② BPP 单源, 反人工伪造) | unit | 战马C / 烈马 | `TestAL1B2_PatchStatusReturns405` (owner PATCH 405 + Allow: GET 头 + error 含 "BPP-driven" 关键词) + `TestAL1B2_PatchStatusAdminAlsoRejected` (admin PATCH 同 405) |

### 文案锁 (AL-1b.3 client — 野马 #190 §11 + 烈马判定)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 `describeAgentState(busy)` → "在工作" tone='ok'; 反 "活跃" / "running" 模糊 | vitest (agent-state.test.ts 扩 it) | 野马 / 烈马 | `packages/client/src/__tests__/agent-state.test.ts::busy → 在工作 tone=ok` (acceptance §3.1, byte-identical) + `PresenceDot.test.tsx::busy → data-task-state="busy" + 文本 "在工作"` |
| 3.2 `describeAgentState(idle)` → "空闲" tone='muted'; 反 "等待中" / "Standing by" | vitest | 野马 / 烈马 | `agent-state.test.ts::idle → 空闲 tone=muted` + `PresenceDot.test.tsx::idle → data-task-state="idle" + 文本 "空闲"` |
| 3.3 AL-1a 三态文案不变 ("在线" / "已离线" / "故障 (xxx)"): REG-AL1A-005 不破 | vitest 回归 | 烈马 | `agent-state.test.ts::AL-1a 三态文案不变 (REG-AL1A-005 回归不破)` + `PresenceDot.test.tsx::AL-1a 三态 data-task-state 为空 string (回归不破)` |
| 3.4 反约束 grep — `lib/agent-state.ts` 不准出现 "活跃" / "running" / "Standing by" / "等待中" 模糊词 | CI grep (vitest 跑) | 烈马 | `presence-reverse-grep.test.ts::§3.4 (AL-1b) agent-state.ts 不出现 "活跃"/"running"/"standing by"/"等待中" 模糊词` (跑过当前 source 0 hit) + `PresenceDot.test.tsx::反约束 §3.4: busy/idle 文案不准用模糊词` 双层闸 |

### e2e (Playwright)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 发任务 → "在工作" ≤ 1s; 结束 → "空闲" ≤ 1s + 5min → "在线" | E2E stopwatch + faked clock | 烈马 / 野马 | _(待 BPP-2 真 frame 落 — store helpers `SetAgentTaskStarted/Finished` + reaper 已就绪, 接 BPP frame dispatcher 即可切真路径; e2e 截屏路径 `g3.x-al-1b-{busy,idle}.png` 命名固化 待 follow-up)_ |

## 退出条件

- 数据契约 5 项 (AL-1b.1 本 PR ✅) + 行为不变量 5 项 (AL-1b.2 + BPP-2) + 文案 4 项 (AL-1b.3) + e2e 1 项**全绿** (一票否决)
- AL-1a REG-AL1A-001..005 回归不破 + AL-3 REG-AL3-001..010 回归不破 + AL-4 REG-AL4-001..005 回归不破 (立场 ① 拆三路径)
- 野马 G2.4 文案签字 + e2e 截屏入 `docs/qa/screenshots/g3.x-al-1b-*.png` + REG-AL1B-001..006 落 6 行 (本 PR 占号 ⚪..🟢 待 follow-up patch)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 — Phase 4 AL-1b 14 验收项 (rt-0.md 同模板) |
| 2026-04-29 | 战马C | flip §1.1-§1.5 schema 段 5 项 ⚪→✅ (AL-1b.1 v=21 落 — `internal/migrations/al_1b_1_agent_status.go` + 6 TestAL1B1_* 全 PASS); 锚 spec brief `docs/implementation/modules/al-1b-spec.md` (战马C v0 3 立场 + 3 拆段); §2 server / §3 文案 / §4 e2e 留 AL-1b.2 + AL-1b.3 + BPP-2 后填. |
| 2026-04-29 | 战马C | flip §2.1-§2.5 server 段 5 项 ⚪→✅ (AL-1b.2 落 — `internal/api/al_1b_2_status.go` + `internal/store/agent_status_queries.go` + 8 TestAL1B2_* 全 PASS): GET /api/v1/agents/:id/status 5-state 合并 + SetAgentTaskStarted/Finished BPP-2 stub + ReapStaleBusyToIdle 5min IdleThreshold const + PATCH 405 reject (owner+admin) + NoDomainBleed 反约束响应不泄漏 schema 内列. §3 文案 / §4 e2e 留 AL-1b.3 + BPP-2 真 frame 后填. |
| 2026-04-29 | 战马C | flip §3.1-§3.4 client 文案段 4 项 ⚪→✅ (AL-1b.3 client SPA dot UI 落): `lib/api.ts` AgentRuntimeState 扩 'busy'\|'idle' + Agent interface 加 last_task_* 三字段; `lib/agent-state.ts` describeAgentState() 加 busy → "在工作" tone='ok' + idle → "空闲" tone='muted' 两 case (AL-1a 三态不动 REG-AL1A-005 回归不破); `components/PresenceDot.tsx` DOM 加 data-task-state 槽位 (busy/idle 时填字面, 其他态填空 string) + presence-task-busy/idle CSS class + data-presence 仍为 'online' (busy/idle = 连着, 跟 AL-3 hub 同步 online 语义); 反约束 4 锚: agent-state.ts 不出现 "活跃"/"running"/"Standing by"/"等待中" 模糊词 (presence-reverse-grep.test.ts §3.4 强守 0 hit) + PresenceDot.test.tsx 反约束 §3.4 双层闸 + AL-1a 三态文案不变回归 + 立场 ① 拆三路径 (busy/idle 不带 reason — 不染 error case). 测试: `agent-state.test.ts` 扩 4 it (busy / idle / 三态回归 / busy-idle 不带 reason); `PresenceDot.test.tsx` 扩 5 it (busy DOM / idle DOM / 三态回归 data-task-state="" / 反约束模糊词 / busy compact); 全套 vitest 240/240 PASS. §4 e2e 1 项留 BPP-2 真 frame 后填 (store helpers + reaper 已就绪, 接 BPP frame dispatcher 即可切真路径). 14/14 验收项里 13 闭 (§4.1 ⏸️ pending BPP-2). |
