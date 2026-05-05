# P1 技术设计：Agent 归属与权限系统

## 1. 背景与问题

当前系统已实现基础的用户、频道、消息、成员管理功能（P0），但缺乏细粒度的权限控制和 Agent 归属机制。P1 需要解决以下核心问题：

- **权限缺位**：所有已登录用户拥有相同能力，无法区分普通成员、管理员，也无法约束 Agent 的行为边界
- **Agent 无主**：Agent 账户与人类账户混在一起，没有归属关系，审计、配额、清理都无从下手
- **频道治理缺失**：没有频道删除能力，Creator 对自己创建的频道也没有管理权
- **准入失控**：任何人只要能访问服务就能注册，不符合团队协作工具的准入模型
- **管理界面缺失**：admin 没有独立的后台入口，管理操作只能靠脚本或数据库直连

P1 不引入新基础设施（不加 Redis、不换 DB），在现有 Fastify + SQLite + React 技术栈上完成。

---

## 2. 目标（可验证的验收标准）

| # | 目标 | 验收标准 |
|---|---|---|
| G1 | 基于 `role + user_permissions` 表的权限中间件 | `requirePermission('channel.create')` 对无权用户返回 403；admin 任意权限通过；单测覆盖 member/agent/admin 三种角色 |
| G2 | Agent 归属关系 | `users.owner_id` 存在；Agent 必须通过 owner 创建；owner 只能拉自己的 Agent 进入自己已在的频道；Agent 创建的频道 `created_by = agent.id` 但管理权归 owner |
| G3 | 频道软删除 | `DELETE /api/v1/channels/:id` 幂等；`#general` 和 DM 返回 409；删除后 WS + SSE 推送 `channel_deleted`；客户端从列表移除 |
| G4 | Creator 默认权限 | member 创建频道后自动拥有 `channel.delete`、`channel.manage_members`、`channel.manage_visibility` 三项 scoped 权限；agent 创建频道不获得这些权限（归 owner） |
| G5 | 邀请码注册 | admin 可生成一次性邀请码；注册端点必须校验邀请码+邮箱唯一；使用后邀请码作废；新用户自动获得默认 member 权限 |
| G6 | Admin 后台 `/admin` | 独立页面；非 admin 访问返回 403；可管理用户、邀请码、查看频道、强制删除频道 |
| G7 | 迁移安全 | 迁移脚本幂等、在事务中执行；已有数据（users、channels）不丢失；首次启动自动跑 |

---

## 3. 方案设计

### 3.1 数据模型变更

在现有 schema 基础上增加/修改以下表。所有变更通过一次迁移脚本 `migrations/db.ts initSchema() 内的 P1 migration 块` 完成。

```sql
-- 3.1.1 users 表扩展：Agent 归属
ALTER TABLE users ADD COLUMN owner_id TEXT REFERENCES users(id);
-- owner_id = NULL 表示人类用户或独立 Agent（P1 强制 Agent 必须有 owner）
-- role = 'admin' | 'member' | 'agent'（已有）
CREATE INDEX IF NOT EXISTS idx_users_owner_id ON users(owner_id);

-- 3.1.2 权限表
CREATE TABLE IF NOT EXISTS user_permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission  TEXT NOT NULL,           -- e.g. 'channel.create', 'channel.delete'
  scope       TEXT NOT NULL DEFAULT '*', -- '*' 或 'channel:<id>'
  granted_by  TEXT REFERENCES users(id),
  granted_at  INTEGER NOT NULL,
  UNIQUE(user_id, permission, scope)
);
CREATE INDEX IF NOT EXISTS idx_user_permissions_user ON user_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_lookup ON user_permissions(user_id, permission, scope);

-- 3.1.3 邀请码表
CREATE TABLE IF NOT EXISTS invite_codes (
  code        TEXT PRIMARY KEY,        -- 随机 16 字符 base32
  created_by  TEXT NOT NULL REFERENCES users(id),
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER,                 -- NULL = 永不过期
  used_by     TEXT REFERENCES users(id),
  used_at     INTEGER,
  note        TEXT                      -- admin 备注，方便管理
);
CREATE INDEX IF NOT EXISTS idx_invite_codes_used ON invite_codes(used_by);
```

**权限命名空间约定**：

