I've read both design docs and explored the codebase. Here's the full task breakdown:

---

## P1 Task List — Agent 归属与权限系统

> **Review Fix Log**
>
> - **CC-C1**：角色命名统一为 `member`（与 DB `role` 列一致），全文不再使用 `user` 指代普通人类用户角色
> - **CC-H1**：T9 新增 `channel_deleted` WS 事件前端处理（从 store 移除、切到 #general、Toast 提示）
> - **CC-H2**：T2 验证项显式列出每个端点的权限挂载
> - **CC-H3**：T8 新增 owner 删除/禁用时 agent 级联软删逻辑
> - **CC-H4**：公开频道预览（24h 消息）显式标注为 P2 defer
> - **Codex-H1**：T1 新增现有 agent 回填 `owner_id`（归属首个 admin）
> - **Codex-H2**：T8 新增 `owner_id`/`role` 不变式校验（role 变更时）
> - **Codex-H3**：T2 新增 `message.send` 权限挂载到 `POST /channels/:id/messages`
> - **Codex-H4**：新增 T2.5 权限读取 API（`GET /api/v1/me/permissions` + `GET /api/v1/admin/users/:id/permissions`），解除 T9/T10 的后端依赖
> - **CC-C2**：T1 新增 `users.deleted_at` (INTEGER, nullable) 和 `users.disabled` (INTEGER DEFAULT 0) 字段，供 T8 owner 删除/禁用级联使用；T2 认证中间件过滤 disabled/deleted 用户
> - **CC-C3**：T8 `DELETE /admin/users/:id` 改为软删除（设 `deleted_at` + `disabled=1`），不再硬删 `users` 行。原因：`owner_id` 是 FK REFERENCES `users(id)`，硬删 owner 会导致 FK 冲突（agent 行的 `owner_id` 指向已删除的 `users` 行）。软删后 T2 认证层已拒绝 `deleted_at IS NOT NULL` 的用户，等效于不可用；同时保留 `users` 行使 agent 的 `owner_id` FK 仍然有效

---

### P2 Deferred Items

- **公开频道预览（24h 消息）**：`product-direction.md` 要求"公开频道未加入时可预览最近 24 小时消息，不能发言，需申请加入"。此功能不在 P1 范围内，显式 defer 到 P2。（CC-H4）

---

### T1 — 数据库迁移：P1 schema 扩展

- **文件**：`packages/server/src/db.ts`
- **改动**：
  - `users` 表加 `owner_id` 列 + 索引
  - `users` 表加 `deleted_at` 列（INTEGER, nullable）— 软删除时间戳，供 T8 owner 删除级联使用（CC-C2）
  - `users` 表加 `disabled` 列（INTEGER DEFAULT 0）— 禁用标记，0=正常 1=禁用，供 T8 owner 禁用级联 + T2 认证过滤使用（CC-C2）
  - 新建 `user_permissions` 表 + 索引
  - 新建 `invite_codes` 表 + 索引
  - 回填现有 member 默认权限（`INSERT OR IGNORE`）
  - 回填现有频道 Creator 权限
  - **回填现有 agent 的 `owner_id`**：将所有 `role='agent'` 且 `owner_id IS NULL` 的行归属到首个 `role='admin'` 用户（Codex-H1）
- **预估行数**：~160 行（SQL + 幂等检查逻辑 + agent 回填 + 新字段迁移）
- **验证**：启动 server，`PRAGMA table_info(users)` 确认 `owner_id`、`deleted_at`、`disabled`；查 `user_permissions` 表确认回填数据；确认所有 `role='agent'` 行均有非 NULL `owner_id`；确认 `disabled` 默认值为 0；重复启动不报错（幂等）
- **依赖**：无

---

### T2 — 权限中间件 `requirePermission` + 认证层 disabled/deleted 过滤

- **文件**：
  - 新建 `packages/server/src/middleware/permissions.ts`（或直接放 `auth.ts`）
  - `packages/server/src/auth.ts`（认证中间件增加 disabled/deleted 检查）
  - `packages/server/src/routes/channels.ts`（挂载中间件）
