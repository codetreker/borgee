# COL-B04: 多用户注册（邀请码）— 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 背景

Collab 目前只有 admin 预创建的用户。需要支持新用户通过邀请码自主注册，让 Collab 可以被更多人使用。

P1 已实现：
- `invite_codes` 表（code, created_by, used_by, used_at, expires_at）
- Admin 页面邀请码管理（生成/撤销）
- 用户登录（/login，JWT cookie）

## 2. 方案设计

### 2.1 注册 API

```
POST /api/v1/auth/register
Content-Type: application/json

Body:
{
  "invite_code": "abc123",
  "email": "user@example.com",
  "password": "...",
  "display_name": "Alice"
}

Success Response: 201
{
  "user": { "id": "...", "display_name": "Alice", "role": "member", "email": "..." }
}
+ Set-Cookie: collab_token=<jwt>

Error Responses:
- 400 { "error": "All fields are required" }
- 404 { "error": "Invalid or expired invite code" }
- 409 { "error": "Email already registered" }
```

### 2.2 注册流程（IMMEDIATE 事务内）

```
1. 验证所有字段非空
2. email lowercase normalize
3. 原子 check-and-claim 邀请码：
   UPDATE invite_codes SET used_by=<userId>, used_at=now()
   WHERE code=? AND used_by IS NULL AND (expires_at IS NULL OR expires_at > now)
   → 如果 affected rows = 0 → 404 邀请码无效/已用
4. 验证 email 不存在于 users 表
5. 密码 hash（bcrypt，限制最大 72 字节）
6. 创建 user（role=member, email, password_hash, display_name）
7. 赋默认权限：channel.create（scope=*）+ message.send（scope=*）
8. 加入 #general 频道（如果存在）
9. 生成 JWT
```

步骤 2-9 在一个 **IMMEDIATE 事务**内，保证原子性。步骤 3 用 UPDATE + affected rows 做原子 check-and-claim，避免并发竞态。JWT 生成也在事务内，失败则回滚。

### 2.3 注册页面

路由：`/register`（SPA 前端路由）

**UI 设计**：
- 居中卡片（和 /login 风格一致）
- 4 个字段：邀请码、邮箱、密码、显示名
- 注册按钮
- 底部链接："已有账号？登录"
- 错误提示（inline，字段下方）

**交互**：
- 表单验证（前端 + 后端）
- 提交后 loading 状态
- 成功后自动跳转到主页（/）
- 失败显示具体错误

### 2.4 路由保护

- `/register` 页面：未登录可访问，已登录跳转到 /
- `/login` 页面：同上
- 其他页面：未登录跳转到 /login

### 2.5 邀请码管理确认

P1 已实现的 admin 邀请码管理需确认：
- 生成邀请码 ✅
- 查看邀请码列表（含使用状态）✅
- 撤销邀请码 ✅
- 设置过期时间（如有）

## 3. 错误处理

| 场景 | HTTP 状态码 | 错误信息 |
|------|------------|---------|
| 字段缺失 | 400 | "All fields are required" |
| 邮箱格式无效 | 400 | "Invalid email format" |
| 密码太短 | 400 | "Password must be at least 8 characters" |
| 密码太长 | 400 | "Password must be at most 72 characters" |
| 显示名无效 | 400 | "Display name must be 1-50 characters" |
| 邀请码无效/已用/过期 | 404 | "Invalid or expired invite code" |
| 邮箱已注册 | 409 | "Email already registered" |

## 4. 安全措施

- **Rate Limiting**：注册端点 IP 级限流，10 次/分钟
- **密码长度**：8-72 字符（bcrypt 72 字节截断防护）
- **Email normalize**：lowercase 后存储和查重
- **邀请码原子 claim**：UPDATE WHERE used_by IS NULL 防并发

## 5. Task Breakdown

### T1: 注册 API（后端）

**改动文件**：`routes/auth.ts`（新建或扩展）、`queries.ts`、`db.ts`

**内容**：
1. POST /api/v1/auth/register handler
2. 事务内完成验证 + 创建用户 + 赋权限 + 标记邀请码 + 加入 #general
3. 返回 JWT cookie

**验收标准**：
- [ ] 有效邀请码 + 新 email → 注册成功 201
- [ ] 无效邀请码 → 404
- [ ] 已使用邀请码 → 404
- [ ] 重复 email → 409
- [ ] 密码 < 8 字符 → 400
- [ ] 注册后用户有 channel.create + message.send 权限
- [ ] 注册后用户是 #general 成员

### T2: 注册页面（前端）

**改动文件**：新建 `RegisterPage.tsx`、`App.tsx`（加路由）、`index.css`

**内容**：
1. /register 页面组件
2. 表单：邀请码 + email + 密码 + 显示名
3. 提交调用注册 API
4. 错误显示
5. 成功后自动登录跳转

**验收标准**：
- [ ] /register 页面可访问
- [ ] 表单填写 + 提交正常
- [ ] 错误信息正确显示
- [ ] 注册成功后自动跳转到主页
- [ ] 已登录用户访问 /register 跳转到 /

### T3: 集成 + E2E 验证

**内容**：
1. 完整流程：admin 生成邀请码 → 新用户注册 → 登录 → 发消息
2. 确认新用户在 #general 里
3. 确认新用户有正确权限

**验收标准**：
- [ ] E2E 流程完整走通
- [ ] 新用户能在 #general 发消息
- [ ] 新用户能创建频道

## 5. 不在范围

- 邮箱验证（v2）
- 自注册（无邀请码，v2）
- 密码重置（v2）
- OAuth 登录（v2）
