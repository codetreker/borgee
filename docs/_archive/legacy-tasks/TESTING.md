# Collab 测试指南

## 环境

| 环境 | URL | 说明 |
|------|-----|------|
| Staging | `https://staging-collab.codetrek.cn` | 自动部署（pipeline），手动重启容器 |
| Prod | `https://collab.codetrek.cn` | 建军 approve 后部署 |
| Admin | `jianjun@codetrek.work` | 管理员账号 |
| 测试邮箱 | `oc-test@codetrek.work` | himalaya config `collab-test` |

## 前置条件

- Playwright: `/usr/bin/google-chrome-stable`
- Node ws: `/workspace/collab/node_modules/.pnpm/ws@8.20.0/node_modules/ws`
- API key: 从 `/api/v1/admin/users` 获取（staging 和 prod 不同）
- **每次登录用新 cookie，不复用**

## E2E 测试清单

### 基础功能

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-001 | 登录 | POST /auth/login | 200 + set-cookie |
| T-002 | 注册（邀请码） | POST /auth/register | 201 + 新用户在 #general |
| T-003 | 邀请码复用 | 二次注册同一 code | 400/404 |
| T-004 | 频道列表 | GET /channels | 200 + 含 general |
| T-005 | 发消息 | POST /channels/:id/messages | 201 |
| T-006 | DM | POST /dm/:userId + 发消息 | 200/201 |
| T-007 | @mention | 消息含 `<@user_id>` | mentions 数组正确 |
| T-008 | 在线状态 | GET /online | user_ids 包含在线用户 |

### 权限系统 (P1)

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-101 | Admin 短路 | Admin 做任何操作 | 200 |
| T-102 | Member 默认权限 | channel.create | 201 |
| T-103 | Member 无权 | 访问 /admin | 403 |
| T-104 | Agent 权限 | agent 创建频道 | 403 |
| T-105 | Agent 归属 | POST /agents | 201 + owner_id |
| T-106 | 非 owner 拉 agent | 别人拉你的 agent | 403 |
| T-107 | 频道删除幂等 | DELETE 两次 | 200 → 204 |
| T-108 | #general 保护 | DELETE #general | 403 |
| T-109 | Agent 创建（UI） | 用户在前端 Agents Tab 创建 Agent | 201 + API key 返回 |

### 频道成员管理

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-201 | 创建公开频道 | visibility=public | 201 + 自动加入 |
| T-202 | 创建私有频道 | visibility=private | 201 + 只有成员 |
| T-203 | 非成员访问私有 | GET 私有频道 | 404 |
| T-204 | 添加成员 | POST /members | 201 |
| T-205 | 移除成员 | DELETE /members/:id | 200 |
| T-206 | 可见性切换 | PUT visibility | 200 |
| T-207 | 自助 join | POST /join | 200 |
| T-208 | 自助 leave | POST /leave | 200 |
| T-209 | 公开频道预览 | 非成员看 24h 消息 | 可见但不能发 |

### 消息功能

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-301 | 编辑消息 | PUT /messages/:id | 200 + edited_at |
| T-302 | 删除消息 | DELETE /messages/:id | 204 + 软删除 |
| T-303 | 非作者编辑 | agent 编辑别人消息 | 403 |
| T-304 | 编辑已删消息 | PUT 已删消息 | 400 |
| T-305 | Reactions | PUT /messages/:id/reactions | 200 + emoji |
| T-306 | 删 Reaction | DELETE /messages/:id/reactions | 200 |
| T-307 | **文件路径链接** | 消息含 /workspace/path | ⚠️ BUG: 不可点击 |
| T-308 | **Markdown 渲染** | 发送 markdown 内容 | ⚠️ BUG: 渲染不对 |

### Workspace (B20)

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-401 | 创建目录 | POST /workspace/mkdir | 201 |
| T-402 | 上传文件 | POST /workspace/upload | 201 |
| T-403 | 列出文件 | GET /workspace | files 数组 |
| T-404 | 下载文件 | GET /workspace/files/:id | 200 |
| T-405 | 重命名 | PATCH /workspace/files/:id | 200 |
| T-406 | 移动 | POST /workspace/files/:id/move | 200 |
| T-407 | 删除 | DELETE /workspace/files/:id | 204 |

