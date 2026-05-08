# Implementation · Auth & Permissions

> 蓝图: [`../../blueprint/current/auth-permissions.md`](../../blueprint/current/auth-permissions.md)
> 现状: `user_permissions(user_id, permission, resource)` 表已有, 但缺 ABAC scope 层级, 没有 UI bundle, 跨 org 限制不严格
> 阶段: ⚡ v0
> 所属 Phase: AP-0 在 Phase 1 (跟 CM-1 并行); AP-1~4 在 Phase 4

## 1. 现状 → 目标 概览

**现状**: 平面权限表, 全靠 `(user, perm, resource)` 三元组; 没分 scope 层级; 没 UI bundle; 跨 org 限制靠 owner_id 判断 (脆弱)。
**目标**: blueprint 四条立场 — C 混合 (ABAC 存储 + UI bundle), 三层 scope, B+A' 授权 UX (无角色名 bundle), 跨 org owner-only。
**主要差距**: scope 层级, UI bundle, 跨 org 严判, capability 清单。

## 2. Milestones

### AP-0: 默认权限注册回填 (从 CM-2 挪过来, 跟 CM-1 并行做)

- **目标**: blueprint §3 — 人类全权 (`*`), agent 默认最小 (`message.send`)。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**:
  - 注册新 human user 时, 写 `(user_id, '*', '*')` 到 `user_permissions`
  - 创建 agent 时, 写 `(agent_id, 'message.send', '*')`
  - 删除注册时不写权限的旧逻辑
- **依赖**: 无 (跟 CM-1 同 PR 或紧跟)
- **预估**: ⚡ v0 1-2 天
- **状态**: ✅ DONE (PR #177 merged)
- **Acceptance**: 数据契约 (新注册 human → 权限表 `*` 一行; 新 agent → `message.send` 一行)

### AP-0-bis: 默认权限补 message.read + backfill 迁移 (R3 新增, 2026-04-28)

> **2026-04-28 4 人 review #1 决议**: blueprint auth-permissions §3 加 `message.read` capability; agent 默认改成 `[message.send, message.read]` (owner 可在 agent 配置关掉)。AP-0 (#177) 已 merged 必须补回归 PR。

- **目标**: blueprint §3 (R3 已固化) — agent 默认 `[message.send, message.read]`, 现网旧 agent backfill。
- **Owner**: 战马 / 飞马 / 烈马
- **范围**:
  - `store.GrantDefaultPermissions(agent)` 改成 grant `[message.send, message.read]`
  - migration v=N: 现网所有 `role=agent` 的 user_permissions 加一行 `(agent_id, 'message.read', '*')` (idempotent)
  - **新增 `testutil.SeedLegacyAgent(t, db)` helper** (烈马 R3 要求, CM-3 也用): 插一个旧 schema 的 agent (无 message.read) 用于 backfill 测试
  - `GET /channels/:id/messages` 加 `RequirePermission("message.read")` middleware
- **不在范围**: agent 配置 UI 关闭 message.read (留给 AP-2 bundle UI)
- **依赖**: **ADM-0.2 已 merge** (飞马 R1 P0 ②: AP-0-bis 加 `RequirePermission("message.read")` 中间件必须在 ADM-0.2 砍掉 admin 直通短路之后, 否则 admin 既被砍直通又没 message.read 而 401 中间态); 不依赖 INFRA-2 / CM-onboarding (可与之并行)
- **预估**: ⚡ v0 1 天
- **Acceptance**:
  - 数据契约 4.3: 新注册 agent → user_permissions 多 2 行 (`message.send`, `message.read`)
  - 行为不变量 4.1: backfill migration up → 旧 agent 加 message.read; down → 回滚干净 (单测覆盖, 用 SeedLegacyAgent helper)
  - 行为不变量 4.1: 无 message.read 的 agent → GET messages 返 403 (单测)
  - 闸 5: 单测 ≥ 80% (含分支文件)

### AP-1: ABAC scope 层级 (org / channel / artifact)

- **目标**: blueprint §1.2 — v1 三层 scope。
- **Owner**: 飞马 (review 模型) / 战马 / 野马 / 烈马
- **范围**: 权限表加 `scope_type, scope_id` 字段; 检查函数按层级 fallback
- **依赖**: CM-1 (org_id), CHN-1 (workspace), CV-1 (artifact)
- **预估**: ⚡ v0 1 周
- **Acceptance**: 行为不变量 (org 级权限覆盖 channel 级, 单测)

### AP-2: UI bundle (无角色名)

- **目标**: blueprint §1.3 — A' 快速 bundle, 名字按 capability 不按角色 (Messaging / Workspace, 不是 PM / Dev)。
- **Owner**: 野马 (立场关键) / 战马 / 飞马 / 烈马
- **范围**: bundle 配置文件 (Messaging / Workspace / Channel / Org); UI 一键 grant; 底层依然落到 ABAC
- **依赖**: AP-1
- **预估**: ⚡ v0 1 周
- **Acceptance**: E2E + 用户感知签字 (野马: "我不会被引导成预设角色思维")

### AP-3: 跨 org owner-only 强制

- **目标**: blueprint §1.4 — 跨 org 协作时, agent 扩权必须 owner 同意。
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: 跨 org 权限请求走审批流, agent 不能自动扩权
- **依赖**: CM-4 (邀请审批已有)
- **预估**: ⚡ v0 4-5 天

### AP-4: capability 清单落地

- **目标**: blueprint §3 — v1 capability 清单 (Messaging / Workspace / Channel / Org), 全部 enum 化, 不允许自由字符串
- **Owner**: 飞马 / 战马 / 野马 / 烈马
- **范围**: capability enum 文件; 所有写入权限的代码用 enum, lint 强制
- **预估**: ⚡ v0 3 天

## 3. 不在 auth-permissions 范围

- 单点登录 (SSO) ❌
- 审计日志 → admin-model
- 跨 org "escape hatch" ❌ v1+

## 4. Blueprint 反查表

| Milestone | §X.Y | 立场一句话 |
|-----------|------|-----------|
| AP-0 | auth-permissions §3 (+ concept §3) | 人全权, agent 最小 |
| AP-0-bis | auth-permissions §3 (R3 2026-04-28 加 message.read) | agent 默认能读所在 channel, owner 可关; 现网 agent backfill |
| AP-1 | auth-permissions §1.2 | 三层 scope |
| AP-2 | auth-permissions §1.3 | bundle 按 capability 不按角色 |
| AP-3 | auth-permissions §1.4 | 跨 org owner-only 扩权 |
| AP-4 | auth-permissions §3 | capability 清单 enum 化 |
