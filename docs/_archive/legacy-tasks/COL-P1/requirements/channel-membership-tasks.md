# 频道成员管理 — Task Breakdown

日期：2026-04-20 | 基于：`channel-membership-design.md` + `channel-membership.md`（PRD）
总估时：**~13.5h** | 分 3 个 PR 交付

---

## PR1：后端（T1–T6） ~5h

### T1 — 数据模型 + 类型 + CHECK 约束（30min）

**目标**：channels 表新增 visibility 列，类型定义同步更新。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/db.ts` | migration：`ALTER TABLE channels ADD COLUMN visibility TEXT DEFAULT 'public'`；加 CHECK 约束 `CHECK(visibility IN ('public','private'))` | +15 |
| `packages/server/src/types.ts` | Channel 接口加 `visibility?: 'public' \| 'private'` | +2 |
| `packages/client/src/types.ts` | 同上 | +2 |

**注意**：SQLite 的 ALTER TABLE 不支持直接加 CHECK，需要在 CREATE TABLE 时加或通过 trigger 模拟。实际方案：对于已存在的表，用 migration 重建表（`CREATE TABLE ... AS SELECT`）或在应用层校验 + 新表创建时包含 CHECK。如果当前 schema 是 `initSchema` 里的 `CREATE TABLE IF NOT EXISTS`，直接在建表语句加 `CHECK(visibility IN ('public','private'))` 即可（新库生效），老库走 ALTER TABLE 不带 CHECK。

**验证方式**：
- 启动 server，`PRAGMA table_info(channels)` 确认 visibility 列存在
- 已有频道 visibility 值为 `'public'`
- TypeScript 编译通过

---

### T2 — 查询层：canAccessChannel + addUserToPublicChannels + Admin 列表（1.5h）

**目标**：核心查询函数，为路由层提供数据访问。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/queries.ts` | 新增 `canAccessChannel(db, channelId, userId)` — 合并为单条 SQL（JOIN channels + channel_members + users，一次查询判断）；新增 `addUserToPublicChannels(db, userId)` 替换 `addUserToDefaultChannel`；新增 `listAllChannelsForAdmin(db, userId)`；修改 `listChannels` 添加 `AND (visibility = 'public' OR visibility IS NULL)` 过滤；修改 `createChannel` 接受 visibility 参数 | +80 |

**canAccessChannel 合并 SQL 示例**：
```sql
SELECT
  c.visibility,
  EXISTS(SELECT 1 FROM channel_members WHERE channel_id = ? AND user_id = ?) AS is_member,
  (SELECT role FROM users WHERE id = ?) AS user_role
FROM channels c
WHERE c.id = ?
```
一次查询拿到所有信息，避免多次 round-trip。

**验证方式**：
- 单元测试：public 频道任意用户 → true；private 频道非成员非 admin → false；private 频道 admin → true；private 频道成员 → true
- `addUserToPublicChannels` 只加入 public 频道，不加入 private
- `listChannels`（未登录）不返回 private 频道
- `listAllChannelsForAdmin` 返回所有频道且带 `is_member` 标记

---

### T3 — 频道路由：创建/更新/详情 + join/leave 端点（1.5h）

