# COL-B27 Admin 控制面板分离 — 技术设计

日期：2026-04-26 | 状态：Draft

> **历史标记** (COOKIE-NAME-CLEANUP 2026-05-01): 本设计文 v0.1 写期 cookie 名为 `borgee_admin_token`, ADM-0.1 实施 (#479) 时改 SSOT 为 `borgee_admin_session` byte-identical. 本文 §3+§5+§9+§13 内 `borgee_admin_token` 字面是 v0.1 历史草稿残留, 真值 SSOT 见 `internal/admin/auth.go::CookieName="borgee_admin_session"`. 改 cookie 名 = 全 admin session 失效, **本 milestone 0 cookie 字面值改**, 仅设计文加历史标记.

## 1. 概述

Admin 与 User 身份体系彻底分离：Admin 不再是 `users` 表中 `role=admin` 的一行记录，而是由环境变量定义的独立身份。Admin 拥有独立的登录接口、JWT 签发、API 路径前缀（`/admin-api/v1/*`）和前端 SPA 入口（`/admin/*`）。普通用户侧完全感知不到 Admin 的存在。

### 核心变更一览

| 维度 | 现状 | 目标 |
|------|------|------|
| Admin 身份 | `users` 表 `role=admin` | 环境变量 `ADMIN_USER` + `ADMIN_PASSWORD`，不入库 |
| Admin 认证 | 与用户共用 JWT（Claims 里带 UserID） | 独立 JWT，Claims 带 `role: "admin"`，无 UserID |
| Admin API | `/api/v1/admin/*`，共用 auth middleware | `/admin-api/v1/*`，独立 middleware |
| Admin 前端 | 嵌套在聊天 SPA 的 `AdminPage` 组件 | 独立 SPA，`/admin/*` 路由 |
| `/api/v1/users` | 返回全量用户（含 admin） | 移除此接口 |
| @mention / DM | 依赖 `loadUsers()` 全量列表 | 改用频道 members API |

## 2. 架构变更

### 2.1 Admin 认证模块

**环境变量：**

```
ADMIN_USER=admin          # 替代现有 ADMIN_EMAIL
ADMIN_PASSWORD=xxx        # 保持不变
```

将 `config.go` 中 `AdminEmail` 改为 `AdminUser`，环境变量从 `ADMIN_EMAIL` 改为 `ADMIN_USER`（用户名而非邮箱，与 UI 线框图一致）。

**登录接口：** `POST /admin-api/v1/auth/login`

```json
// Request
{ "username": "admin", "password": "xxx" }

// Response 200
{ "token": "eyJ..." }
// JWT Claims: { role: "admin", iat, exp }
```

- 直接比对环境变量明文（不走 DB，不做 bcrypt），密码错误返回 401
- 签发独立 JWT，使用与用户 JWT 相同的 `JWT_SECRET`，但 Claims 结构不同（无 UserID/Email，仅 `role: "admin"`）
- Token 通过 `borgee_admin_token` cookie 设置（与用户 cookie `borgee_token` 分开）

**Admin Auth Middleware：**

新建 `AdminAuthMiddleware`，仅解析 `borgee_admin_token` cookie，验证 JWT 中 `role == "admin"`。与现有 `AuthMiddleware` 完全独立。

### 2.2 Admin API 路径迁移

所有 admin 接口从 `/api/v1/admin/*` 迁移到 `/admin-api/v1/*`，使用 `AdminAuthMiddleware` 保护。

**新增接口：**

| 接口 | 用途 |
|------|------|
| `POST /admin-api/v1/auth/login` | Admin 登录 |
| `POST /admin-api/v1/auth/logout` | Admin 登出 |
| `GET /admin-api/v1/auth/me` | 验证 Admin session |
| `GET /admin-api/v1/stats` | 概览页统计（用户总数 / 频道总数 / 在线数） |
| `GET /admin-api/v1/users/{id}/agents` | User Detail 页查看用户的 agent 列表 |

**安全红线实施：** `sanitizeUserAdmin()` 中移除 `api_key` 字段输出（第 78-79 行）。`/admin-api/v1/users/{id}/agents` 返回的 agent 信息也不包含 `api_key`。

### 2.3 Admin 前端独立 SPA

**路由结构：**

```
/admin                → Admin 登录页
/admin/dashboard      → 概览
/admin/users          → 用户管理
/admin/users/:id      → User Detail
/admin/channels       → 频道管理
/admin/invites        → 邀请码管理
/admin/settings       → 系统设置
```

**实现方式：** 在现有 Vite 项目中新增入口点 `admin.html` + `src/admin/main.tsx`，使用 Vite 的多入口 (`build.rollupOptions.input`) 构建为独立 bundle。与用户侧共享 UI 基础组件（按钮、表格、表单），但路由和状态管理完全独立。

**Admin 前端不包含：** WebSocket 连接、聊天相关组件、`AppContext`。

### 2.4 用户侧前端适配

**移除 `loadUsers()` 依赖：**

- 删除 `AppContext` 中的 `users` state、`SET_USERS` action、`loadUsers` callback、`fetchUsers()` API 调用
- 删除 `App.tsx:70` 的 `await actions.loadUsers()` 调用
- 删除 `lib/api.ts:253-255` 的 `fetchUsers()` 函数

**@mention 选人器改造：**

现有 `MentionList.tsx` / `useMention.ts` 改为从当前频道的 `GET /api/v1/channels/{channelId}/members` 获取候选人列表。该接口已存在（`channels.go:67`），返回 `ChannelMemberInfo`，包含 `display_name`、`avatar_url` 等必要字段。

**DM 列表适配：**

- 创建新 DM：从当前频道成员列表中选人（复用 members API）
- 已有 DM 列表：基于 `loadDmChannels()` 展示，不依赖全量用户列表（现有实现已满足）

### 2.5 DB Schema 变更

**数据可清空重建**（已确认），因此无需写迁移脚本。

变更内容：
- `users` 表：`role` 字段取值范围从 `admin | member` 改为 `member | agent`。不再有 `role=admin` 的行
- Seed 逻辑：删除现有 `SeedAdmin` / `EnsureAdmin` 相关代码（DB 中不再创建 admin 用户）
- `handleCreateUser`：移除创建 `role=admin` 的能力（第 113 行，只允许 `member` 和 `agent`）
- `handleUpdateUser`：移除将用户角色改为 `admin` 的能力（第 179-180 行）

## 3. 详细设计

### 3.1 后端 Admin 认证模块

**文件：** 新建 `packages/server-go/internal/api/admin_auth.go`

```go
type AdminAuthHandler struct {
    Config *config.Config
    Logger *slog.Logger
}

type AdminClaims struct {
    Role string `json:"role"`
    jwt.RegisteredClaims
}
```

- `POST /admin-api/v1/auth/login`：比对 `Config.AdminUser` / `Config.AdminPassword`，签发 AdminClaims JWT
- `POST /admin-api/v1/auth/logout`：清除 `borgee_admin_token` cookie
- `GET /admin-api/v1/auth/me`：返回 `{ role: "admin", username: Config.AdminUser }`
- `AdminAuthMiddleware`：解析 `borgee_admin_token`，验证 AdminClaims，不查 DB

**配置变更：** `config.go` 中 `AdminEmail` → `AdminUser`，环境变量 `ADMIN_EMAIL` → `ADMIN_USER`。

**启动校验：** 当 `ADMIN_USER` 或 `ADMIN_PASSWORD` 为空时，admin API 不注册路由，日志输出 warning。

### 3.2 后端 Admin API 路由迁移

**文件：** 修改 `packages/server-go/internal/api/admin.go`

- `RegisterRoutes` 中所有路由前缀从 `/api/v1/admin/` 改为 `/admin-api/v1/`
- `authMw` 参数改为 `AdminAuthMiddleware`
- 移除 `requireAdmin()` 方法（Admin 身份由 middleware 保证）
- `AdminHandler` 不再需要依赖 `auth.UserFromContext()`（admin 不在 users 表中）

**新增 handler：**

- `handleStats`：`GET /admin-api/v1/stats` — 聚合查询 users count、channels count、online count
- `handleListUserAgents`：`GET /admin-api/v1/users/{id}/agents` — 调用现有 `Store.ListAgentsByOwner(id)`，返回 agent 列表（排除 api_key）

**`sanitizeUserAdmin()` 变更（安全红线）：**

```go
// 删除这两行：
// if u.APIKey != nil {
//     m["api_key"] = *u.APIKey
// }
```

### 3.3 前端 Admin 独立 SPA

**目录结构：**

```
packages/client/
├── admin.html              # Admin 入口 HTML
├── src/
│   ├── admin/
│   │   ├── main.tsx        # Admin React 入口
│   │   ├── AdminApp.tsx    # 路由 + Layout
│   │   ├── api.ts          # Admin API 客户端
│   │   ├── auth.ts         # Admin 登录状态管理
│   │   └── pages/
│   │       ├── LoginPage.tsx
│   │       ├── DashboardPage.tsx
│   │       ├── UsersPage.tsx
│   │       ├── UserDetailPage.tsx
│   │       ├── ChannelsPage.tsx
│   │       ├── InvitesPage.tsx
│   │       └── SettingsPage.tsx
│   └── ...（现有用户侧代码）
```

**Vite 配置：**

```js
build: {
  rollupOptions: {
    input: {
      main: 'index.html',
      admin: 'admin.html',
    }
  }
}
```

**Go 静态文件服务：** 对 `/admin` 和 `/admin/*` 路径（非 `/admin-api/`）返回 `admin.html`，实现 SPA 路由。

### 3.4 前端用户侧适配

**AppContext 清理：**

1. 删除 `state.users` 和 `state.userMap`
2. 删除 `SET_USERS` dispatch action
3. 删除 `loadUsers` callback 和 `fetchUsers` API 函数
4. 从 `App.tsx` 初始化流程中移除 `loadUsers()` 调用

**MentionList / useMention 改造：**

在 `useMention.ts` 中：
- 当用户输入 `@` 时，调用 `GET /api/v1/channels/{currentChannelId}/members` 获取频道成员
- 缓存结果（按 channelId），切换频道时刷新
- 过滤逻辑不变（按 display_name 前缀匹配）

**MentionPicker 改造：**

数据源从 `state.users` 改为频道 members 缓存。

**DM 选人（openDm）：**

- 新建 DM 时，提供"从当前频道成员选人"入口
- 已有 DM 列表不受影响（基于 `dmChannels` state）

### 3.5 DB Schema 与 Seed 变更

- `User.Role` 有效值：`member`（移除 `admin`）
- 删除启动时创建 admin 用户的 seed 逻辑
- `handleCreateUser`：`role` 校验只接受 `member`
- `handleUpdateUser`：`role` 校验只接受 `member`
- `users.ListAdminUsers()`：查询中排除 `role=admin`（过渡期兜底，正常情况下不应有此类数据）

## 4. 接口清单

### 4.1 Admin API — 新增

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/admin-api/v1/auth/login` | Admin 登录 |
| POST | `/admin-api/v1/auth/logout` | Admin 登出 |
| GET | `/admin-api/v1/auth/me` | 验证 Admin session |
| GET | `/admin-api/v1/stats` | 概览页统计数据 |
| GET | `/admin-api/v1/users/{id}/agents` | 查看用户的 agent 列表（只读，无 api_key） |

### 4.2 Admin API — 路径迁移（14 个接口）

| 旧路径 | 新路径 |
|--------|--------|
| `GET /api/v1/admin/users` | `GET /admin-api/v1/users` |
| `POST /api/v1/admin/users` | `POST /admin-api/v1/users` |
| `PATCH /api/v1/admin/users/{id}` | `PATCH /admin-api/v1/users/{id}` |
| `DELETE /api/v1/admin/users/{id}` | `DELETE /admin-api/v1/users/{id}` |
| `POST /api/v1/admin/users/{id}/api-key` | `POST /admin-api/v1/users/{id}/api-key` |
| `DELETE /api/v1/admin/users/{id}/api-key` | `DELETE /admin-api/v1/users/{id}/api-key` |
| `GET /api/v1/admin/users/{id}/permissions` | `GET /admin-api/v1/users/{id}/permissions` |
| `POST /api/v1/admin/users/{id}/permissions` | `POST /admin-api/v1/users/{id}/permissions` |
| `DELETE /api/v1/admin/users/{id}/permissions` | `DELETE /admin-api/v1/users/{id}/permissions` |
| `POST /api/v1/admin/invites` | `POST /admin-api/v1/invites` |
| `GET /api/v1/admin/invites` | `GET /admin-api/v1/invites` |
| `DELETE /api/v1/admin/invites/{code}` | `DELETE /admin-api/v1/invites/{code}` |
| `GET /api/v1/admin/channels` | `GET /admin-api/v1/channels` |
| `DELETE /api/v1/admin/channels/{id}/force` | `DELETE /admin-api/v1/channels/{id}/force` |

### 4.3 User API — 删除

| 方法 | 路径 | 处置 |
|------|------|------|
| `GET /api/v1/users` | 删除 | 信息泄露风险，由频道 members API 替代 |

### 4.4 User API — 保留（无变更）

| 方法 | 路径 |
|------|------|
| `GET /api/v1/me/permissions` | 保留 |
| `GET /api/v1/online` | 保留（仅返回 user IDs，风险低） |
| `GET /api/v1/channels/{channelId}/members` | 保留（@mention / DM 选人的数据源） |

## 5. 验收标准（技术视角）

| PRD 验收标准 | 技术验证方法 |
|-------------|-------------|
| admin 只能登录管理后台 | `POST /admin-api/v1/auth/login` 返回的 JWT 无法通过 `AuthMiddleware`；`/api/v1/*` 接口不接受 `borgee_admin_token` |
| admin 无法发送消息、加入频道 | Admin 不在 `users` 表中，WebSocket 连接的 `AuthMiddleware` 拒绝 admin token |
| admin 不出现在用户列表 | `users` 表无 `role=admin` 行；`GET /api/v1/users` 已删除；频道 members API 不会返回 admin |
| 普通用户无法访问管理后台 | `/admin-api/v1/*` 仅接受 `borgee_admin_token`（AdminAuthMiddleware） |
| 管理后台有独立入口 | `/admin` 路由返回 `admin.html`，独立 SPA |
| 创建用户只能创建 user | `handleCreateUser` 校验 `role in (member, agent)`，前端表单无 role 选择 |
| User Detail 展示用户信息 + agent | `GET /admin-api/v1/users/{id}` + `GET /admin-api/v1/users/{id}/agents` |
| API 接口不返回 api_key | `sanitizeUserAdmin()` 移除 `api_key` 输出；agents 接口排除 `api_key` |
| @mention 基于频道成员 | `useMention.ts` 改用 `GET /api/v1/channels/{id}/members` |
| 私信基于已有会话 | `dmChannels` state 不依赖 `users` state |

## 6. 迁移方案

**数据可以清空重建**（建军已确认）。

执行步骤：
1. 部署新版后端（包含新的 admin 认证和 API 路径）
2. 设置环境变量 `ADMIN_USER` 和 `ADMIN_PASSWORD`
3. 清空 DB（或删除 `data/collab.db`），服务启动时自动 migrate
4. Admin 通过 `/admin` 登录管理后台，创建初始用户和邀请码
5. 用户通过邀请码注册

**回滚方案：** 回退代码版本 + 清空 DB 重建。

## 7. 任务拆分

### Phase 1：后端 Admin 认证分离（1d）

| # | 子任务 | 估时 |
|---|--------|------|
| 1.1 | `config.go`：`AdminEmail` → `AdminUser`，环境变量改名 | 0.5h |
| 1.2 | 新建 `admin_auth.go`：AdminClaims、登录/登出/me 接口 | 2h |
| 1.3 | 新建 `AdminAuthMiddleware`：解析 `borgee_admin_token` | 1h |
| 1.4 | 删除 DB admin seed 逻辑 | 0.5h |
| 1.5 | 单元测试 | 2h |

### Phase 2：后端 Admin API 迁移（0.5d）

| # | 子任务 | 估时 |
|---|--------|------|
| 2.1 | `admin.go` 路由前缀迁移 `/api/v1/admin/*` → `/admin-api/v1/*` | 1h |
| 2.2 | 切换到 AdminAuthMiddleware，移除 `requireAdmin` | 0.5h |
| 2.3 | `sanitizeUserAdmin` 移除 api_key 输出 | 0.5h |
| 2.4 | 新增 `GET /admin-api/v1/stats`、`GET /admin-api/v1/users/{id}/agents` | 1h |
| 2.5 | `handleCreateUser` / `handleUpdateUser` role 校验收紧 | 0.5h |
| 2.6 | 删除 `/api/v1/users` 接口 | 0.5h |

### Phase 3：前端 Admin 独立 SPA（2d）

| # | 子任务 | 估时 |
|---|--------|------|
| 3.1 | Vite 多入口配置 + `admin.html` + Go 静态文件路由 | 2h |
| 3.2 | Admin 登录页 + auth 状态管理 | 2h |
| 3.3 | Dashboard 概览页 | 2h |
| 3.4 | 用户管理页（列表 + 创建弹窗） | 3h |
| 3.5 | User Detail 页（用户信息 + Agent 列表只读） | 2h |
| 3.6 | 邀请码管理页 | 2h |
| 3.7 | 频道管理页 | 1.5h |
| 3.8 | 系统设置页（迁移现有功能） | 1.5h |

### Phase 4：前端用户侧适配（1d）

| # | 子任务 | 估时 |
|---|--------|------|
| 4.1 | 移除 `loadUsers` / `fetchUsers` / `users` state | 1h |
| 4.2 | @mention 选人器改用频道 members API | 3h |
| 4.3 | DM 选人改为从频道成员选 | 2h |
| 4.4 | 删除旧 `AdminPage.tsx` 及 `admin/` 子组件 | 0.5h |
| 4.5 | 回归测试（聊天、@mention、DM 功能） | 1.5h |

### Phase 5：清理与验收（0.5d）

| # | 子任务 | 估时 |
|---|--------|------|
| 5.1 | 删除旧 `/api/v1/admin/*` 路由残留 | 0.5h |
| 5.2 | 更新 PermissionsTab 中的 loadUsers（已迁入 admin SPA） | 0.5h |
| 5.3 | 端到端验收测试 | 2h |

**总估时：约 5 个工作日**

### 依赖关系

```
Phase 1 → Phase 2 → Phase 3（可与 Phase 4 并行）
                   ↘ Phase 4
Phase 3 + Phase 4 → Phase 5
```