| permission | scope 语义 | 说明 |
|---|---|---|
| `channel.create` | `*` | 可创建频道 |
| `channel.delete` | `*` 或 `channel:<id>` | 删除频道 |
| `channel.manage_members` | `*` 或 `channel:<id>` | 增删频道成员 |
| `channel.manage_visibility` | `*` 或 `channel:<id>` | 改公开/私有 |
| `message.send` | `*` | 可发消息 |
| `agent.manage` | `*` | 管理自己的 Agent（member 默认有） |

admin (`role='admin'`) 不走权限表，代码直接短路通过。

### 3.2 权限中间件 `requirePermission`

位置：`server/src/middleware/permissions.ts`

```ts
// 伪代码
export function requirePermission(permission: string, scopeResolver?: (req) => string) {
  return async (req, reply) => {
    const user = req.user; // 来自已有 auth preHandler
    if (!user) return reply.code(401).send({ error: 'unauthorized' });
    if (user.role === 'admin') return; // admin = *

    const scope = scopeResolver ? scopeResolver(req) : '*';
    const ok = db.prepare(`
      SELECT 1 FROM user_permissions
      WHERE user_id = ?
        AND permission = ?
        AND (scope = '*' OR scope = ?)
      LIMIT 1
    `).get(user.id, permission, scope);

    if (!ok) return reply.code(403).send({ error: 'forbidden', permission, scope });
  };
}
```

使用示例：

```ts
fastify.post('/api/v1/channels', {
  preHandler: [authRequired, requirePermission('channel.create')]
}, createChannelHandler);

fastify.delete('/api/v1/channels/:channelId', {
  preHandler: [authRequired, requirePermission('channel.delete', req => `channel:${req.params.channelId}`)]
}, deleteChannelHandler);
```

**默认权限授予**（注册或 Agent 创建时，在事务内插入）：

- `member` 注册：`(channel.create, *)`, `(message.send, *)`, `(agent.manage, *)`
- `agent` 创建：`(message.send, *)`
- `admin` 注册：不插任何行（通配）

**Creator 权限授予**（创建频道成功后）：

```ts
// 仅当 creator.role === 'member' 时授予，agent 创建频道不授予
if (creator.role === 'member') {
  const scope = `channel:${newChannelId}`;
  for (const p of ['channel.delete', 'channel.manage_members', 'channel.manage_visibility']) {
    insertPermission(creator.id, p, scope, creator.id);
  }
}
```

### 3.3 Agent 归属

#### 3.3.1 数据约束

- `users.owner_id` 引用 `users.id`
- 应用层强制：`role='agent'` 必须有 `owner_id` 且指向 `role='member'` 或 `role='admin'` 用户
- `role='member'/'admin'` 的 `owner_id` 必须为 NULL

#### 3.3.2 API

```
POST   /api/v1/agents              # member 创建自己的 agent
GET    /api/v1/agents              # 列出自己拥有的 agents（admin 可 ?owner_id=xxx）
DELETE /api/v1/agents/:agentId     # owner 或 admin 删除
POST   /api/v1/agents/:agentId/rotate-api-key
GET    /api/v1/agents/:agentId/permissions    # owner 查看 agent 权限
PUT    /api/v1/agents/:agentId/permissions    # owner 配置 agent 权限
```

**Owner 配置 Agent 权限**：

```
PUT /api/v1/agents/:agentId/permissions
Body: { permissions: ["message.send", "channel.create", ...] }
```

校验：
- `req.user.id === agent.owner_id`（owner 只能管自己的 agent）
- admin 可以管任何 agent 的权限
- 全量覆盖（和 admin 权限管理 API 一致）

创建 Agent 请求体：`{ display_name, avatar_url? }`。服务端：

1. 校验 `req.user.role in ('member','admin')` 且有 `agent.manage` 权限
2. 生成 `id`、`api_key`，`owner_id = req.user.id`，`role = 'agent'`
3. 事务内插入 users 行 + 默认权限 `(message.send, *)`
4. 返回一次性 `api_key`（后续查询不再返回明文）

#### 3.3.3 Agent 入频道校验

**两条路径都必须校验**：`POST /channels`（创建时指定 member_ids）和 `POST /channels/:id/members`（后续添加）。

抽取公共 helper：