**目标**：API 层实现 visibility 控制和自助 join/leave。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/routes/channels.ts` | 修改 POST `/channels`：接受 visibility、member_ids；**创建频道用 transaction 包裹**（创建 + 批量加成员原子化）；公开频道创建时 `addUserToPublicChannels` 风格批量加所有用户；**去掉"至少选一人"限制**（私有频道允许只有创建者一人） | +60 |
| 同上 | 修改 PUT `/channels/:channelId`：支持 visibility 切换；#general 禁止设 private（403）；**私有→公开切换时给新增成员推 `channel_added` 事件**（遍历新加入的用户，逐一 broadcastToUser）；广播 `visibility_changed` 事件；**私有→公开切换使用事务 + `INSERT OR IGNORE` 批量加全员**，避免并发 race condition | +50 |
| 同上 | 修改 GET `/channels/:channelId`：私有频道非成员非 admin → 404 | +15 |
| 同上 | 修改 GET `/channels`：按 admin/已登录/未登录分流查询 | +20 |
| 同上 | 新增 POST `/channels/:channelId/join`：仅公开频道（DM 频道返回 403）；生成 `user_joined` 事件；新增 POST `/channels/:channelId/leave`：#general 不可离开（DM 频道返回 403）；生成 `user_left` 事件 | +50 |
| 同上 | 修改 GET `/channels/:channelId/members`：私有频道非成员非 admin → 404 | +10 |
| 同上 | 修改 POST `/channels/:channelId/members`（添加成员）：成功后向被添加用户 broadcastToUser `channel_added` 事件 | +10 |
| 同上 | 修改 DELETE `/channels/:channelId/members/:userId`（移除成员）：成功后向被移除用户 broadcastToUser `channel_removed` 事件 | +10 |

**关键处理项**：
1. **私有→公开切换时给新增成员推 `channel_added` 事件**：切换时先拿到原有 member set，在事务中 `INSERT OR IGNORE` 全员后，diff 出新增用户，对每个新增用户调用 `broadcastToUser({ type: 'channel_added', channel })`。事务 + `INSERT OR IGNORE` 保证并发切换时幂等、不会因 unique 冲突报错
2. **创建频道用 transaction**：`db.transaction(() => { createChannel(); addMembers(); })()`
3. **join/leave 生成 user_joined/user_left 事件**：join 成功后写入 `{ type: 'user_joined', channelId, userId }` 事件；leave 成功后写入 `{ type: 'user_left', channelId, userId }` 事件
4. **DM 频道禁止 join/leave**：join/leave 端点首先检查 `channel.type === 'dm'`，是则返回 403（DM 成员关系由系统管理，不可手动变更）

**验证方式**：
- curl 创建 private 频道 → 只有创建者+指定用户在 channel_members
- curl 创建 public 频道 → 所有用户在 channel_members
- curl PUT visibility 从 private→public → 所有用户被加入
- curl GET 私有频道（非成员）→ 404
- curl POST join 公开频道 → 200；join 私有频道 → 403
- curl POST join/leave DM 频道 → 403
- curl POST leave #general → 403
- join/leave 后 WS 收到 user_joined/user_left 事件

---

### T4 — 消息路由：私有频道访问控制（45min）

**目标**：消息读取、发送和搜索增加私有频道权限检查。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/routes/messages.ts` | GET `/channels/:channelId/messages`：调用 `canAccessChannel`，不通过 → 404；**POST `/channels/:channelId/messages`：调用 `isChannelMember` 检查，非成员（包括 Admin 非成员）→ 403**；GET `/channels/:channelId/messages/search`：调用 `canAccessChannel`，不通过 → 404 | +30 |

**关键处理项**：
1. **POST 发消息的访问控制独立于 GET 读取**：GET 允许 Admin 非成员查看（canAccessChannel），但 POST 发消息必须是频道成员才可（isChannelMember），Admin 非成员也不能发。这防止了绕过前端禁用输入框直接 POST 发消息到私有频道的安全漏洞

**验证方式**：
- 非成员非 admin GET 私有频道消息 → 404
- admin 非成员 GET → 200
- 成员 GET → 200
- **非成员 POST 消息到私有频道 → 403**
- **Admin 非成员 POST 消息到私有频道 → 403**
- 成员 POST 消息到私有频道 → 201

---

### T5 — WebSocket：broadcastToUser + Admin 订阅 + 新事件类型（45min）