- **改动**：
  - 实现 `requirePermission(permission, scopeResolver?)` Fastify preHandler
  - admin 短路通过；其余查 `user_permissions` 表
  - **认证中间件增加 disabled/deleted 用户过滤**（CC-C2）：在现有 API key / session 认证逻辑中，验证通过后额外检查 `users.deleted_at IS NOT NULL` 或 `users.disabled = 1`，若命中则返回 `401 Unauthorized`（body: `{ error: "account_disabled" }` 或 `{ error: "account_deleted" }`）。此检查确保被禁用/删除的用户（包括其 API key）无法访问任何 API 端点
  - 挂载到以下路由：
    - `POST /channels` — `channel.create`
    - `DELETE /channels/:id` — `channel.delete`（scope: channelId）
    - `PATCH /channels/:id` (visibility) — `channel.manage_visibility`（scope: channelId）
    - `POST /channels/:id/members` — `channel.manage_members`（scope: channelId）
    - `DELETE /channels/:id/members/:uid` — `channel.manage_members`（scope: channelId）
    - `POST /channels/:id/messages` — `message.send`（scope: channelId）（Codex-H3）
- **预估行数**：~80 行（中间件 + disabled/deleted 检查）+ ~40 行（路由改造）
- **验证**：
  - 单测覆盖 admin 短路 / member 有权 / member 无权 / scope 通配 vs 精确匹配；无权限用户请求返回 403
  - **disabled/deleted 用户认证测试**（CC-C2）：
    - `disabled=1` 的用户使用有效 API key 调任意端点 → 401 `account_disabled`
    - `deleted_at IS NOT NULL` 的用户使用有效 API key 调任意端点 → 401 `account_deleted`
    - `disabled=1` 的 agent 使用有效 API key 发消息 → 401
    - 正常用户不受影响
  - **逐端点验证**（CC-H2）：
    - `POST /channels`：无 `channel.create` 权限 → 403
    - `DELETE /channels/:id`：无 `channel.delete` 权限 → 403
    - `PATCH /channels/:id`：无 `channel.manage_visibility` 权限 → 403
    - `POST /channels/:id/members`：无 `channel.manage_members` 权限 → 403
    - `DELETE /channels/:id/members/:uid`：无 `channel.manage_members` 权限 → 403
    - `POST /channels/:id/messages`：无 `message.send` 权限 → 403；撤销 agent 的 `message.send` 后该 agent 无法发消息
- **依赖**：T1

---

### T2.5 — 权限读取 API（Codex-H4）

- **文件**：
  - `packages/server/src/routes/channels.ts` 或新建 `packages/server/src/routes/permissions.ts`
  - `packages/server/src/routes/admin.ts`
  - `packages/server/src/queries.ts`
- **改动**：
  - `GET /api/v1/me/permissions` — 返回当前登录用户的完整权限列表（T9 前端 `useCan` hook 依赖此接口）
  - `GET /api/v1/admin/users/:id/permissions` — admin 查看指定用户的权限列表（T10 权限管理页依赖此接口）
- **预估行数**：~50 行
- **验证**：member 调 `/me/permissions` 返回自身权限列表；非 admin 调 `/admin/users/:id/permissions` 返回 403；admin 可查任意用户权限
- **依赖**：T1

---

### T3 — 默认权限授予 + Creator 权限 helper

- **文件**：
  - `packages/server/src/queries.ts`（新增 helper 函数）
  - `packages/server/src/routes/channels.ts`（创建频道后调用 Creator 权限授予）
- **改动**：
  - `grantDefaultPermissions(userId, role)` — 注册/Agent 创建时插入默认权限
  - `grantCreatorPermissions(creatorId, creatorRole, channelId, ownerIdIfAgent?)` — 频道创建后授 `channel.delete/manage_members/manage_visibility`
  - member 创建 → 权限归 member；agent 创建 → 权限归 owner
- **预估行数**：~80 行
- **验证**：单测：member 创建频道得 3 项 scoped 权限；agent 创建频道 owner 得权限、agent 不得
- **依赖**：T1

---

### T4 — 邀请码 CRUD + 注册端点

- **文件**：
  - `packages/server/src/auth.ts`（新增 `POST /api/v1/auth/register`）
  - `packages/server/src/routes/admin.ts`（邀请码 CRUD）
  - `packages/server/src/queries.ts`（邀请码查询）