```ts
function validateAgentMembership(caller: User, agentUser: User, channelId: string, memberIds: Set<string>): void {
  // caller 必须是 agent 的 owner（或 admin）
  if (caller.role !== 'admin' && agentUser.owner_id !== caller.id) {
    throw { code: 403, error: 'can_only_add_own_agent' };
  }
  // owner 必须已在频道内（或正在被加入同一批 member_ids）
  if (!isChannelMember(channelId, agentUser.owner_id) && !memberIds.has(agentUser.owner_id)) {
    throw { code: 409, error: 'owner_must_join_first' };
  }
}
```

**POST /channels 创建时**：遍历 `member_ids`，对每个 `role=agent` 的用户调用 `validateAgentMembership`。

**POST /channels/:id/members 添加时**：同样调用。

现有 `POST /api/v1/channels/:id/members` 增加逻辑：

```ts
async function addMemberHandler(req) {
  const { channelId } = req.params;
  const { userId: targetId } = req.body;
  const caller = req.user;
  const target = getUserById(targetId);

  // 1. 已有校验：caller 必须有 channel.manage_members on channel:{id}
  // 2. 新增：若 target.role === 'agent'
  if (target.role === 'agent') {
    //   - caller 必须是 target.owner_id 本人（或 admin）
    if (caller.role !== 'admin' && target.owner_id !== caller.id) {
      return reply.code(403).send({ error: 'can_only_add_own_agent' });
    }
    //   - owner 必须已在该频道内
    const ownerInChannel = isChannelMember(channelId, target.owner_id);
    if (!ownerInChannel) {
      return reply.code(409).send({ error: 'owner_must_join_first' });
    }
  }
  // ... 继续 insert channel_members
}
```

#### 3.3.4 Agent 创建的资源归属

消息：`messages.sender_id = agent.id`（保持不变，便于审计）。
频道：`channels.created_by = agent.id`，但 Creator 管理权限只授予 `owner_id`：

```ts
if (creator.role === 'agent') {
  const owner = getUserById(creator.owner_id);
  const scope = `channel:${newChannelId}`;
  for (const p of ['channel.delete', 'channel.manage_members', 'channel.manage_visibility']) {
    insertPermission(owner.id, p, scope, creator.id); // granted_by 记录 agent
  }
}
```

### 3.4 频道删除

#### 3.4.1 API

```
DELETE /api/v1/channels/:channelId
  preHandler: requirePermission('channel.delete', req => `channel:${req.params.channelId}`)
```

处理逻辑：

```ts
async function deleteChannelHandler(req, reply) {
  const { channelId } = req.params;
  const ch = getChannelById(channelId);
  if (!ch) return reply.code(404).send({ error: 'not_found' });
  if (ch.deleted_at) return reply.code(204).send(); // 幂等

  // 硬约束
  if (ch.name === 'general') return reply.code(409).send({ error: 'cannot_delete_general' });
  if (ch.type === 'dm') return reply.code(409).send({ error: 'cannot_delete_dm' });

  const now = Date.now();
  const memberIds = db.transaction(() => {
    db.prepare('UPDATE channels SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL')
      .run(now, channelId);
    const rows = db.prepare('SELECT user_id FROM channel_members WHERE channel_id = ?')
      .all(channelId);
    insertEvent({ type: 'channel_deleted', channel_id: channelId, actor_id: req.user.id, ts: now });
    return rows.map(r => r.user_id);
  })();

  // 广播
  for (const uid of memberIds) {
    wsBroadcastToUser(uid, { type: 'channel_deleted', channel_id: channelId });
  }
  return reply.code(204).send();
}
```

#### 3.4.2 读取端过滤

所有 channel list / get / message send 端点增加 `WHERE deleted_at IS NULL` 过滤；已删除频道一律 404 或 410。

#### 3.4.3 前端

- `MembersModal` 已有删除按钮（a875547 commit），绑定 `DELETE` 端点
- 收到 `channel_deleted` WS/SSE 事件：
  - 从 channels store 移除该频道
  - 若当前选中频道被删，切换到 `#general`
  - Toast: "Channel #xxx was deleted"

### 3.5 独立 Admin 后台

#### 3.5.1 路由

- 前端：`/admin`（React Router），入口在顶部导航（仅 admin 可见）
- 子路由：
  - `/admin/users` — 用户列表
  - `/admin/invites` — 邀请码管理
  - `/admin/channels` — 频道列表（含已删除）
  - `/admin/permissions` — 手动授予/撤销权限