**目标**：WS 层支持定向推送和新事件。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/ws.ts` | 新增 `broadcastToUser(userId, payload)` 函数；修改 `subscribe` handler：admin 可订阅私有频道（即使非成员）；新增 EventKind `'visibility_changed' \| 'channel_added' \| 'channel_removed' \| 'user_joined' \| 'user_left'`；**广播 `user_joined`/`user_left` 事件到频道订阅者** | +45 |
| `packages/server/src/types.ts` | EventKind 类型扩展（如果有独立定义） | +3 |

**验证方式**：
- WS 连接后 subscribe 私有频道（非成员非 admin）→ error
- WS 连接后 subscribe 私有频道（admin）→ 成功
- 添加成员后被添加用户 WS 收到 `channel_added`
- 移除成员后被移除用户 WS 收到 `channel_removed`
- join/leave 时频道订阅者收到 `user_joined`/`user_left` 事件

---

### T6 — Seed + Admin 路由：替换 addUserToDefaultChannel（20min）

**目标**：新用户创建流程使用新函数。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/seed.ts` | 调用 `addUserToPublicChannels` 替换 `addUserToDefaultChannel` | +3, -3 |
| `packages/server/src/routes/admin.ts` | 同上 | +3, -3 |

**验证方式**：
- 创建新用户 → 自动加入所有 public 频道
- 创建新用户 → 不加入任何 private 频道
- seed 数据正常生成

---

## PR2：前端（T7–T11） ~3.5h

### T7 — 前端类型 + API 客户端（30min）

**目标**：前端类型和 API 方法与后端对齐。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/types.ts` | Channel 接口加 `visibility?: 'public' \| 'private'`（T1 已改，此处确认） | +1 |
| `packages/client/src/lib/api.ts` | 修改 `createChannel` 加 visibility + member_ids 参数；新增 `updateChannelVisibility(channelId, visibility)`；新增 `joinChannel(channelId)`；新增 `leaveChannel(channelId)` | +30 |

**验证方式**：
- TypeScript 编译通过
- 各 API 方法调用后端返回正确

---

### T8 — 创建频道 UI：visibility 选择（45min）

**目标**：创建频道表单支持选择公开/私有。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/Sidebar.tsx` | 创建频道表单新增 visibility radio toggle（公开/私有）；选 private 时显示成员选择器（复用 `UserPicker`）；**去掉"至少选一人"限制**（private 频道允许不选额外成员，只有创建者自己）；调用 `createChannel` 时传 visibility 和 member_ids | +50 |

**验证方式**：
- 打开创建频道弹窗 → 看到 公开/私有 选项
- 选 private → 显示成员选择器（可不选任何人）
- 创建后频道正确出现在侧边栏

---

### T9 — 侧边栏 + 频道头部：锁图标 + join/leave UI（30min）

**目标**：私有频道视觉区分。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/Sidebar.tsx` | 频道列表项：`visibility === 'private'` 时显示 🔒 替代 `#`；**公开频道未加入时显示「加入」按钮**，调用 `joinChannel` API | +15 |
| `packages/client/src/components/ChannelView.tsx` | 频道标题旁：私有频道显示 🔒 图标；**公开频道已加入时在频道头部显示「离开频道」按钮**（#general 除外），调用 `leaveChannel` API | +20 |

**验证方式**：
- 侧边栏私有频道显示 🔒，公开频道显示 #
- 频道标题区域私有频道有锁图标
- 公开频道未加入时侧边栏显示「加入」按钮，点击后加入频道
- 公开频道已加入时频道头部显示「离开频道」按钮（#general 除外），点击后离开频道

---

### T10 — 成员管理弹窗：可见性切换 + Admin 非成员提示（1h）