### UX — 页面加载与导航

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-501 | 首页加载 | 打开 URL | 登录页或主页，无白屏 |
| T-502 | SPA 路由 | 直接访问 /admin、/login、/register | 200 不 500 |
| T-503 | 侧边栏频道列表 | 登录后 | #general + 私有🔒 + DM 分区 |
| T-504 | 频道切换 | 点击不同频道 | 消息区切换，URL 不变 |
| T-505 | 在线用户列表 | 侧边栏底部 | 绿点 + Bot 标签 + 人数 |
| T-506 | 空频道状态 | 新建频道进入 | "还没有消息"提示 |

### UX — 消息交互

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-510 | **Enter 发送** | Enter 键 | ⚠️ BUG: 应为 Enter 发送，Ctrl+Enter 换行 |
| T-511 | 消息送达 ✓ | 发送后观察 | ⏳ → ✓ |
| T-512 | 消息时间戳 | 每条消息 | 显示时间（HH:MM AM/PM） |
| T-513 | 消息头像 | 每条消息 | 发送者头像/首字母圆圈 |
| T-514 | 消息 hover 操作 | 鼠标悬停消息 | 出现反应/回复/编辑/删除按钮 |
| T-515 | 编辑消息 UI | 点编辑 | 内联编辑框，保存/取消 |
| T-516 | 删除消息 UI | 点删除 | 确认弹窗，确认后消息变"此消息已删除" |
| T-517 | (已编辑)标记 | 编辑后 | 消息旁显示 (已编辑) |
| T-518 | 消息滚动 | 新消息到达 | 自动滚动到底部 |
| T-519 | 历史消息加载 | 向上滚动 | 加载更多历史消息 |
| T-520 | **Markdown 渲染** | 发送 `**bold** *italic* \`code\`` | ⚠️ BUG: 渲染不对 |
| T-521 | 代码块渲染 | 发送 \`\`\`代码\`\`\` | 语法高亮代码块 |
| T-522 | **文件路径链接** | 消息含 /workspace/path | ⚠️ BUG: 不可点击 |

### UX — 富文本编辑器

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-530 | Tiptap 编辑器 | ProseMirror 检测 | 替换 textarea |
| T-531 | 格式工具栏 | 工具栏按钮 | B/I/code/block/ul/ol/quote |
| T-532 | 粗体快捷键 | 输入 **text** 或 Ctrl+B | 文字变粗 |
| T-533 | 代码块快捷键 | 输入 \`\`\` + Enter | 出现代码编辑区 |
| T-534 | @mention picker | 输入 @ | 用户列表弹出 |
| T-535 | @mention 过滤 | 输入 @ji | 过滤匹配用户 |
| T-536 | @mention 插入 | 选择用户 | 插入 <@user_id> |
| T-537 | @mention 渲染 | 消息列表 | 蓝色高亮标签 |
| T-538 | Emoji Picker | 点击😊按钮 | emoji-mart 网格弹出 |
| T-539 | Emoji 插入 | 选择 emoji | 插入到编辑器 |
| T-540 | Slash Commands | 输入 / | 5 个命令列表 |
| T-541 | /help 执行 | 输入 /help Enter | 系统消息列出命令 |
| T-542 | /topic 执行 | 输入 /topic xxx | 频道 topic 更新 |

### UX — Reactions

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-550 | 加 reaction | hover 消息 → 点反应按钮 → 选 emoji | emoji badge 出现 |
| T-551 | reaction 计数 | 多人同一 emoji | 数字递增 |
| T-552 | 取消 reaction | 再次点击已加的 emoji | badge 消失 |
| T-553 | 多种 emoji | 同一消息加不同 emoji | 多个 badge 并排 |

### UX — 频道管理

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-560 | 创建频道按钮 | 侧边栏 + 号 | 弹出创建表单 |
| T-561 | 创建频道表单 | 填名字 + 选 public/private | 创建成功，侧边栏出现 |
| T-562 | 私有频道🔒图标 | 侧边栏 | 🔒代替 # |
| T-563 | 成员管理弹窗 | 点击成员按钮 | 弹窗：成员列表 + 添加/移除 |
| T-564 | 弹窗 overlay | 弹窗打开 | 半透明遮罩，不透出内容 |
| T-565 | 频道删除确认 | 点删除 | 确认弹窗 |
| T-566 | 频道 topic 显示 | 频道 header | 显示 topic 文字 |
| T-567 | 公开频道预览 | 非成员查看 | 只读 + "加入"按钮 |

### UX — DM

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-570 | 点击用户发起 DM | 在线列表点用户名 | 创建/打开 DM |
| T-571 | DM 侧边栏分区 | 私信区 | 独立于频道，有头像 |
| T-572 | DM header | 进入 DM | 显示对方名字（不是频道名） |
| T-573 | DM 未读角标 | 有新消息 | 红色数字 badge |

### UX — 主题与响应式

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-580 | 暗色主题 | 默认 | 暗色背景、白色文字 |
| T-581 | **亮色主题** | 点击🌙切换 | ⚠️ BUG: 侧边栏/workspace 仍暗色 |
| T-582 | 移动端布局 | 390x844 | hamburger + 全屏消息区 |
| T-583 | 移动端侧边栏 | 点 hamburger | 滑出侧边栏 |
| T-584 | 移动端输入框 | 底部 | 固定底部 + safe-area |
| T-585 | PWA manifest | 检测 | name="Collab" |
| T-586 | Service Worker | navigator.serviceWorker | API 可用 |

### UX — Admin 后台

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-590 | Admin 入口 | 齿轮按钮 | 仅 admin 可见 |
| T-591 | 用户管理页 | /admin/users | 用户列表 + 角色 + API key |
| T-592 | 创建用户 | 表单 | agent 隐藏邮箱密码 |
| T-593 | 邀请码管理 | /admin/invites | 生成/撤销 |
| T-594 | 非 admin 访问 | member 访问 /admin | 403/redirect |
| T-595 | 注册页 | /register | 邀请码 + 邮箱 + 密码表单 |
| T-596 | 登录页 | /login | 邮箱 + 密码 + 注册链接 |

### UX — Workspace

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-600 | Workspace tab | 频道内 | 文件树 tab |
| T-601-ws | 文件上传 UI | 拖拽或按钮 | 上传成功 |
| T-602-ws | 文件预览 | 点击文件 | 图片/文本/Markdown 预览 |
| T-603-ws | 右键菜单 | 右键文件 | 重命名/移动/删除 |
| T-604-ws | 文件夹展开 | 点击文件夹 | 展开子文件列表 |

### SSE / WS

| ID | 功能 | 验证方法 | 期望 |
|----|------|----------|------|
| T-601 | SSE 连接 | GET /stream | :connected |
| T-602 | SSE 实时推送 | 发消息后 SSE 收到 | < 1s |
| T-603 | Last-Event-ID | 断线续传 | 补发事件 |
| T-604 | Poll 兼容 | POST /poll | cursor + events |
| T-605 | WS Plugin | wss://host/ws/plugin | 连接保持 |

## 已知 Bug

| ID | 描述 | 来源 | 状态 |
|----|------|------|------|
| BUG-001 | 删除消息不限制只能删自己的 | 建军 04-22 | 战马在修 |
| BUG-002 | 文件路径没有变成可点击链接 | 建军 04-22 | 待修 |
| BUG-003 | 亮色主题下侧边栏仍暗色 | 建军 04-22 | 待修 |
| BUG-004 | Enter/Ctrl+Enter 行为反了 | 建军 04-22 | 待修 |
| BUG-005 | Markdown 渲染不对 | 建军 04-22 | 待修 |

## 清理

测试完清理创建的频道、用户、文件（staging 共享环境）。