#### 3.5.2 访问控制

```ts
// 前端：AdminLayout 组件
if (currentUser.role !== 'admin') return <Navigate to="/" />;

// 后端：所有 /api/v1/admin/* 端点 preHandler
async function requireAdmin(req, reply) {
  if (req.user?.role !== 'admin') return reply.code(403).send({ error: 'admin_only' });
}
```

#### 3.5.3 Admin API

```
GET    /api/v1/admin/users                  # 用户列表（含 agents）
PATCH  /api/v1/admin/users/:id              # 改 role/display_name/disabled
POST   /api/v1/admin/invites                # 生成邀请码
GET    /api/v1/admin/invites                # 列表
DELETE /api/v1/admin/invites/:code          # 作废
GET    /api/v1/admin/channels               # 全部频道（含 deleted_at）
DELETE /api/v1/admin/channels/:id/force     # 强制删除 #general 之外的任意频道
POST   /api/v1/admin/permissions            # 授予
DELETE /api/v1/admin/permissions/:id        # 撤销
```

#### 3.5.4 前端页面（React + Vite）

沿用现有 UI 风格。每个页面一个简单表格 + 操作按钮。不引入新 UI 库。

### 3.6 邀请码

#### 3.6.1 生成

```ts
// POST /api/v1/admin/invites
// body: { note?, expires_at? }
const code = crypto.randomBytes(10).toString('hex').slice(0, 16).toLowerCase(); // 16 chars
db.prepare(`INSERT INTO invite_codes (code, created_by, created_at, expires_at, note) VALUES (?,?,?,?,?)`)
  .run(code, admin.id, Date.now(), expires_at ?? null, note ?? null);
```

#### 3.6.2 注册流程

```
POST /api/v1/auth/register
body: { invite_code, email, password, display_name }
```

1. 查 `invite_codes`：存在、`used_by IS NULL`、未过期 → 否则 400
2. 查 `users.email` 唯一性
3. 事务：
   - 插入 users（role='member', owner_id=NULL）
   - 插入默认权限 `(channel.create,*)`, `(message.send,*)`, `(agent.manage,*)`
   - 更新 invite_code `used_by, used_at`
   - 自动加入 `#general`
4. 返回登录 token

注册端点本身 **不需要** auth preHandler，但必须做速率限制（每 IP 10/min，沿用已有 rate-limit 插件或加 in-memory 令牌桶）。

### 3.7 数据迁移

位置：`server/src/db/migrations/db.ts initSchema() 内的 P1 migration 块` + 启动时执行器。

```sql
BEGIN;

-- 1. schema
ALTER TABLE users ADD COLUMN owner_id TEXT REFERENCES users(id);
-- (user_permissions, invite_codes 见 3.1)

-- 2. 回填默认权限（幂等：INSERT OR IGNORE）
INSERT OR IGNORE INTO user_permissions (user_id, permission, scope, granted_by, granted_at)
  SELECT id, 'channel.create', '*', id, strftime('%s','now')*1000 FROM users WHERE role='member';
INSERT OR IGNORE INTO user_permissions (user_id, permission, scope, granted_by, granted_at)
  SELECT id, 'message.send', '*', id, strftime('%s','now')*1000 FROM users WHERE role IN ('member','agent');
INSERT OR IGNORE INTO user_permissions (user_id, permission, scope, granted_by, granted_at)
  SELECT id, 'agent.manage', '*', id, strftime('%s','now')*1000 FROM users WHERE role='member';

-- 3. 回填 Creator 权限（现有频道的 created_by 如果是 member 则授权）
INSERT OR IGNORE INTO user_permissions (user_id, permission, scope, granted_by, granted_at)
  SELECT c.created_by, 'channel.delete', 'channel:'||c.id, c.created_by, strftime('%s','now')*1000
  FROM channels c JOIN users u ON u.id = c.created_by
  WHERE u.role = 'member' AND c.deleted_at IS NULL AND c.name != 'general';
-- 同上 channel.manage_members, channel.manage_visibility

COMMIT;
```

**幂等性保证**：
- `ALTER TABLE ADD COLUMN` 用迁移版本号表 `initSchema()（沿用现有 db.ts 的 ad-hoc migration 模式：PRAGMA table_info + ALTER TABLE ADD COLUMN IF NOT EXISTS）(version)` 记录，已执行跳过
- 所有 INSERT 用 `OR IGNORE`，依赖 `UNIQUE(user_id, permission, scope)`

