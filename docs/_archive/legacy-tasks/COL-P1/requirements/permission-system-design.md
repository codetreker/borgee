# 权限系统 — 技术设计文档

日期：2026-04-20 | 状态：Draft | 作者：飞马（架构师）

---

## 背景与问题

Collab 当前只有 admin/member/agent 三种角色标签，权限逻辑散落在各路由中（硬编码 `if role === 'admin'`）。没有统一权限检查、没有可配置权限，每加一个功能都要单独讨论"谁能做"。

建军要求引入权限系统，核心定义：
- **admin** = `*`（拥有一切权限，包括管理权限本身和未来新增的）
- **member/agent** = 可配置权限，由 admin 赋予

详见 [权限系统 PRD](../requirements/permission-system.md)。

## 目标

1. 统一权限检查中间件，替代分散的 `if role === 'admin'` 判断
2. 支持给 member/agent 赋予/收回权限
3. 频道删除作为权限系统的第一个应用场景
4. admin = `*`，不存储权限列表，代码层面直接通过

## 方案设计

### 权限模型

```
角色（role）：admin | member | agent
  ↓
admin → 权限 = *（代码判断，不查表）
member/agent → 查 user_permissions 表
```

**权限检查伪码**：
```typescript
function checkPermission(userId: string, permission: string, resourceId?: string): boolean {
  const user = getUserById(userId);
  if (user.role === 'admin') return true;  // * 通配
  
  // 检查全局权限
  if (hasPermission(userId, permission)) return true;
  
  // 检查资源级权限（如特定频道）
  if (resourceId && hasPermission(userId, permission, resourceId)) return true;
  
  // 检查频道创建者权限（创建者自动拥有该频道的管理权限）
  if (resourceId && isChannelCreator(userId, resourceId)) {
    return CREATOR_PERMISSIONS.includes(permission);
  }
  
  return false;
}
```

### 数据模型

#### 新增表：`user_permissions`

```sql
CREATE TABLE user_permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     TEXT NOT NULL REFERENCES users(id),
  permission  TEXT NOT NULL,           -- 'channel.create', 'channel.delete', etc.
  scope       TEXT DEFAULT '*',        -- '*' = 全局, 'channel:<id>' = 特定频道
  granted_by  TEXT NOT NULL,           -- 谁授予的
  granted_at  INTEGER NOT NULL,        -- Unix timestamp (ms)
  UNIQUE(user_id, permission, scope)
);

CREATE INDEX idx_user_permissions_user ON user_permissions(user_id);
```

#### 修改表：`channels`

```sql
ALTER TABLE channels ADD COLUMN deleted_at INTEGER DEFAULT NULL;
```

所有查询加 `WHERE deleted_at IS NULL` 过滤。

### 权限点定义

| 权限 | 说明 | member 默认 | agent 默认 |
|------|------|:---:|:---:|
| `channel.create` | 创建频道 | ✅ | ❌ |
| `channel.delete` | 删除频道 | ❌ | ❌ |
| `channel.manage_members` | 管理频道成员 | ❌ | ❌ |
| `channel.manage_visibility` | 修改频道可见性 | ❌ | ❌ |
| `message.send` | 发送消息 | ✅ | ✅ |
| `message.delete` | 删除消息 | ❌ | ❌ |
| `user.manage` | 管理用户 | ❌ | ❌ |
| `permission.manage` | 管理权限 | ❌ | ❌ |

**频道创建者自动权限**：创建者对自己创建的频道自动拥有 `channel.delete`、`channel.manage_members`、`channel.manage_visibility`，不需要单独赋予。

### 权限中间件

