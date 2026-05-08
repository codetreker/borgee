# Phase 4 Acceptance Templates — Index

> 作者: 烈马 (QA) · 2026-04-28 · Phase 4 派活前置
> v0 索引 — 6 milestone 验收 item 列项 + owner; 详细 template 派活时单 doc 落 (跟 `adm-0.md` / `rt-0.md` 同模板, 4 列: 验收项 / 实施方式 / Owner / 实施证据).

## 1. AL-1b busy/idle (战马A, BPP 同期)

蓝图 `agent-lifecycle.md` §2.3 4-state. (1) BPP frame `task_started/finished` schema 锁; (2) Tracker 扩 busy/idle, 优先级 error > busy > idle > online > offline; (3) Sidebar 文案锁 "在忙/空闲/在线" 不准 "活跃" 模糊 (野马); (4) busy → idle 自动超时 (单一 const); (5) 单测 5 角覆盖; (6) e2e 任务发起 → busy ≤ 1s + 结束 → idle ≤ 1s.

## 2. AL-2a/2b 配置 SSOT + 热更新 (战马A)

蓝图 `agent-lifecycle.md` §3. (1) `agents.config_json` server + client TS 1:1; (2) PUT /agents/:id/config 原子 + 版本号 (no lost update); (3) config_changed 事件 ≤ 3s (无重启); (4) server-side schema 校验 (拒 unknown field); (5) 改 api_key → 清 error + 重连; (6) 单测 schema + 版本竞争 + hot-reload; (7) e2e owner 改 config → UI 刷新.

## 3. AL-3 presence 完整版 (战马A)

蓝图 `agent-lifecycle.md` §2 + `data-layer.md` agent_presence. (1) `agent_presence` 表 (PK agent_id, state, reason, last_heartbeat_at, updated_at); (2) Tracker → 表持久 (替 in-memory, restart 不丢); (3) 心跳过期 → 自动 offline; (4) GET /agents/:id/presence/history 审计; (5) 单测 persistence + 过期 + history 排序; (6) presence/contract.go (G2.5 留账闭).

## 4. AL-4 退役 = 禁用 (战马B)

蓝图 `agent-lifecycle.md` §4. (1) disabled + retired_at 时间戳, 不删行; (2) 退役保留 channel membership 只读; (3) 退役 owner 不能发任务 (UI + server 双拦); (4) 24h 内 owner Reset, 之后仅 admin; (5) 单测状态机 + Reset + 时间窗; (6) 反向断言: 退役 agent 不在 active 列表 / 不可邀请.

## 5. ADM-1 实施 (战马B, #228 spec)

蓝图 `docs/implementation/modules/adm-1.md`. (1) `AdminBanner` 组件 "管理员视图" 红横幅 (野马 §11 锁); (2) admin 进 user 路径 banner 必显 (反向: 无 banner = bug); (3) admin_actions audit 行 (who/when/what/target); (4) admin 改字段 → audit + "已审计" toast; (5) 单测 banner + audit + ADM-0 双轨 cookie 不破 (REG-ADM0-001/002); (6) e2e G2.4 demo #6 → 野马补 G2.4 5/6 → 6/6; (7) 反向断言 user-rail → admin endpoints 401.

## 6. ADM-2 分层透明 audit (战马B)

蓝图 `admin-model.md` §3. (1) `admin_actions` 表 (id, admin_id, action, target_type, target_id, payload, created_at); (2) 所有 admin 写路径自动落 audit 行; (3) GET /me/admin-actions owner 可见自己被操作历史; (4) admin 可见自己 + 全局历史; (5) append-only (PATCH/DELETE → 403); (6) 单测自动落 + owner/admin 可见性 + append-only.

---

## 7. Owner 分布

| Milestone | 主 owner | 协作 | 验收 |
|---|---|---|---|
| AL-1b | 战马A | 野马 文案 | 烈马 |
| AL-2a/2b | 战马A | — | 烈马 |
| AL-3 | 战马A | — | 烈马 (presence/contract.go G2.5 闭) |
| AL-4 | 战马B | 野马 文案 | 烈马 |
| ADM-1 | 战马B | 野马 demo (G2.4 #6) | 烈马 |
| ADM-2 | 战马B | — | 烈马 |

## 8. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 — 6 milestone 验收 item 索引 (列项 + owner) |