**事务安全**：单个 BEGIN/COMMIT 包裹；失败回滚，版本号不写入。

**执行时机**：server 启动时 `initSchema()（在现有 initSchema 函数内追加新的 migration 块）`，所有版本号 < current 的文件顺序执行；已在 P0 做过一次（`现有 db.ts initSchema() 初始化逻辑`），沿用机制。

---

## 4. API 变更清单

| 方法 | 路径 | 权限 | 新/改 |
|---|---|---|---|
| POST | `/api/v1/auth/register` | 公开（邀请码） | 新 |
| POST | `/api/v1/channels` | `channel.create` | 改（加权限中间件） |
| DELETE | `/api/v1/channels/:id` | `channel.delete` on `channel:<id>` | 已存在，加中间件 |
| POST | `/api/v1/channels/:id/members` | `channel.manage_members` | 改（加 Agent 校验） |
| DELETE | `/api/v1/channels/:id/members/:uid` | `channel.manage_members` | 改 |
| PATCH | `/api/v1/channels/:id` (visibility) | `channel.manage_visibility` | 改 |
| POST | `/api/v1/agents` | `agent.manage` | 新 |
| GET | `/api/v1/agents` | 认证 | 新 |
| DELETE | `/api/v1/agents/:id` | owner 或 admin | 新 |
| POST | `/api/v1/agents/:id/rotate-api-key` | owner 或 admin | 新 |
| GET/POST/DELETE/PATCH | `/api/v1/admin/*` | admin only | 新（见 3.5.3） |

WS 事件新增：`channel_deleted`（已在 P0 的 `CHANNEL_CHANGE_KINDS` 预留）。

---

## 5. 备选方案

| 方向 | 备选 | 为什么不选 |
|---|---|---|
| 权限存储 | RBAC（roles 表 + role_permissions 表 + user_roles 表） | 当前只有 3 个角色，直接查 user_permissions 足够；引入 roles 表增加 2 次 JOIN，收益不足 |
| 权限表达式 | Casbin / CEL 规则引擎 | 过度工程；P1 只需 permission + 简单 scope |
| Agent 归属 | 独立 agents 表 | 需要同步两张表，API key、display_name 等字段重复；`users + owner_id` 最简 |
| 频道删除 | 硬删除 + 归档表 | 数据迁移复杂；软删除靠 `deleted_at` 一列足够，审计可直接查 events |
| Admin 后台 | 嵌入主应用同一路由 | 混淆普通用户 UI；独立 `/admin` 更清晰，后续可独立部署 |
| 邀请码 | OAuth / SSO | 团队协作工具早期不值得；邀请码实现 < 100 行 |
| 准入 | 公开注册 + admin 事后审批 | 用户体验差（需要等待）；邀请码即发即用 |

---

## 6. 测试策略

### 6.1 单元测试

- `requirePermission` 中间件：admin 短路 / member 有权限 / member 无权限 / scope 匹配与通配
- Creator 权限授予：member 创建获得三项；agent 创建 owner 获得三项、agent 本身不获得
- Agent 入频道校验：owner 不在频道 → 409；非 owner 拉别人的 agent → 403
- 邀请码：重复使用 → 400；过期 → 400；并发使用（两请求同一 code）→ 只有一个成功（DB 行锁 / UNIQUE）

### 6.2 集成测试（sqlite in-memory + fastify.inject）

- 完整注册流（邀请码→注册→登录→创建频道→删除频道）
- Agent 完整流（member 创建 agent→agent 登录发消息→owner 拉 agent 入频道→删除 agent）
- 软删频道：删除后 GET/POST 到该频道 → 404；WS 推送收到 `channel_deleted`
- 迁移脚本：在 P0 dump 上跑 migrate → 所有已有用户得到默认权限且无重复

### 6.3 端到端（Playwright，选做）

- Admin 登录 → 进入 `/admin/invites` → 生成邀请码 → 退出 → 用邀请码注册 → 进入主应用
- Member 创建频道 → 看到删除按钮 → 点击 → 频道消失；另一个 tab 同步消失

### 6.4 手动冒烟

- Agent API key 流：curl 发消息成功；rotate 后旧 key 失效
- `#general` 删除返回 409；DM 删除返回 409

---