- **改动**：
  - `POST /api/v1/admin/invites` — 生成 16 字符随机码
  - `GET /api/v1/admin/invites` — 列表
  - `DELETE /api/v1/admin/invites/:code` — 作废
  - `POST /api/v1/auth/register` — 校验邀请码+邮箱唯一，事务内注册+授权+消费码+加入 #general
  - 注册端点速率限制（IP 10/min）
- **预估行数**：~200 行
- **验证**：集成测试：生成码→注册成功→码作废→重复注册失败；过期码拒绝；并发同码只成功一个
- **依赖**：T1, T3

---

### T5 — Agent CRUD API

- **文件**：
  - 新建 `packages/server/src/routes/agents.ts`
  - `packages/server/src/index.ts`（注册路由）
  - `packages/server/src/queries.ts`（Agent 查询）
- **改动**：
  - `POST /api/v1/agents` — 创建 agent（设 `owner_id`、`role=agent`、生成 api_key、默认权限）
  - `GET /api/v1/agents` — 列出 owner 的 agents（admin 可查全部）
  - `DELETE /api/v1/agents/:id` — owner 或 admin 删除
  - `POST /api/v1/agents/:id/rotate-api-key` — 轮换密钥
  - `GET /api/v1/agents/:id/permissions` — 查看 agent 权限
  - `PUT /api/v1/agents/:id/permissions` — 全量覆盖 agent 权限
- **预估行数**：~250 行
- **验证**：单测：member 创建 agent 成功、agent 有 owner_id、api_key 只返回一次；非 owner 删除返回 403；rotate 后旧 key 失效
- **依赖**：T1, T2, T3

---

### T6 — Agent 入频道校验

- **文件**：
  - `packages/server/src/routes/channels.ts`（修改 create + add member）
  - `packages/server/src/queries.ts`（`validateAgentMembership` helper）
- **改动**：
  - 抽取 `validateAgentMembership(caller, agentUser, channelId, memberIds)`
  - `POST /channels` 创建时：遍历 `member_ids`，对 `role=agent` 的 member 校验
  - `POST /channels/:id/members` 添加时：同样校验
  - 校验规则：caller 必须是 agent 的 owner（或 admin）；owner 必须已在频道内或同批加入
- **预估行数**：~60 行
- **验证**：单测：非 owner 拉 agent → 403；owner 不在频道 → 409；owner 同批加入 → 成功
- **依赖**：T5

---

### T7 — 频道删除端点权限化 + 读取端 `deleted_at` 过滤核查

- **文件**：
  - `packages/server/src/routes/channels.ts`
  - `packages/server/src/queries.ts`
- **改动**：
  - DELETE 端点改用 `requirePermission('channel.delete', scopeResolver)` 替代现有 creator/admin 硬编码
  - 核查所有 channel list/get/message 查询确保 `WHERE deleted_at IS NULL`
  - 删除事务内清理该频道的 `user_permissions`（避免孤儿权限）
- **预估行数**：~40 行
- **验证**：删频道后 GET 返回 404；list 不含已删频道；#general/DM 删除返回 409；幂等（重复删返回 204）
- **依赖**：T2（可与 T5 并行）

---

### T8 — Admin API 扩展

- **文件**：
  - `packages/server/src/routes/admin.ts`
- **改动**：
  - `GET /api/v1/admin/channels` — 全部频道含 deleted_at
  - `DELETE /api/v1/admin/channels/:id/force` — 强制删除（#general 除外）
  - `POST /api/v1/admin/permissions` — 授予权限
  - `DELETE /api/v1/admin/permissions/:id` — 撤销权限
  - `PATCH /api/v1/admin/users/:id` — 改 role（现有 PUT 改为支持 role 变更）
  - **`owner_id`/`role` 不变式校验**（Codex-H2）：role 变更时检查——将 member 改为 agent 必须提供 `owner_id`；将 agent 改为 member/admin 必须清除 `owner_id`；admin/member 的 `owner_id` 必须为 NULL
  - **`DELETE /api/v1/admin/users/:id` — 软删除用户**（CC-C3）：不再执行 `DELETE FROM users`，改为设 `users.deleted_at = unixepoch()` + `users.disabled = 1`。原因：`owner_id` 是 FK REFERENCES `users(id)`，硬删 owner 行会导致其名下 agent 的 `owner_id` FK 约束失败。软删后 T2 认证层已拒绝 `deleted_at IS NOT NULL` 的用户，等效于账号不可用。同时级联软删其名下所有 `role='agent'` 的用户（设 `deleted_at = unixepoch()` + `disabled = 1`），并清理被删用户及其 agent 的 `user_permissions` 记录、从 `channel_members` 中移除（但不删 `users` 行）。对于 `mentions` 表中引用被删用户的记录保持不动（软删不影响历史数据完整性）
  - **禁用用户**：`PATCH /api/v1/admin/users/:id` 设 `disabled=1` 时，级联禁用其名下 agent（设 `users.disabled = 1`），但不设 `deleted_at`，以便解禁时可恢复
