# Implementation · Channel Model

> 蓝图: [`../../blueprint/current/channel-model.md`](../../blueprint/current/channel-model.md)
> 现状: [`../../current/server/`](../../current/server/) — channels / messages 表已存在, 但缺 workspace 概念, DM 与 channel 边界模糊, 分组只有作者侧
> 阶段: ⚡ v0
> 所属 Phase: Phase 3 — 见 [`execution-plan.md`](../00-foundation/execution-plan.md)

## 1. 现状 → 目标 概览

**现状**: channels / messages / DM 已具雏形, 但 (a) channel 没有"workspace"侧只剩聊天; (b) DM 共用 channel 表无独立语义; (c) 分组只有 server 端作者定义, 用户无法私下重排。

**目标**: blueprint 四条立场 — channel = 协作场 (chat + workspace 双支柱), DM 概念独立但底层可复用, workspace = artifact 集合, channel 分组分作者层 + 个人层。

**主要差距**:
1. workspace 表与 channel 关联不存在
2. DM 没有独立 entity, 与 channel 混用
3. channel 分组只有作者层, 缺个人 reorder / pin
4. 缺 "channel 自带 workspace" 的初始化逻辑

## 2. Milestones

### CHN-1: workspace 与 channel 关联

- **目标**: blueprint §1.1 + §1.3 — channel 自带 workspace, channel = 协作场。
- **Owner**: 飞马 (review) / 战马 (实现) / 野马 (立场) / 烈马 (acceptance)
- **范围**:
  - 新建 `workspaces(id, channel_id, org_id, created_at)` 表
  - 创建 channel 时自动建对应 workspace (1:1)
- **不在范围**: artifact 层细节 → canvas-vision
- **依赖**: CM-1, CM-3
- **预估**: ⚡ v0 3-4 天
- **PR 拆分**:
  - CHN-1.1 schema + 自动建 workspace (战马 / 飞马 review / 烈马 数据契约验证)
  - CHN-1.2 channel API 返回 workspace_id (战马 / 飞马 / 烈马 E2E)

### CHN-2: DM 概念独立

- **目标**: blueprint §1.2 — DM 与 channel 概念独立, 底层可复用 messages 但 entity 分离。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: `dm_threads` 表 (id, participant_a, participant_b, org_id_a, org_id_b); 跨 org DM first-class; 列表 API 区分 channels vs dms
- **不在范围**: 多人 DM ❌ (v1+)
- **依赖**: CM-1
- **预估**: ⚡ v0 3-4 天
- **Acceptance**: 数据契约 + E2E (跨 org DM 在双方 inbox 可见, 烈马跑)

### CHN-3: 个人分组 (reorder + pin)

- **目标**: blueprint §1.4 — 分组分作者层 + 个人层。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: `user_channel_prefs(user_id, channel_id, sort_index, pinned, hidden)` 表; 个人重排不影响他人; 作者新增 channel 默认插底
- **不在范围**: 智能分组 (AI 推荐) ❌
- **依赖**: CHN-1
- **预估**: ⚡ v0 3 天
- **Acceptance**: 行为不变量 4.1 (A 重排不影响 B 视图, 单测断言 — 烈马)

### CHN-4: channel 协作场骨架 demo ⭐

- **目标**: 把 CHN-1~3 串成可演示的"channel 协作场"形态, Phase 3 第一段产品 demo。
- **Owner**: 野马 (主, 跑 demo + 签字) / 战马 (准备 demo 环境) / 飞马 (review 整体) / 烈马 (跑 E2E)
- **范围**: 新建 channel → 默认带 workspace 入口 → 邀请 agent → 在 workspace 放第一个 artifact 占位
- **依赖**: CHN-1 ~ 3, CM-4, CV-1
- **预估**: ⚡ v0 1 周
- **Acceptance**: E2E + 用户感知签字 4.2 (野马跑 demo + 留关键截屏)

## 3. 不在 channel-model 范围

- artifact 内容渲染 → canvas-vision
- channel 内 realtime 推送 → realtime
- channel 权限细化 → auth-permissions

## 4. Blueprint 反查表 (闸 3)

| Milestone | Blueprint §X.Y | 立场一句话 |
|-----------|----------------|-----------|
| CHN-1 | channel-model §1.1 + §1.3 | channel 是协作场, 自带 workspace |
| CHN-2 | channel-model §1.2 | DM 概念独立, 底层可复用 messages |
| CHN-3 | channel-model §1.4 | 分组作者 + 个人双层, 个人不影响他人 |
| CHN-4 | channel-model §1.1 (整合) | "协作场" 形态首次可见 |
