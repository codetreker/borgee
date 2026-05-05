## P6 Slash Commands — Task Breakdown

### Task 1: Command Registry (`CommandRegistry` class + types)
- **文件**: 新建 `packages/client/src/commands/registry.ts`
- **行数**: ~50 行
- **内容**: `CommandDefinition`, `CommandContext`, `CommandError` 接口/类 + `CommandRegistry` 类 (register/get/search/all) + singleton export
- **依赖**: 无
- **验证**: 单元测试 — register、get、search 返回正确结果

### Task 2: 5 个内置命令定义
- **文件**: 新建 `packages/client/src/commands/builtins.ts`
- **行数**: ~80 行
- **内容**: 注册 help/invite/leave/topic/dm 的 `CommandDefinition`，每个命令的 `execute` 函数
- **依赖**: Task 1 (registry)
- **Bootstrap**: Task 5 的 MessageInput 导入 builtins.ts 触发注册（或 App.tsx 顶层 import）
- **验证**: 导入后 `commandRegistry.all().length === 5`；每个命令的 execute 逻辑可单独测试

### Task 3: `useSlashCommands` hook
- **文件**: 新建 `packages/client/src/hooks/useSlashCommands.ts`
- **行数**: ~90 行
- **内容**: 输入 `/` 检测、命令过滤、selectedIndex、ArrowUp/Down/Tab/Enter/Esc 键盘导航、空状态标记
- **依赖**: Task 1 (registry.search)
- **验证**: hook 测试 — 输入 `/in` 过滤出 invite；空格后关闭 picker；Esc 关闭

### Task 4: `SlashCommandPicker` 组件
- **文件**: 新建 `packages/client/src/components/SlashCommandPicker.tsx`
- **行数**: ~70 行
- **内容**: 弹出面板 UI（参考 `MentionPicker` 样式），显示命令名 + description，高亮选中项，空状态 "没有找到命令"
- **CSS**: 在现有样式文件中添加 `.slash-command-picker` 等 ~30 行
- **依赖**: Task 1 (CommandDefinition 类型)
- **验证**: 手动 — 输入 `/` 看到 5 个命令列表；输入 `/t` 过滤出 topic；无匹配显示空状态

### Task 5: MessageInput 集成 — 触发 + picker 展示 + 无参命令即选即执行
- **文件**: 修改 `packages/client/src/components/MessageInput.tsx`
- **行数**: ~50 行改动
- **内容**:
  - 引入 `useSlashCommands` hook 和 `SlashCommandPicker`
  - `handleChange` 中增加 `/` 触发检测（与 `@` mention 互斥：`/` 优先）
  - `handleKeyDown` 中 slash picker active 时委托给 hook 的 keyboard handler（优先于 mention）
  - JSX 中 MentionPicker 前插入 `<SlashCommandPicker />`
  - **`paramType: "none"` 命令（/help、/leave）选中即执行**：Tab/Enter 选中时直接调用 `command.execute(ctx)`，不插入输入框、不需要二次提交
- **依赖**: Task 3, Task 4
- **验证**: 手动 — 输入 `/` 出现 picker，键盘导航正常，Esc 关闭；Tab/Enter 选中 `/help` 后立即执行（出现系统消息），不在输入框中残留文本

### Task 6: 命令执行 — handleSubmit 拦截
- **文件**: 修改 `packages/client/src/components/MessageInput.tsx` 的 `handleSend`
- **行数**: ~30 行改动
- **内容**:
  - 检测 `text.startsWith('/')` + 解析 commandName/args
  - `commandRegistry.get(name)` 存在则构建 `CommandContext` 并 execute
  - 成功 → 清空输入；失败 → 显示 inline error
  - 未知命令 → 作为普通消息发送（不拦截）
- **依赖**: Task 1, Task 2, Task 5
- **验证**: 输入 `/topic new topic` + Enter 执行命令不发送消息；输入 `/foo` 作为普通消息发送

### Task 7: AppContext — `INSERT_LOCAL_SYSTEM_MESSAGE` + `NAVIGATE_AFTER_LEAVE` actions
- **文件**: 修改 `packages/client/src/context/AppContext.tsx`
- **行数**: ~25 行
- **内容**:
  - 新增 `INSERT_LOCAL_SYSTEM_MESSAGE` action type + reducer case（往 channel messages 追加 `{ type: 'system', persisted: false }` 的本地消息）
  - 新增 `NAVIGATE_AFTER_LEAVE` action type + reducer case（将当前 channel 从列表中移除，导航到频道列表或第一个可用频道）
- **依赖**: 无（可与 Task 1 并行）
- **验证**: dispatch `INSERT_LOCAL_SYSTEM_MESSAGE` 后 messages list 中出现 system 消息；dispatch `NAVIGATE_AFTER_LEAVE` 后当前频道被移除且视图切换

### Task 8: `/help` 命令端到端
- **文件**: `builtins.ts`（已在 Task 2 定义，此 task 确保端到端可用）
- **行数**: 0（逻辑已在 Task 2，此为集成验证）
- **依赖**: Task 6, Task 7
- **验证**: `/help` → 出现本地系统消息列出 5 个命令

### Task 9: `/leave` 命令端到端
- **文件**: `builtins.ts`（execute 中调用 `api.leaveChannel`）
- **行数**: ~5 行
- **依赖**: Task 6, Task 7
- **验证**: `/leave` → 弹出确认框 → 确认后离开频道并跳转
- **后端**: 无新增，使用现有 `POST /channels/:id/leave` 接口

### Task 10: `/topic` 命令端到端（前端）
- **文件**: `builtins.ts` execute 调用 `api.updateChannel`
- **行数**: ~5 行
- **依赖**: Task 6, Task 10A
- **验证**: `/topic New topic` → channel topic 更新，其他客户端实时收到更新

