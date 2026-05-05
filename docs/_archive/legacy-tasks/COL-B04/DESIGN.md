# COL-B04: Multi-User Registration (Invite Code)

## Overview

Admin 生成邀请码，新用户通过邀请码注册账号，注册后自动登录并加入 #general 频道。

## API Design

### POST /api/v1/auth/register

**Request:**
```json
{
  "invite_code": "string",
  "email": "string",
  "password": "string",
  "display_name": "string"
}
```

**Success Response (201):**
```json
{
  "user": { "id": "string", "email": "string", "display_name": "string", "role": "member" },
  "token": "string"
}
```

**Error Responses:**

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `INVALID_INPUT` | 缺少必填字段 / 格式不合法 |
| 400 | `INVALID_INVITE_CODE` | 邀请码不存在 |
| 400 | `INVITE_CODE_USED` | 邀请码已被使用 |
| 409 | `EMAIL_ALREADY_EXISTS` | Email 已注册 |

### Processing Logic

1. 验证请求体（email 格式、password 长度 >= 8、display_name 非空）
2. 查询 invite_codes 表，校验邀请码存在且未使用
3. 查询 users 表，校验 email 唯一
4. 创建用户（role = `member`，hash password with bcrypt）
5. 标记邀请码为已使用（关联 user_id + used_at）
6. 将用户加入 #general channel（insert channel_members）
7. 签发 JWT，设置 httpOnly cookie
8. 返回用户信息 + token

步骤 2-6 在单个 SQLite 事务中执行。

## Frontend

### /register Page

- 路由：`/register`（未登录可访问，已登录重定向到 `/`）
- 表单字段：Invite Code、Email、Password、Display Name
- 提交后调用 register API，成功则跳转 `/`
- 错误信息内联显示在表单下方
- 页面底部 "Already have an account? Login" 链接到 `/login`

### Router Changes

- 添加 `/register` 公开路由
- `/login` 页面添加 "Don't have an account? Register" 链接

## Task Breakdown

### T1: Register API Endpoint

**Scope:** 新增 `POST /api/v1/auth/register` 路由及处理逻辑

- 输入验证（Fastify schema validation）
- 邀请码校验（存在 + 未使用）
- Email 唯一性校验
- 创建用户（bcrypt hash, role=member）
- 标记邀请码已使用
- 自动加入 #general channel
- 签发 JWT cookie
- 事务包裹所有写操作

**验收标准:**
- [ ] 有效邀请码 + 合法输入 → 201，返回 user + token，cookie 已设置
- [ ] 无效邀请码 → 400 INVALID_INVITE_CODE
- [ ] 已使用邀请码 → 400 INVITE_CODE_USED
- [ ] 重复 email → 409 EMAIL_ALREADY_EXISTS
- [ ] 密码长度 < 8 → 400 INVALID_INPUT
- [ ] 注册后用户出现在 #general 的 channel_members 中
- [ ] 邀请码 used_at 和 used_by 字段已更新

### T2: Register Page Frontend

**Scope:** 新增 `/register` 页面，表单提交调用注册 API

- 表单：invite_code, email, password, display_name
- 调用 `POST /api/v1/auth/register`
- 成功后跳转到 `/`（主页）
- 错误状态内联显示
- 登录/注册页面互相链接

**验收标准:**
- [ ] `/register` 页面可访问，表单渲染正确
- [ ] 填写有效信息提交 → 自动登录并跳转到主页
- [ ] 已登录用户访问 `/register` → 重定向到 `/`
- [ ] 各错误场景显示对应中文/英文错误提示
- [ ] `/login` 有链接跳转到 `/register`，反之亦然

### T3: E2E Test

**Scope:** 端到端验证注册流程

**验收标准:**
- [ ] 测试完整注册流程：生成邀请码 → 注册 → 自动登录 → 可见 #general
- [ ] 测试错误场景：无效邀请码、已用邀请码、重复 email
- [ ] 注册用户可以发送消息（验证默认权限）

## Out of Scope

- 自注册（无邀请码注册）
- 邮件验证
- 密码重置
- OAuth / 第三方登录