**目标**：频道设置中支持可见性切换，Admin 非成员场景正确处理。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/components/ChannelMembersModal.tsx` | 弹窗顶部新增可见性切换区域；创建者/Admin 可操作，其他人只读；#general 禁用切换按钮；切换时弹确认对话框（公开→私有 / 私有→公开 提示文案不同）；调用 `updateChannelVisibility` API | +60 |
| `packages/client/src/components/ChannelView.tsx` | **Admin 非成员查看私有频道时：禁用输入框 + 显示提示**（如 "你不是此频道成员，无法发送消息。请先将自己添加为成员。"）；判断条件：`channel.visibility === 'private' && user.role === 'admin' && !isMember` | +25 |
| `packages/client/src/components/MessageInput.tsx` | 支持 `disabled` prop，禁用时显示提示文案 | +15 |

**关键处理项**：
3. **Admin 非成员查看私有频道时禁用输入框 + 提示**：ChannelView 检测当前用户是否为非成员 admin，是则传 `disabled` 和提示文案给 MessageInput

**验证方式**：
- 创建者/Admin 看到可见性切换按钮
- 普通成员看不到切换按钮
- #general 切换按钮禁用
- 切换时有确认弹窗
- Admin 以非成员身份查看私有频道 → 输入框禁用 + 提示文案

---

### T11 — WebSocket 事件 + AppContext reducer（45min）

**目标**：前端实时响应成员变更事件。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/client/src/hooks/useWebSocket.ts` | 处理 `channel_added`：将频道加入 state；处理 `channel_removed`：从 state 移除频道 + **如果当前正在该频道则跳转到 #general**；处理 `visibility_changed`：更新频道 visibility 属性；**处理 `user_joined`：更新成员列表（添加用户）+ 更新在线状态；处理 `user_left`：更新成员列表（移除用户）+ 更新在线状态** | +50 |
| `packages/client/src/context/AppContext.tsx` | 新增 reducer action：`REMOVE_CHANNEL`（从列表删除 + 判断是否需要切换当前频道）；`UPDATE_CHANNEL`（更新频道部分属性）；可能需要 `ADD_CHANNEL`（添加新频道到列表）；**`UPDATE_CHANNEL_MEMBERS`（处理 user_joined/user_left 时更新成员列表）** | +40 |

**关键处理项**：
2. **被移除频道时前端跳转到 #general**：`channel_removed` 事件处理中，检查 `state.currentChannelId === removedChannelId`，是则 dispatch `SET_CURRENT_CHANNEL` 到 #general

**验证方式**：
- 用户 A 将用户 B 添加到私有频道 → B 的侧边栏实时出现该频道
- 用户 A 移除用户 B → B 的侧边栏实时消失该频道
- B 正在被移除的频道中 → 自动跳转到 #general
- 频道可见性切换 → 侧边栏图标实时更新
- 用户 join 公开频道 → 成员列表实时更新，显示 user_joined
- 用户 leave 频道 → 成员列表实时更新，显示 user_left

---

## PR3：测试 + E2E（T12–T13） ~3h

### T12 — 后端单元测试（2h）