### Task 10A: `/topic` 后端 — topic 字段支持 + WebSocket 广播
- **文件**: 修改 `packages/server/src/routes/channels.ts`
- **行数**: ~30 行
- **内容**:
  - channel update route handler 增加 `topic` 字段的接受和校验
  - 添加 migration（如 `topic` 列不存在）
  - 授权检查：仅频道成员（或管理员）可设置 topic
  - WebSocket 广播：更新 topic 后 emit `channel:updated` 事件，携带新 topic，使其他客户端实时更新
- **依赖**: 无（可与前端 Task 并行开发）
- **验证**: `PUT /channels/:id { topic: "new" }` 返回 200 且 WebSocket 收到 `channel:updated` 事件；非成员调用返回 403

### Task 11: 用户参数命令的 MentionPicker 复用
- **文件**: 修改 `MessageInput.tsx`
- **行数**: ~20 行
- **内容**: `/invite ` 和 `/dm ` 后进入参数阶段，触发 MentionPicker（复用现有 `@` 逻辑），选中用户后 `resolvedUser` 存入 hook state
- **依赖**: Task 5, Task 6
- **验证**: 输入 `/invite ` 后弹出用户列表，选择后自动填入 `@username`

### Task 11A: text 参数输入提示 UX
- **文件**: 修改 `MessageInput.tsx` + `useSlashCommands.ts`
- **行数**: ~15 行
- **内容**: `paramType: "text"` 命令（如 `/topic`）进入参数阶段后，在输入框中显示 inline placeholder 提示（如 "输入频道主题…"），提示文本从 `CommandDefinition` 的 `usage` 或新增 `placeholder` 字段获取
- **依赖**: Task 5
- **验证**: 输入 `/topic ` 后输入框显示占位提示文字；输入内容后提示消失

### Task 12: `/invite` 命令端到端
- **文件**: `builtins.ts`（execute 中调用 `api.addChannelMember`）
- **行数**: 0（API 已存在：`addChannelMember`）
- **依赖**: Task 6, Task 11
- **验证**: `/invite @bob` → bob 被加入频道（API 200）

### Task 13: `/dm` 命令端到端
- **文件**: `builtins.ts`（execute 中调用 `dispatch({ type: "OPEN_DM" })`）
- **行数**: 0
- **依赖**: Task 6, Task 7, Task 11
- **验证**: `/dm @alice` → 跳转到与 alice 的 DM

### Task 14: 错误处理 + inline error UI
- **文件**: 修改 `MessageInput.tsx`
- **行数**: ~20 行
- **内容**: catch `CommandError`（缺参数）和 `ApiError`（后端失败），在输入框下方显示短暂错误提示（复用 `send-status-error` 样式）
- **依赖**: Task 6
- **验证**: `/invite` 无参数 → "Usage: /invite @user"；API 403 → 显示服务器错误消息

---

## 依赖关系图

```
Task 1 (registry) ──┬── Task 2 (builtins) ──┐
                    ├── Task 3 (hook)       ├── Task 5 (集成 + 无参即执行)
                    └── Task 4 (picker)     ┘         │
                                                       ├── Task 6 (执行拦截)
Task 7 (system msg + navigate actions) ───────────────┤
                                                       │
Task 10A (/topic 后端) ──────────────────────┐        ├── Task 8  (/help e2e)
                                              │        ├── Task 9  (/leave e2e)
                                              └── Task 10 (/topic e2e)
                                                       │
                              Task 11 (user-param) ────┤
                                                       ├── Task 12 (/invite e2e) ← 依赖 Task 11
                                                       ├── Task 13 (/dm e2e)     ← 依赖 Task 11
                              Task 11A (text hint UX) ─┘
                                                       │
                                                  Task 14 (错误处理)
```

## Review 修正说明

| # | 问题 | 修正 |
|---|------|------|
| 1 | `/leave` API 不一致：设计文档写 `DELETE /members/:userId`，task 写 `POST /leave` | 统一使用已有的 `POST /channels/:id/leave`（Task 9 标注）。**注：设计文档 §3.3 中的 DELETE 示例为过时写法，以 task breakdown 为准** |
| 2 | 缺 `NAVIGATE_AFTER_LEAVE` action | 合并到 Task 7，与 `INSERT_LOCAL_SYSTEM_MESSAGE` 一起实现 |
| 3 | `/topic` 后端并非"无需改动"，需要 topic 校验 + WebSocket 广播 | 新增 Task 10A 专门处理后端 topic 支持 |
| 4 | `paramType: "none"` 命令应选中即执行，不应仅插入输入框 | Task 5 重写：Tab/Enter 选中无参命令后直接执行，不插入文本 |
| 5 | `/invite` `/dm` 依赖用户参数解析（Task 11）但原依赖图未体现 | `/invite` 拆为 Task 12、`/dm` 拆为 Task 13，均显式依赖 Task 11 |
| 6 | 缺 text 参数输入提示 UX | 新增 Task 11A 处理 `/topic` 等 text 参数的 placeholder 提示 |

## 总结

- **新建文件**: 4 个（registry.ts, builtins.ts, useSlashCommands.ts, SlashCommandPicker.tsx）
- **修改文件**: 3 个（MessageInput.tsx, AppContext.tsx, channels.ts）+ CSS
- **后端改动**: Task 10A — topic 字段支持、授权、WebSocket 广播
- **总预估**: ~450 行新代码 + ~30 行 CSS + ~30 行后端
- **关键路径**: Task 1 → 2/3/4 → 5 → 6 → 11 → 12/13（Task 7、10A 可并行）