```typescript
// packages/server/src/middleware/permissions.ts

import type { FastifyRequest, FastifyReply } from 'fastify';

const CREATOR_PERMISSIONS = [
  'channel.delete',
  'channel.manage_members', 
  'channel.manage_visibility',
];

export function requirePermission(permission: string, getResourceId?: (req: FastifyRequest) => string | undefined) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    const userId = request.currentUser?.id;
    if (!userId) return reply.status(401).send({ error: 'Authentication required' });
    
    const user = Q.getUserById(db, userId);
    if (!user) return reply.status(401).send({ error: 'User not found' });
    
    // admin = * = 通过
    if (user.role === 'admin') return;
    
    const resourceId = getResourceId?.(request);
    
    // 检查全局权限
    if (Q.hasPermission(db, userId, permission)) return;
    
    // 检查资源级权限
    if (resourceId && Q.hasPermission(db, userId, permission, resourceId)) return;
    
    // 检查创建者权限
    if (resourceId && CREATOR_PERMISSIONS.includes(permission)) {
      const channel = Q.getChannel(db, resourceId);
      if (channel?.created_by === userId) return;
    }
    
    return reply.status(403).send({ error: 'Permission denied' });
  };
}
```

**使用方式**：
```typescript
// 频道删除路由
app.delete('/api/v1/channels/:channelId', {
  preHandler: requirePermission('channel.delete', req => req.params.channelId),
}, async (request, reply) => {
  // 到这里权限已通过
  // ...
});
```

### 频道删除实现

#### API：`DELETE /api/v1/channels/:channelId`

```typescript
app.delete('/api/v1/channels/:channelId', {
  preHandler: requirePermission('channel.delete', req => req.params.channelId),
}, async (request, reply) => {
  const { channelId } = request.params;
  const db = getDb();
  const channel = Q.getChannel(db, channelId);
  
  if (!channel) return reply.status(404).send({ error: 'Channel not found' });
  if (channel.name === 'general') return reply.status(403).send({ error: 'Cannot delete #general' });
  if (channel.type === 'dm') return reply.status(403).send({ error: 'Cannot delete DM channels' });
  
  // 软删除
  db.prepare('UPDATE channels SET deleted_at = ? WHERE id = ?').run(Date.now(), channelId);
  
  // 插入事件（SSE + 长轮询）
  Q.insertEvent(db, 'channel_deleted', channelId, { 
    channel_id: channelId, 
    channel_name: channel.name,
    deleted_by: request.currentUser.id 
  });
  
  // WS 广播给该频道所有成员
  broadcastToChannel(channelId, {
    type: 'channel_deleted',
    channel_id: channelId,
    channel_name: channel.name,
  });
  
  return { ok: true };
});
```

#### 前端

- 频道设置（或右键菜单）加"删除频道"按钮
- 只对有权限的用户显示（admin 或创建者）
- 点击后弹确认框：`确定要删除 #频道名 吗？此操作不可撤销。`
- 确认后调 `DELETE /api/v1/channels/:channelId`
- 收到 `channel_deleted` WS 事件后：从侧边栏移除、如果当前在该频道则自动跳到 #general

### Admin 权限管理 API

```
GET  /api/v1/admin/users/:userId/permissions
→ { permissions: [{ permission, scope, granted_by, granted_at }] }

PUT  /api/v1/admin/users/:userId/permissions
Body: { permissions: ["channel.create", "channel.delete", ...] }
→ { ok: true }
```

PUT 是全量覆盖（简单直接），admin 发什么就存什么。v1 不做增量 add/remove。

### 数据库查询

```typescript
// queries.ts 新增

export function hasPermission(db, userId: string, permission: string, scope: string = '*'): boolean {
  const row = db.prepare(
    'SELECT 1 FROM user_permissions WHERE user_id = ? AND permission = ? AND (scope = ? OR scope = ?) LIMIT 1'
  ).get(userId, permission, scope, '*');
  return !!row;
}

export function getUserPermissions(db, userId: string) {
  return db.prepare(
    'SELECT permission, scope, granted_by, granted_at FROM user_permissions WHERE user_id = ? ORDER BY permission'
  ).all(userId);
}

export function setUserPermissions(db, userId: string, permissions: string[], grantedBy: string) {
  const now = Date.now();
  const txn = db.transaction(() => {
    db.prepare('DELETE FROM user_permissions WHERE user_id = ? AND scope = ?').run(userId, '*');
    const stmt = db.prepare(
      'INSERT INTO user_permissions (user_id, permission, scope, granted_by, granted_at) VALUES (?, ?, ?, ?, ?)'
    );
    for (const perm of permissions) {
      stmt.run(userId, perm, '*', grantedBy, now);
    }
  });
  txn();
}
```

### 迁移