**目标**：覆盖所有后端逻辑的自动化测试。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/server/src/__tests__/channel-membership.test.ts`（新建） | 测试用例覆盖以下场景 | +300 |

**测试用例**：
1. Migration：visibility 列存在，默认 'public'
2. 创建 public 频道 → 所有用户自动加入
3. 创建 private 频道 → 只有创建者 + 指定成员加入
4. 创建 private 频道不指定额外成员 → 只有创建者（无"至少选一人"限制）
5. 不传 visibility → 默认 public
6. visibility 值非法 → 400
7. public → private → 成员不变
8. private → public → 所有用户加入 + 新增成员收到 channel_added 事件
9. #general 设为 private → 403
10. 非成员访问私有频道详情 → 404
11. 非成员读取私有频道消息 → 404
12. Admin 非成员可访问私有频道详情和消息
13. Admin 非成员不能发送消息（或需先加入）
14. 添加成员到私有频道 → 被添加者可访问
15. 移除成员 → 被移除者不可访问
16. 非创建者非 Admin 添加/移除成员 → 403
17. 新用户自动加入所有 public 频道，不加入 private
18. 用户自行加入 public 频道 → 200
19. 用户自行加入 private 频道 → 403
20. 用户离开 #general → 403
21. 用户离开 private 频道 → 200
22. `canAccessChannel` 合并 SQL 结果正确
23. `listChannels`（未登录）不返回 private 频道
24. `listAllChannelsForAdmin` 返回所有频道且 `is_member` 正确
25. join/leave DM 频道 → 403
26. join 公开频道后生成 user_joined 事件
27. leave 频道后生成 user_left 事件
28. Poll 端点过滤私有频道：未登录/非成员用户的 poll 结果不包含私有频道的消息

**验证方式**：
- `npm test` 全部通过
- 覆盖率报告确认核心函数 >90%

---

### T13 — E2E 验收测试（1h）

**目标**：浏览器端到端验证完整流程。

**改动文件**：
| 文件 | 改动 | 预估行数 |
|------|------|----------|
| `packages/e2e/channel-membership.spec.ts`（新建，或手动测试清单） | E2E 场景 | +150 |

**验收场景**：
1. 创建 public 频道 → 其他用户侧边栏立即可见
2. 创建 private 频道 → 仅成员可见，🔒 图标正确显示
3. 切换 public → private → 确认弹窗 → 非成员侧边栏消失
4. 切换 private → public → 确认弹窗 → 所有用户侧边栏出现
5. 添加成员到私有频道 → 被添加者侧边栏实时出现
6. 移除成员 → 被移除者侧边栏实时消失 + **如果在该频道则跳转 #general**
7. Admin 登录 → 可看到所有频道（包括未加入的私有频道）
8. **Admin 非成员打开私有频道 → 输入框禁用 + 提示文案**
9. 新用户注册 → 自动加入所有 public，不加入 private
10. #general 不可设为 private
11. 公开频道支持自行加入/离开
12. #general 不可离开

**验证方式**：
- 所有场景手动/自动通过
- 无控制台错误
- 无网络请求异常

---

## 附：Review 必须处理项 Checklist

| # | 处理项 | 对应 Task | 处理方式 |
|---|--------|-----------|----------|
| 1 | 私有→公开切换时给新增成员推 `channel_added` 事件 | T3 | PUT visibility 路由中 diff 成员列表，对新增用户 broadcastToUser |
| 2 | 被移除频道时前端跳转到 #general | T11 | `channel_removed` handler 检查 currentChannelId，匹配则切换 |
| 3 | Admin 非成员查看私有频道时禁用输入框 + 提示 | T10 | ChannelView 检测 admin+非成员，MessageInput 支持 disabled |
| 4 | **POST messages 后端访问控制** | T4 | POST `/channels/:id/messages` 检查 isChannelMember，Admin 非成员也不能发，返回 403 |
| 5 | **user_joined/user_left 事件全链路** | T3, T5, T11 | T3 生成事件 → T5 WS 广播 → T11 前端更新成员列表和在线状态 |
| 6 | **DM 频道 join/leave 防护** | T3 | join/leave 端点检查 channel.type === 'dm'，是则 403 |
| 7 | **公开频道 join/leave 前端 UI** | T9 | 侧边栏「加入」按钮 + 频道头部「离开频道」按钮 |
| 8 | **Poll 端点测试** | T12 | 补 poll 过滤私有频道的测试用例 |
| 9 | **私有→公开并发 race condition** | T3 | 事务 + INSERT OR IGNORE 保证幂等 |

## 附：额外改进项 Checklist

| # | 改进项 | 对应 Task | 处理方式 |
|---|--------|-----------|----------|
| 1 | visibility 列加 CHECK 约束 | T1 | CREATE TABLE 语句加 `CHECK(visibility IN ('public','private'))` |
| 2 | canAccessChannel 合并为单条 SQL | T2 | 一次 JOIN 查询 channel+member+user_role |
| 3 | 创建频道用 transaction | T3 | `db.transaction(() => { ... })()` 包裹创建+加成员 |
| 4 | 去掉"至少选一人"限制 | T3, T8 | 后端不校验 member_ids 非空；前端允许不选额外成员 |

---

## 时间汇总

| PR | Tasks | 估时 |
|----|-------|------|
| PR1：后端 | T1–T6 | 5.5h |
| PR2：前端 | T7–T11 | 4.5h |
| PR3：测试 | T12–T13 | 3.5h |
| **总计** | | **13.5h** |
