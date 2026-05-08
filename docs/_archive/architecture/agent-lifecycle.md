# Implementation · Agent Lifecycle

> 蓝图: [`../../blueprint/current/agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md)
> 现状: agent 创建已存在 (admin API), 但状态只有 enabled/disabled 二态, 没有"故障态", 没有四态 UX, presence 只有 CM-4 引入的 minimal in-process map
> 阶段: ⚡ v0
> 所属 Phase: Phase 4

## 1. 现状 → 目标 概览

**现状**: agent 创建有, 状态二态, 无故障可解释; CM-4 已落 minimal presence (`IsOnline` 接口契约).
**目标**: blueprint 四条立场 — agent 创建完全用户决定 (无模板), 运行时默认 remote-agent 可选直连, 状态四态 + 故障可解释, 退役为禁用 (删除藏高级)。
**主要差距**:
1. 状态从二态扩到四态 (online / busy / error / offline) + 故障原因
2. agent 配置走平台下发 + 热更新 (blueprint §2.1 加条)
3. presence 完整版 (持久化 + 多端 + heartbeat 超时)

## 2. Milestones

### AL-1: 状态三态 (Phase 2 起步) + busy/idle 留 Phase 4 (R3 调整)

> **2026-04-28 4 人 review #5 决议** (蓝图 agent-lifecycle §2.3 已固化): busy/idle 在 Phase 2 不实现 (source 必须是 plugin 上行 `task_started/task_finished` frame, 没 BPP 就只能 stub, stub 一旦上 v1 要拆掉 = 白写)。**Phase 2 在线列表只承诺 online/offline + error 旁路**, busy/idle 跟 BPP 同期 (Phase 4) 落地。
> **野马硬条件 §11**: Phase 2 Sidebar 显示 online/offline 时, 不准用"灰点 + 不说原因"糊弄, 必须明确文案 ("已离线" 而不是模糊 idle 灰)。

- **目标**: blueprint §2.3 (R3 已固化分 Phase) — Phase 2 三态 (online/offline + error), Phase 4 补 busy/idle。
- **Owner (Phase 2 部分)**: 战马 / 飞马 / 烈马 / 野马 (§11 文案审过)
- **范围 (Phase 2 — AL-1a)**:
  - online/offline 由 hub.OnlineUsers + plugin connection map 推
  - error 旁路: agent runtime 故障时旁路态显示原因码
  - **Sidebar 文案**: "已离线" / "在线" / "故障 (api_key_invalid)" 等具体文案 (野马审过)
- **范围 (Phase 4 — AL-1b, BPP 同期)**:
  - busy: BPP frame `task_started` → state busy
  - idle: BPP frame `task_finished` → state idle (>5min 无活动)
- **依赖 (Phase 2)**: CM-4 (presence 接口)
- **依赖 (Phase 4)**: BPP-1 (协议骨架)
- **预估**: ⚡ v0 AL-1a 3-4 天 / AL-1b 4-5 天 (BPP 同期)
- **Acceptance** (Phase 2 部分):
  - 行为不变量 4.1: 状态机非法转移单测 (online↔offline / error 旁路)
  - 蓝图行为对照 §2.3 (Phase 2 行)
  - 用户感知签字 (野马, 走 G2.4 截屏的"左栏团队感知"那张): online/offline 文案明确, 不用糊弄灰

### AL-2: agent 配置 SSOT + 热更新 (拆 a/b)

- **目标**: blueprint §2.1 加条 + 核心 §9 — 配置 SSOT 在平台, 热更新立即生效。
- **Owner**: 飞马 / 战马 / 野马 (立场: 改 prompt 立刻生效, 不重启) / 烈马
- **范围 (拆 2 个 PR)**:
  - **AL-2a** `agent_configs(agent_id, schema_version, blob, updated_at)` 表 + update API; agent 端 reload 走轮询 (临时, 等 AL-2b)
  - **AL-2b** BPP `ConfigUpdated` frame; plugin 收到立即 reload — **必须跟 BPP-3 同 PR 合**, 防止 frame 字段改两次 (战马 D5 锁紧)
- **依赖**: AL-2a 无前置 (可并行 CM-*); AL-2b 依赖 BPP-1, 与 BPP-3 同合
- **预估**: ⚡ v0 AL-2a 4 天 + AL-2b 4 天 (与 BPP-3 联合)
- **Acceptance**:
  - AL-2a: 数据契约 (config 表 + update API) + 行为不变量 4.1 (并发 update idempotent)
  - AL-2b: E2E (改 config → 下条对话已生效, 不重启) + **用户感知截屏 4.2** (修改前 prompt / 修改后下条响应, 验证 §9 立场)

### AL-3: presence 完整版

- **目标**: 把 CM-4 的 minimal presence 扩到 multi-session + heartbeat 超时 + (可选) 跨进程 (留接口)。
- **Owner**: 飞马 (review 接口稳定) / 战马 / 野马 / 烈马
- **范围**: heartbeat (10s 间隔, 30s 超时); 同 agent 多 session (复用 CM-4 已锁定的 `Sessions(userID) []SessionID`); 超时自动清理
- **不在范围**: 跨进程 (Redis backed) ❌ — 留接口, 实现 v1+
- **依赖**: CM-4 (presence 接口契约已含 Sessions)
- **预估**: ⚡ v0 4-5 天
- **Acceptance**: 行为不变量 4.1 (heartbeat 超时 → IsOnline=false 单测; Sessions 多端去重单测) + **接口签名 snapshot test** (CM-4 锁定的 IsOnline + Sessions 签名不变, 烈马要求)

### AL-4: 退役 = 禁用 (删除藏高级)

- **目标**: blueprint §2.4 — 退役默认禁用, 删除藏在高级菜单。
- **Owner**: 飞马 / 战马 / 野马 (UX) / 烈马
- **范围**: `disabled_at` 字段; 禁用 agent 不会被 mention; 删除二次确认 + 高级菜单
- **预估**: ⚡ v0 3 天
- **Acceptance**: E2E + 蓝图行为对照 §2.4

## 3. 不在 agent-lifecycle 范围

- agent 运行时本身 → BPP / plugin 模块
- agent 权限 → auth-permissions

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| AL-1a | agent-lifecycle §2.3 (Phase 2 部分, R3 2026-04-28) | online/offline + error 三态 + 文案明确 (§11 不糊弄) |
| AL-1b | agent-lifecycle §2.3 (Phase 4 部分, BPP 同期) | busy/idle 加进来, 走 BPP `task_started`/`task_finished` |
| AL-2a | agent-lifecycle §2.1 加条 + 核心 §9 | 配置 SSOT 表 + update API |
| AL-2b | agent-lifecycle §2.1 加条 + 核心 §9 | 配置热更新 BPP frame, 改 prompt 立即生效不重启 |
| AL-3 | (实施加条) | presence 完整版, 复用 CM-4 接口契约 |
| AL-4 | agent-lifecycle §2.4 | 退役 = 禁用, 删除高级 |