- **预估行数**：~200 行
- **验证**：
  - admin 可列全部频道（含已删）；非 admin 403；权限增删立即生效
  - 将 member 改 agent 不提供 owner_id → 400
  - **软删除用户验证**（CC-C3）：
    - 删除 member → `users` 行仍存在，`deleted_at` 非 NULL + `disabled=1`
    - 被删 member 名下 agent 全部 `deleted_at` 非 NULL + `disabled=1`
    - 被删 member 及其 agent 的 `user_permissions` 已清理
    - 被删 member 及其 agent 已从 `channel_members` 移除
    - 被删用户使用 API key 调接口 → 401（由 T2 认证层保证）
    - 被级联软删的 agent 使用 API key 调接口 → 401
    - `users` 表中被删行的 `id` 仍可被其他 agent 的 `owner_id` 引用（FK 不冲突）
  - 禁用 member → 其 agent 全部 `disabled=1`（`deleted_at` 仍为 NULL）
  - 被级联禁用的 agent 使用 API key 调接口 → 401（由 T2 认证层保证）
- **依赖**：T2（可与 T5/T7 并行）

---

### T9 — 前端权限 hook `useCan` + WS 事件处理

- **文件**：
  - 新建 `packages/client/src/hooks/usePermissions.ts`
  - `packages/client/src/lib/api.ts`（调用 `GET /api/v1/me/permissions`）
  - `packages/client/src/context/AppContext.tsx`（state 加 `permissions`）
  - `packages/client/src/components/ChannelMembersModal.tsx`（按钮条件渲染）
  - `packages/client/src/components/Sidebar.tsx`（按钮条件渲染）
  - `packages/client/src/hooks/useWebSocket.ts` 或相关 WS handler
- **改动**：
  - 登录后调用 `GET /api/v1/me/permissions` 拉取当前用户权限列表缓存到 context
  - `useCan(permission, scope?)` hook：admin 恒 true，其余查本地权限
  - 删除按钮、管理成员按钮等根据 `useCan` 显示/隐藏
  - **`channel_deleted` WS 事件处理**（CC-H1）：收到 `channel_deleted` 事件后，从 channel store 中移除该频道；若当前选中频道被删，自动切换到 #general；Toast 提示"频道已被删除"
- **预估行数**：~130 行
- **验证**：member 无 `channel.delete` 权限时看不到删除按钮；creator 能看到；admin 恒看到；点击后端 403 时优雅提示；另一用户删除当前频道 → 自动跳转 #general + Toast
- **依赖**：T2.5, T7（WS 事件依赖后端频道删除广播）

---

### T10 — 前端 `/admin` 路由 + 4 个管理页面

- **文件**：
  - `packages/client/src/components/AdminPage.tsx`（重构/扩展）
  - 新建 `packages/client/src/components/admin/AdminLayout.tsx`
  - 新建 `packages/client/src/components/admin/UsersPage.tsx`
  - 新建 `packages/client/src/components/admin/InvitesPage.tsx`
  - 新建 `packages/client/src/components/admin/ChannelsPage.tsx`
  - 新建 `packages/client/src/components/admin/PermissionsPage.tsx`
  - `packages/client/src/App.tsx`（路由切换逻辑）
  - `packages/client/src/lib/api.ts`（admin API 调用，含 `GET /api/v1/admin/users/:id/permissions`）
- **改动**：
  - AdminLayout 带侧边 tab 导航（Users / Invites / Channels / Permissions）
  - 非 admin 访问重定向回主页
  - 每个页面：表格 + 操作按钮（CRUD）
  - 用户页：列表、改 role、禁用、软删除
  - 邀请码页：生成、列表、作废
  - 频道页：全部频道（含已删）、强制删除
  - 权限页：按用户查看/授予/撤销（调用 `GET /api/v1/admin/users/:id/permissions` 获取权限列表）
