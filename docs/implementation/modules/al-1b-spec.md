# AL-1b spec brief — busy/idle 状态扩展 (BPP 同期)

> 战马C · 2026-04-29 · ≤80 行 spec lock (跟 al-3-spec / al-4-spec 同模式)
> **蓝图锚**: [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.3 (5-state, 2026-04-28 4 人 review #5 决议: busy/idle 跟 BPP 同期 Phase 4)
> **关联**: AL-1a (#249) ✅ online/offline/error 三态 / AL-3 (#310/#317/#324) ✅ presence WS hub / AL-4 (#398) ✅ agent_runtimes process-level / BPP-2 留账 task_started/task_finished frame
> **acceptance**: `docs/qa/acceptance-templates/al-1b.md` (烈马 #193 v0)

## 0. 关键约束 (3 条立场)

- **立场 ① 拆三路径**: AL-1b busy/idle 跟 AL-1a online/offline/error + AL-3 presence_sessions + AL-4 agent_runtimes 拆死. busy/idle = task in-flight 真值, 不混 hub 心跳 (AL-3) 不混 process status (AL-4) 不混 runtime memory (AL-1a). 三表三路径互不污染, 改 = 改 4 个 spec.
- **立场 ② BPP 单源**: busy/idle source 必须是 plugin 上行 `task_started` / `task_finished` frame (BPP-2 待落), 没 BPP 不 stub (蓝图 §2.3 决议字面 "stub 一旦上 v1 要拆掉 = 白写"). schema 先落占号, server endpoint 仅暴露 GET 不暴露 PATCH 直接改 state (避免人工伪造).
- **立场 ③ 文案三态**: client UI 见 5-state 合并显示 (优先级 error > busy > idle > online > offline), 但 schema 仅 2 态 (busy/idle), AL-1a 三态独立. 客户端 `describeAgentState()` 合并逻辑跟 AL-1a state.go + AL-3 presence 字面对齐 (改 = 改 3 处单测锁).

## 1. 拆段实施 (AL-1b.1 / 1.2 / 1.3, ≤ 3 PR; v=21 schema)

| 拆段 | 范围 | 拆 PR | Owner |
|------|------|-------|-------|
| **AL-1b.1** schema (v=21 — zhanma-a 用 v=20 CHN-4.1 占位无 schema, 顺延 v=21) | `agent_status` 表 (agent_id PK / state CHECK busy/idle / last_task_id nullable / last_task_started_at + last_task_finished_at Unix ms / created_at + updated_at NOT NULL); INDEX idx_agent_status_state lookup busy 列表; 反约束 NoDomainBleed (无 cursor / is_online / last_error_reason / endpoint_url / process_kind — 跟 AL-3 / AL-4 / RT-1 拆死); 6 测试 (CreatesTable + NoDomainBleed + AcceptsBusyIdleEnum + Idempotent + HasIndex + NoCascadeDelete) | 本 PR | 战马C |
| **AL-1b.2** server | GET `/api/v1/agents/:id/status` (返 5-state 合并: AL-1a error > AL-1b busy/idle > AL-3 online/offline) + PATCH `/api/v1/agents/:id/status` admin-only god-mode reject (立场 ② 不允许人工改 busy/idle, 仅 BPP frame source); state machine: BPP `task_started` frame → busy + last_task_started_at; `task_finished` frame → idle + last_task_finished_at; 5min 无 frame → idle (单 const `IdleThreshold = 5*time.Minute`) | 待 PR | 战马C |
| **AL-1b.3** client | SPA 状态 dot UI 5-state 合并 (跟 AL-3 presence dot DOM `data-presence` byte-identical 同模式, AL-1b 加 `data-task-state="busy\|idle"`); `describeAgentState()` 函数加 busy/idle 两 case (文案锁 §3 字面 "在工作"/"空闲" 跟 #190 §11 + acceptance al-1b.md 字面对齐); REASON_LABELS 不动 (busy/idle 不带 reason) | 待 PR | 战马C |

## 2. v 号 sequencing (字面延续)

- CV-2.1 v=14 ✅ #359 / DM-2.1 v=15 ✅ #361 / AL-4.1 v=16 ✅ #398 / CV-3.1 v=17 ✅ #396 / CV-4.1 v=18 ✅ #405 / CHN-3.1 v=19 ✅ #410 / CHN-4.1 v=20 ✅ #411 (占位无 schema 改) / **AL-1b.1 v=21 待 (本 PR)** / AL-2a.1 v=22 占号 (zhanma-a Phase 4 平行)
- 反约束: 不抢 v=20 (CHN-4.1 占位) / 不复用 v=16 (AL-4.1 已落) / 不写 ON DELETE CASCADE (跟 al_3_1 / al_4_1 / cv_2_1 / dm_2_1 同模式逻辑 FK)

## 3. grep 反查 (CI 闭环锚)

```bash
git grep -nE 'agent_status'                    packages/server-go/internal/migrations/   # ≥ 1 hit (AL-1b.1)
git grep -nE 'idx_agent_status_state'          packages/server-go/internal/migrations/   # ≥ 1 hit (HasIndex)
git grep -nE "CHECK.*busy.*idle"               packages/server-go/internal/migrations/   # ≥ 1 hit (AcceptsBusyIdleEnum)
git grep -nE 'is_online|last_error_reason|cursor|endpoint_url'  packages/server-go/internal/migrations/al_1b*  # 0 hit (NoDomainBleed)
```

## 4. 反约束 (改 = 改 spec)

- 不动 AL-1a 三态字面 (REG-AL1A-005 反约束); 不动 AL-3 presence_sessions schema (REG-AL3-001 反约束); 不动 AL-4 agent_runtimes status CHECK (REG-AL4-005 反约束)
- 不挂 cursor 列 (跟 RT-1 envelope cursor 拆死, 同 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 模式)
- 不挂 ON DELETE CASCADE (蓝图 §2.3 字面 "保留状态历史"; SQLite FK 默认禁用 + 逻辑 FK 模式)
- 不开 PATCH /status admin-only god-mode 改 busy/idle (立场 ② BPP single source — admin 只能看, 不能改)

## 5. 退出条件 (本 PR AL-1b.1)

- 6 测试全 PASS (CreatesTable / NoDomainBleed / AcceptsBusyIdleEnum / Idempotent / HasIndex / NoCascadeDelete)
- registry.go 加 `al1b1AgentStatus` Migration v=21 + 注册 `All` 列表 (跟 al31 / al41 同模式)
- acceptance al-1b.md §1.* schema 段 5 项 ⚪→✅ 翻 (本 PR); 行为 §2.* / 文案 §3.* / e2e §4.* 留 AL-1b.2/1.3 + BPP-2 后填
- ≤300 行 (spec 80 + migration 90 + test 130 ≈ 300)
