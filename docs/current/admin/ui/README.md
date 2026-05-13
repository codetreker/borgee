# Admin UI Architecture Sketches

These ASCII sketches are Interaction And Layout Reference for core Admin SPA pages. They help maintainers recognize the older login, dashboard, user, invite, and channel-management shapes after reading `../README.md` and `../spa.md`.

They do not define product behavior, implementation contracts, verification status, or a complete inventory of the current admin route tree. The current Admin SPA also includes route groups such as audit views, runtime metadata, heartbeat lag, archived channels, description history, and settings; those boundaries are described in `../spa.md` and `../server-rail.md`.

## Context

- Admin is a separate browser application with its own `/admin` entry, session, API client, and route tree.
- Admin authority comes from the admin rail, not from the user SPA or user cookies.
- These sketches show page layout intent only; server enforcement and privacy filtering remain backend responsibilities.

## 1. Admin 登录页

```
+──────────────────────────────────────────────────────────────────────────────+
│                                                                              │
│                            🖖 Borgee Admin                                  │
│                                                                              │
│                   ┌──────────────────────────────┐                           │
│                   │  Username                     │                           │
│                   └──────────────────────────────┘                           │
│                   ┌──────────────────────────────┐                           │
│                   │  Password            [👁]     │                           │
│                   └──────────────────────────────┘                           │
│                                                                              │
│                   ┌──────────────────────────────┐                           │
│                   │           LOG IN              │                           │
│                   └──────────────────────────────┘                           │
│                                                                              │
+──────────────────────────────────────────────────────────────────────────────+
```

- **入口**：独立于用户登录页的 admin browser rail。
- **凭据区域**：展示 admin username/password form placement only.
- **登录结果**：session handling and route protection are owned by `../spa.md` and the admin server rail.

## 2. Admin 后台主页

```
+────────────────+─────────────────────────────────────────────────────────────+
│                │                                                             │
│  🖖 Borgee     │   概览                                                      │
│  Admin         │                                                             │
│                │   ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  ┌───────────┐ │   │ 用户总数  │  │ 频道总数  │  │ 当前在线  │                 │
│  │ 📊 概览   │ │   │    12    │  │    8     │  │    3     │                 │
│  │ 👤 用户   │ │   └──────────┘  └──────────┘  └──────────┘                 │
│  │ 💬 频道   │ │                                                             │
│  │ 🎟️ 邀请码 │ │   最近注册用户                                               │
│  │ ⚙️ 设置   │ │   ┌───────────────────────────────────────────────────────┐ │
│  └───────────┘ │   │ 用户名      邮箱                    注册日期           │ │
│                │   │ ──────────────────────────────────────────────────── │ │
│                │   │ alice       alice@example.com       2026-04-20      │ │
│                │   │ bob         bob@example.com         2026-04-21      │ │
│                │   │ carol       carol@example.com       2026-04-22      │ │
│                │   └───────────────────────────────────────────────────────┘ │
│                │                                                             │
+────────────────+─────────────────────────────────────────────────────────────+
```

- **左侧导航**：shows the core admin page grouping in this older sketch.
- **概览卡片**：metadata summary placement.
- **最近注册用户**：metadata table placement.

## 3. 用户管理

```
+────────────────+─────────────────────────────────────────────────────────────+
│                │                                                             │
│  🖖 Borgee     │   用户管理                              [+ 创建用户]        │
│  Admin         │                                                             │
│                │   搜索: ┌──────────────────────────────┐                    │
│  ┌───────────┐ │         │ 搜索用户名或邮箱...            │                    │
│  │   概览    │ │         └──────────────────────────────┘                    │
│  │ ▶ 用户   │ │                                                             │
│  │   频道    │ │   ┌───────────────────────────────────────────────────────┐ │
│  │   邀请码  │ │   │ 用户名      邮箱                    状态     注册日期  │ │
│  │   设置    │ │   │ ──────────────────────────────────────────────────── │ │
│  └───────────┘ │   │ alice       alice@example.com       活跃    04-20   │ │
│                │   │ bob         bob@example.com         活跃    04-21   │ │
│                │   │ carol       carol@example.com       已禁用  04-22   │ │
│                │   └───────────────────────────────────────────────────────┘ │
│                │                                                             │
│                │   ← 1  2  3 →                                               │
│                │                                                             │
+────────────────+─────────────────────────────────────────────────────────────+
```

**创建用户弹窗：**

```
         ┌─────────────────────────────────────┐
         │  创建用户                       [✕]  │
         ├─────────────────────────────────────┤
         │                                     │
         │  用户名                              │
         │  ┌─────────────────────────────┐    │
         │  │                             │    │
         │  └─────────────────────────────┘    │
         │                                     │
         │  邮箱                               │
         │  ┌─────────────────────────────┐    │
         │  │                             │    │
         │  └─────────────────────────────┘    │
         │                                     │
         │  初始密码                            │
         │  ┌─────────────────────────────┐    │
         │  │                             │    │
         │  └─────────────────────────────┘    │
         │                                     │
         │  [取消]                [创建]        │
         └─────────────────────────────────────┘
```

- **用户列表**：metadata table placement for account rows.
- **创建用户区域**：action placement only; server rail owns authorization and accepted fields.
- **用户详情入口**：row-to-detail navigation shape.
- **分页**：table navigation placement.