现有数据迁移：
1. `ALTER TABLE channels ADD COLUMN deleted_at INTEGER DEFAULT NULL`
2. `CREATE TABLE user_permissions (...)`
3. 给所有现有 member 插入默认权限（`channel.create`, `message.send`）
4. 给所有现有 agent 插入默认权限（`message.send`）
5. 现有的分散权限检查逐步替换为 `requirePermission` 中间件

### 错误处理

| 场景 | 处理 |
|------|------|
| 无权操作 | 403 Forbidden |
| 访问不存在或已删除的频道 | 404 Not Found |
| 非成员访问私有频道 | 404 Not Found（不暴露存在） |
| 删除 #general | 403 + 具体错误信息 |
| 删除 DM | 403 + 具体错误信息 |

## 备选方案

### 方案 B：RBAC（基于角色的访问控制）

- 定义多个角色（admin, moderator, member, guest, agent）
- 每个角色有固定权限集
- **不选**：5 人团队 overkill，建军的模型更简洁（角色只管"谁能改权限"）

### 方案 C：保持现状（硬编码 role 检查）

- **不选**：每加功能都要讨论权限，不可维护

## 测试策略

### 单元测试

| 模块 | 测试重点 |
|------|----------|
| `permissions.ts` 中间件 | admin 通配、permission 查询、creator 权限、403/404 |
| `hasPermission()` | 全局权限、资源级权限、scope 匹配 |
| 频道删除路由 | 软删除、#general 保护、DM 保护、WS 广播 |
| 权限管理 API | 权限 CRUD、非 admin 403 |

### E2E 验收

1. admin 可以删除任何频道（除 #general）
2. 创建者可以删除自己创建的频道
3. 普通 member 不能删除频道（按钮不显示）
4. agent 不能删除频道
5. 删除后在线成员自动跳回 #general
6. admin 可以给 member 赋予 channel.delete 权限后 member 可以删频道

## Task Breakdown

| ID | 任务 | 依赖 | 估时 | 说明 |
|----|------|------|------|------|
| PERM-T01 | 数据库迁移 | — | 1h | user_permissions 表 + channels.deleted_at + 默认权限数据 |
| PERM-T02 | 权限查询函数 | T01 | 1.5h | hasPermission、getUserPermissions、setUserPermissions |
| PERM-T03 | 权限中间件 | T02 | 2h | requirePermission preHandler + creator 权限逻辑 |
| PERM-T04 | 频道删除 API | T03 | 2h | DELETE /api/v1/channels/:channelId + 软删除 + 事件 + WS 广播 |
| PERM-T05 | 现有路由迁移 | T03 | 2h | 现有权限检查逐步替换为 requirePermission |
| PERM-T06 | Admin 权限管理 API | T02 | 1.5h | GET/PUT 用户权限 |
| PERM-T07 | 前端：频道删除 UI | T04 | 2h | 删除按钮 + 确认弹窗 + channel_deleted 事件处理 |
| PERM-T08 | 前端：权限感知 | T06 | 1.5h | 按钮显示/隐藏根据用户权限 |
| PERM-T09 | 测试 | T01-T08 | 3h | 单测 + 集成测试 |
| PERM-T10 | 部署 staging + E2E | T09 | 1.5h | staging 验收 → prod |

**总估时：~18h**

**关键路径**：T01 → T02 → T03 → T04/T05（并行）→ T07 → T09 → T10

## 风险与开放问题

| 风险 | 影响 | 缓解 |
|------|------|------|
| 软删除查询遗漏 | 已删除频道仍可见 | 所有频道查询统一加 `AND deleted_at IS NULL` |
| 权限缓存一致性 | 权限变更后旧缓存生效 | v1 不缓存，每次查 DB（5 人规模无性能问题） |

**开放问题**：
1. ~~软删除 vs 硬删除~~ → **已确认：软删除**
2. ~~admin 权限模型~~ → **已确认：admin = `*`**
3. 前端权限管理 UI 做多深？v1 建议只在 admin 页面加权限编辑，不做独立权限管理页面

## 参考资料

- [权限系统 PRD](../requirements/permission-system.md)
- [Collab v1 技术设计](./technical-design-v1.md)