- **预估行数**：~500 行
- **验证**：浏览器中 admin 登录 → 进入 /admin → 四个 tab 均可操作；非 admin 看不到入口
- **依赖**：T8, T2.5（可与 T9 并行）

---

### T11 — 前端邀请码注册页

- **文件**：
  - 新建 `packages/client/src/components/RegisterPage.tsx`
  - `packages/client/src/components/LoginPage.tsx`（加"注册"链接）
  - `packages/client/src/App.tsx`（路由切换）
  - `packages/client/src/lib/api.ts`（register API）
- **改动**：
  - 注册表单：invite_code + email + password + display_name
  - 成功后自动登录跳转主页
  - Login 页面加 "有邀请码？去注册" 入口
- **预估行数**：~120 行
- **验证**：用有效邀请码注册成功并自动登录；无效码报错；已用码报错
- **依赖**：T4（可与 T10 并行）

---

### T12 — 前端 Agent 管理页面

- **文件**：
  - 新建 `packages/client/src/components/AgentManager.tsx`
  - `packages/client/src/components/Sidebar.tsx`（入口）
  - `packages/client/src/lib/api.ts`（agent API 调用）
- **改动**：
  - Agent 列表（当前 member 拥有的）
  - 创建 Agent（display_name, avatar_url）→ 显示一次性 api_key
  - 删除 Agent
  - 轮换 API key
  - 查看/编辑 Agent 权限
- **预估行数**：~200 行
- **验证**：member 创建 agent → 列表显示 → api_key 只展示一次 → 删除后消失；拉 agent 入频道后可发消息
- **依赖**：T5（可与 T10/T11 并行）

---

### T13 — E2E 冒烟测试 + 集成测试补全

- **文件**：
  - `packages/server/src/__tests__/` 下新建/扩展测试文件
  - 可选：Playwright e2e 脚本
- **改动**：
  - 集成测试：完整注册流、Agent 完整流、频道删除流、迁移幂等性
  - 手动冒烟：Agent API key 流、#general 删除 409、DM 删除 409
  - **新增**：`owner_id`/`role` 不变式测试——admin role 变更不产生非法 `owner_id` 组合；owner 删除后其 agent 均已软删
  - **新增**：`message.send` 权限测试——撤销后 agent 发消息 → 403
  - **新增**：disabled/deleted 用户认证拒绝测试——被禁用/删除的用户和 agent 使用有效 API key 访问任意端点 → 401；owner 删除/禁用级联后 agent 立即被认证层拦截
  - **新增**：owner 软删除 FK 完整性测试（CC-C3）——删除 owner 后 `users` 行仍存在，agent 的 `owner_id` FK 约束不报错；查询 agent 时可 JOIN users 获取 owner 信息（含 `deleted_at` 标记）
- **预估行数**：~400 行
- **验证**：`npm test` 全部通过
- **依赖**：T1-T12 全部

---

## 依赖关系图

```
T1 (DB migration + agent owner backfill + deleted_at/disabled 字段)
├── T2 (permission middleware + message.send + disabled/deleted 认证过滤)
│   ├── T5 (Agent CRUD) ←── T3
│   │   └── T6 (Agent channel validation)
│   ├── T7 (channel delete + filter) ── 可与 T5 并行
│   ├── T8 (Admin API + soft-delete + cascade + invariants) ── 可与 T5/T7 并行
│   └── T9 (frontend useCan + channel_deleted WS) ←── T2.5, T7
├── T2.5 (permission read APIs) ←── T1
├── T3 (default + creator permissions)
│   └── T4 (invite codes + register) ── T1+T3
└──────────────────────────────────────────
    T10 (admin UI) ← T8, T2.5  ┐
    T11 (register UI) ← T4     ├─ 可并行
    T12 (agent mgmt UI) ← T5   ┘
                                  └── T13 (E2E tests) ← all
```

**关键路径**：T1 → T2 → T5 → T6 → T13（~12h）。T2.5 可与 T2 并行开发。双人协作可将 T7/T8/T9 和 T10/T11/T12 分两条线并行推进，总工期压到 2-3 天。