## 4. User Detail

```
+────────────────+─────────────────────────────────────────────────────────────+
│                │                                                             │
│  🖖 Borgee     │   ← 返回用户列表                                            │
│  Admin         │                                                             │
│                │   ┌─ 用户信息 ──────────────────────────────────────────┐   │
│  ┌───────────┐ │   │                                                     │   │
│  │   概览    │ │   │  用户名:    alice                                   │   │
│  │ ▶ 用户   │ │   │  邮箱:      alice@example.com                      │   │
│  │   频道    │ │   │  状态:      🟢 活跃                                │   │
│  │   邀请码  │ │   │  注册日期:   2026-04-20                             │   │
│  │   设置    │ │   │  注册方式:   邀请码                                 │   │
│  └───────────┘ │   │                                                     │   │
│                │   │  操作: [禁用用户] [删除用户]                          │   │
│                │   └─────────────────────────────────────────────────────┘   │
│                │                                                             │
│                │   ┌─ 该用户的 Agent（2）────────────────── metadata ───┐   │
│                │   │                                                     │   │
│                │   │  Agent 名称     Agent ID        状态     创建日期   │   │
│                │   │  ─────────────────────────────────────────────────  │   │
│                │   │  🤖 bot-1       agent-bot1-001   🟢 在线  04-21    │   │
│                │   │  🤖 bot-2       agent-bot2-002   ⚫ 离线  04-22    │   │
│                │   │                                                     │   │
│                │   │  Sensitive fields are not page content here.         │   │
│                │   │  Owner workflow remains outside admin UI.            │   │
│                │   └─────────────────────────────────────────────────────┘   │
│                │                                                             │
+────────────────+─────────────────────────────────────────────────────────────+
```

- **用户信息**：account metadata placement.
- **Agent 列表**：owned-agent metadata placement.
- **敏感字段**：server rail owns returned fields and sanitization; this sketch only shows that secrets are not page content here.
- **操作按钮**：action placement only; server rail owns authorization, side effects, and audit behavior.
- **Agent ownership**：user-owned agent management remains part of the user SPA architecture.

## 5. 邀请码管理

```
+────────────────+─────────────────────────────────────────────────────────────+
│                │                                                             │
│  🖖 Borgee     │   邀请码管理                          [+ 生成邀请码]         │
│  Admin         │                                                             │
│                │   ┌───────────────────────────────────────────────────────┐ │
│  ┌───────────┐ │   │ 邀请码         创建者     使用者     状态     创建日期 │ │
│  │   概览    │ │   │ ──────────────────────────────────────────────────── │ │
│  │   用户    │ │   │ INV-A83K       alice      bob       已使用   04-20  │ │
│  │   频道    │ │   │ INV-Z92M       alice      —         可用     04-21  │ │
│  │ ▶ 邀请码 │ │   │ INV-Q7WP       admin      carol     已使用   04-22  │ │
│  │   设置    │ │   │ INV-M3NX       admin      —         已作废   04-23  │ │
│  └───────────┘ │   └───────────────────────────────────────────────────────┘ │
│                │                                                             │
│                │   ← 1  2 →                                                  │
│                │                                                             │
+────────────────+─────────────────────────────────────────────────────────────+
```

- **邀请码列表**：metadata table placement.
- **生成入口**：action placement only.
- **状态列**：status metadata placement.
- **使用者列**：relationship metadata placement.

## 6. 频道管理

```
+────────────────+─────────────────────────────────────────────────────────────+
│                │                                                             │
│  🖖 Borgee     │   频道管理                                                   │
│  Admin         │                                                             │
│                │   ┌───────────────────────────────────────────────────────┐ │
│  ┌───────────┐ │   │ 频道名        类型     成员数    状态      创建日期   │ │
│  │   概览    │ │   │ ──────────────────────────────────────────────────── │ │
│  │   用户    │ │   │ #general      公开      12       活跃     04-01    │ │
│  │ ▶ 频道   │ │   │ #random       公开       8       活跃     04-01    │ │
│  │   邀请码  │ │   │ #dev          公开       5       活跃     04-05    │ │
│  │   设置    │ │   │ #old-project  公开       3       已归档   04-10    │ │
│  └───────────┘ │   └───────────────────────────────────────────────────────┘ │
│                │                                                             │
│                │   操作区：                                                   │
│                │   - 查看频道元数据                                            │
│                │   - 管理操作入口                                              │
│                │   - 历史/归档入口                                             │
│                │                                                             │
+────────────────+─────────────────────────────────────────────────────────────+
```

- **频道列表**：channel metadata table placement.
- **频道操作**：action placement only; server rail owns authorization, returned fields, and side effects.
- **与用户 rail 的区别**：admin pages consume admin rail metadata; user channel visibility remains a user rail concern.

## Architecture Notes

- Admin is a separate browser rail with an independent `/admin` entry.
- User-owned agent management belongs to the user SPA; admin user detail can show owner-agent metadata without becoming the owner workflow.
- Sensitive user or agent secrets should not become admin page content unless the server rail intentionally exposes a safe metadata contract.
- Admin identity is separate from the user table and user roles.
- These sketches cover core pages only. Use `../spa.md` for the current route groups and `../server-rail.md` for admin API/server-only surfaces.