## 7. Task Breakdown（总 ~30h）

标注 `[P]` 可与前项并行。

| # | Task | 预估 | 依赖 |
|---|---|---|---|
| T1 | 迁移脚本 `db.ts initSchema() 内的 P1 migration 块` + 启动执行器更新 | 3h | — |
| T2 | `requirePermission` 中间件 + 单测 | 2h | T1 |
| T3 | 默认权限授予 helper（注册/Agent 创建时调用）+ Creator 权限授予 helper | 2h | T1 |
| T4 | 邀请码 CRUD + 注册端点改造 + 速率限制 | 4h | T1, T3 |
| T5 | Agent CRUD API（create/list/delete/rotate-key）+ 单测 | 3h | T1, T2, T3 |
| T6 | Channel members 端点加 Agent 入频道校验 | 1.5h | T5 |
| T7 | 频道删除端点绑定权限中间件 + 读取端过滤 `deleted_at` 核查 | 2h `[P with T5]` | T2 |
| T8 | Admin API: `/api/v1/admin/users|invites|channels|permissions` | 4h `[P with T5/T7]` | T2 |
| T9 | 前端权限 hook `useCan(permission, scope)` + 按钮级显示控制 | 2h | T2（API 稳定） |
| T10 | 前端 `/admin` 路由 + 4 个管理页面（简表格） | 5h `[P with T9]` | T8 |
| T11 | 前端邀请码注册页 + 登录页入口 | 2h `[P with T10]` | T4 |
| T12 | 前端 Agent 管理页面（创建/列表/删除） | 2h `[P with T10/T11]` | T5 |
| T13 | E2E 冒烟 + 文档更新（README + API 列表） | 2h | all |

**关键路径**：T1 → T2 → T5 → T6 → T13（≈ 12h）。其余可并行，双人协作可压到 2-3 天。

---

## 8. 风险与开放问题

### 风险

| 风险 | 缓解 |
|---|---|
| 迁移在已有数据上失败（已有 channels.created_by 指向已删除用户） | 迁移时加 `JOIN users u ON u.id = c.created_by` 过滤掉悬挂引用；不阻塞迁移 |
| 权限表膨胀（每频道 Creator 3 行 × N 频道） | P1 规模可忽略（< 10k 行）；加索引 `(user_id, permission, scope)` 后查询 O(1) |
| `channel.delete` scope 为 `channel:<id>`，频道删除后权限行变"孤儿" | 定期清理 job 或删频道时同步 `DELETE FROM user_permissions WHERE scope = ?`（放在删除事务内） |
| Agent owner 被删除后 agent 无主 | 删 owner 时级联软删其所有 agent（admin UI 提示）；或阻止删除有 agent 的 member |
| 邀请码生成可被枚举 | 16 字符 base32 = 80 bit 熵，暴力不可行；加注册端点速率限制兜底 |
| 前端 `useCan` 与后端权限不一致导致 UI 显示按钮但点击 403 | 按钮 action 仍以后端 403 为准，前端只做"尽力隐藏"；不做权限前置的强校验 |

### 开放问题

1. **Agent 能否创建其他 Agent？** 当前决策隐含 "不能"（`agent.manage` 默认只给 member）。需要确认——如果 agent 可以孵化 sub-agent，`owner_id` 链怎么处理？**倾向：禁止，agent 不授 `agent.manage`。**
2. **Member 离开频道是否保留 Creator 权限？** 当前授权是 `channel:<id>` scope，与是否是 channel_members 无关。即使 Creator 退出频道，仍可删除频道。**倾向：保留，符合"创建者即管理员"直觉。**
3. **Admin 能否"代理"以 member 身份操作？** 当前 admin = *，可以直接操作。不需要 impersonation 机制。
4. **邀请码是否支持批量生成 / 带角色（直接邀请成为 admin）？** P1 只支持单个、只生成 member 邀请码。升级 admin 需要另一个 admin 通过 `PATCH /admin/users/:id` 改 role。
5. **被删除频道的消息是否可搜索？** P1 不涉及搜索；默认 `deleted_at IS NOT NULL` 的频道消息不返回给客户端，但保留在 DB 供审计。
6. **WS `channel_deleted` 是否推给已离线用户？** 通过 SSE events 表回放补齐，已在 P0 的 `CHANNEL_CHANGE_KINDS` 中覆盖。无需额外处理。
