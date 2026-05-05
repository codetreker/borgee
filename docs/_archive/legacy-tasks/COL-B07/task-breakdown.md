# B07/B08 Slash Commands - 详细 Task Breakdown

> 基于 [design.md](./design.md) 中的 10 个 task,细化为可执行的实现计划。

---

## 依赖关系总览

```
T1 ──→ T2 ──→ T3 ──┐
                    ├──→ T6 ──→ T8
T4a ──→ T4b ──→ T4c┘
T5 ─────────────────┘
T7(独立,可与 T1-T4 并行)
```

**可并行组**:
- **并行组 A**:T1 + T5 + T7(无依赖交叉)
- **并行组 B**:T4a 可与 T1/T2 并行
- T8(测试)在所有功能 task 完成后执行

---

## Task 1: Server - CommandStore 内存存储

**来源**:design.md #1

**说明**:实现 `CommandStore` 类,提供命令的注册(snapshot 全量替换)、按 connectionId 清除、按名称查询、全量列出(按 agentId 分组)功能。含 100 条/Agent 上限校验与内置命令名冲突跳过逻辑。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/server/src/command-store.ts` | **新增** | ~110 行 |
| `packages/server/src/types.ts` | 修改(新增 `AgentCommand` 接口) | ~10 行 |

### 详细内容

- `AgentCommand` 接口:`name, description, usage, params: Array<{ name, type, required?, placeholder? }>` (支持多参数;单参数命令 params 长度为 1)
- `CommandStore` 类:
  - `commands` 主数组 + `byConnection` / `byName` 索引 Map
  - `register(agentId, connectionId, commands, builtinNames)` → `{ registered, skipped }`
  - `unregisterByConnection(connectionId)`
  - `getAll()` → 按 agentId 分组
  - `getByName(name)` → 返回所有匹配(含 agentId)
  - `rebuildIndexes()` 内部方法
- 导出单例 `commandStore`

### 验证方式

```bash
cd packages/server && npx vitest run src/__tests__/command-store.test.ts
```

### 前置依赖

无

---

## Task 2: Server - WS `register_commands` 处理

**来源**:design.md #2

**说明**:在 `ws.ts` 中处理 Agent 发来的 `register_commands` 消息类型。校验 `role === 'agent'`,调用 `commandStore.register()`,广播 `commands_updated` 事件。连接断开时调用 `unregisterByConnection()` 并广播。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/server/src/ws.ts` | 修改 | ~55 行 |

### 详细内容

- `WsClient` 接口新增 `connectionId: string`(用 `crypto.randomUUID()`)和 `role?: string`
- 连接建立时从 WS 握手 JWT token 的 claim 中提取 `role`,存入 `WsClient`(不额外查 DB;role 在连接生命周期内不可变)
- **heartbeat/ping-pong**:确认 ws.ts 已有 heartbeat 机制(或新增 30s ping interval + 10s pong timeout),确保 TCP 半开连接被检测并触发 `close` → cleanup,防止僵尸命令
- `case 'register_commands':`
  - 校验 `client.role === 'agent'`,否则 nack
  - 调用 `commandStore.register(client.userId, client.connectionId, msg.commands, BUILTIN_NAMES)`
  - `broadcastAll({ type: 'commands_updated' })`
- `socket.on('close')` 中新增 `commandStore.unregisterByConnection(client.connectionId)` + `broadcastAll({ type: 'commands_updated' })`
- 新增 `broadcastAll()` 辅助函数(广播给所有已连接客户端)
- `BUILTIN_NAMES`:`new Set(['help','leave','topic','invite','dm','status','clear','nick'])`

### 验证方式

```bash
cd packages/server && npx vitest run src/__tests__/slash-ws-e2e.test.ts
```

### 前置依赖

T1

---

## Task 3: Server - `GET /api/v1/commands` + WS 事件

**来源**:design.md #3

**说明**:新增 HTTP 端点供前端获取所有可用命令(内置 + Agent)。返回格式按设计文档 `{ builtin, agent }` 结构。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/server/src/routes/commands.ts` | **新增** | ~65 行 |
| `packages/server/src/index.ts` | 修改(注册路由) | ~3 行 |

### 详细内容

- `registerCommandRoutes(app)` 函数
- `GET /api/v1/commands?channelId=xxx`(需认证):
  - 支持可选 `channelId` query 参数,按频道过滤 Agent 命令(根据频道成员列表,仅返回当前频道内 Agent 的命令)
  - 内置命令列表(硬编码 8 个:help/leave/topic/invite/dm/status/clear/nick)
  - Agent 命令:`commandStore.getAll()`,按 channelId 过滤后,查 DB 获取 agent_name
- Response schema 与 design.md 一致

### 验证方式

```bash
cd packages/server && npx vitest run src/__tests__/commands.test.ts
```

### 前置依赖

T1, T2

---

## Task 4a: Server - 协议/Schema 定义(`command` content_type)

**来源**:design.md #4a

**说明**:扩展消息 content_type 白名单,使 `'command'` 类型消息能通过 WS 发送。定义 command 消息的 content JSON schema。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/server/src/ws.ts` | 修改(content_type 白名单) | ~3 行 |
| `packages/server/src/types.ts` | 修改(CommandMessage 类型) | ~10 行 |

### 详细内容

- ws.ts L268-270:将 `ct !== 'text' && ct !== 'image'` 改为 `!['text','image','command'].includes(ct)`
- `types.ts` 新增 `CommandMessageContent` 接口:`{ command: string; args: string }`
- command 消息的 `content` 字段为 JSON 字符串:`JSON.stringify({ command, args })`
- `mentions` 字段包含目标 agentId

### 验证方式

```bash
cd packages/server && npx vitest run src/__tests__/messages.test.ts
```

### 前置依赖

无(可与 T1/T2 并行)

---

## Task 4b: Server - 校验 + 权限(command_id 生成)

**来源**:design.md #4b

**说明**:Server 在处理 `content_type: 'command'` 消息时,自动生成 `command_id` 写入消息 metadata,确保 sender_id 从认证态填充(现有逻辑已满足)。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/server/src/ws.ts` | 修改(command 消息特殊处理) | ~15 行 |
| `packages/server/src/queries.ts` | 修改(createMessage 支持 metadata) | ~8 行 |
| `packages/client/src/components/CommandParamForm.tsx` | **新增**(多参数输入表单,线框图 10d) | ~70 行 |

### 详细内容

- WS `send_message` handler 中,当 `ct === 'command'` 时:
  - `command_id` 就是消息 ID 本身（`Q.createMessage()` 返回的 `id`），不另外生成 UUID
  - 校验 `msg.mentions` 至少包含一个 agentId
  - **多参数支持**：`content` JSON 中 `args` 改为 `params: Array<{ name, value }>`，对应 `AgentCommand.params` 定义
- **参数输入表单 UI**(`CommandParamForm.tsx`,~70 行):
  - 选中多参数命令后,弹出参数输入面板(线框图 10d)
  - 根据 `AgentCommand.params` 渲染表单字段(必填 `*` 标记、placeholder)
  - Cancel / Execute 按钮
  - 校验必填字段后组装 `params` 数组发送

### 验证方式

```bash
cd packages/server && npx vitest run src/__tests__/messages.test.ts
```

### 前置依赖

T4a

---

## Task 4c: Client - pending/timeout UX

**来源**:design.md #4c

**说明**:前端追踪 command 消息的 pending 状态,30s 无 Agent 回复(`reply_to_id` 匹配 `command_id`)时显示超时提示。消息流中 command 类型消息带 ⚡ 前缀,执行中显示 ⏳ 状态。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/client/src/hooks/useCommandTracking.ts` | **新增** | ~60 行 |
| `packages/client/src/components/MessageItem.tsx` | 修改(command 消息渲染) | ~45 行 |
| `packages/client/src/components/CommandResultCard.tsx` | **新增** | ~55 行 |

### 详细内容

- `useCommandTracking` hook：
  - 维护 `Map<messageId, { timestamp, status }>` pending 状态（key = 命令消息的 ID）
  - 监听新消息，匹配 `reply_to_id === messageId` 标记完成
  - 30s `setTimeout` 超时后标记 timeout
- `MessageItem.tsx`:
  - `content_type === 'command'` 时渲染 ⚡ 前缀 + 命令名 + 参数
  - 关联 `CommandResultCard` 显示 executing/completed/failed/timeout
- `CommandResultCard.tsx`(UI 线框图 10e):
  - 状态图标:⏳ executing / ✅ completed / ❌ failed / ⏰ timeout
  - Agent 名 + 状态文案
  - 进度条占位(executing 时)

### 验证方式

```bash
cd packages/client && npx tsc --noEmit
# 手动验证:发送 command 消息 → 30s 后显示超时提示
```

### 前置依赖

T4a, T4b

---

## Task 5: Client - CommandRegistry 扩展

**来源**:design.md #5

**说明**:扩展 `CommandRegistry` 支持远程命令(Agent 注册的命令)。新增 `RemoteCommand` 类型、`setRemoteCommands()`、`resolve()` 三态返回(builtin/remote/ambiguous)、`search()` 按 Agent 分组。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/client/src/commands/registry.ts` | 修改 | ~70 行(净增) |
| `packages/client/src/hooks/useSlashCommands.ts` | 修改(适配分组结构) | ~30 行 |
| `packages/client/src/components/SlashCommandPicker.tsx` | 重写(分组显示 + Agent 选择 + 键盘导航跨组) | ~220 行(净增 ~180) |

### 详细内容

- `registry.ts`:
  - 新增 `RemoteCommand` 接口(含 agentId, agentName)
  - `setRemoteCommands(commands: RemoteCommand[])`:全量替换 + 重建 byName 索引
  - `resolve(name)` → `{ type: 'builtin' | 'remote' | 'ambiguous', ... } | null`
  - `search(prefix)` → `Array<{ group: string; items: ... }>`(分组结果)
  - 保留 `register()`/`get()`/`all()` 向后兼容
- `useSlashCommands.ts`:
  - `filtered` 改为分组结构 `{ group, items }[]`
  - 支持展开 ambiguous 场景(多 Agent 同名)
- `SlashCommandPicker.tsx`(对应 UI 线框图 10a/10b/10c,净增 ~180 行):
  - 分组标题:`── System ──` / `── 🤖 AgentName ──`
  - 面板标题:`⚡ Slash Commands` + 关闭按钮 `[✕]`
  - **键盘导航**:上下选择跨组、Enter 确认、Esc 关闭
  - 同名命令:点击/Enter 后弹出 Agent 选择子面板(10c 线框图),含独立状态管理
  - Agent 选择卡片:Agent 名 + 命令描述
  - 现有 5 个内置命令(help/invite/leave/topic/dm)在分组 UI 中正常显示(不回归)

### 验证方式

```bash
cd packages/client && npx tsc --noEmit
# 手动验证:输入 / 查看分组列表,输入 /de 查看过滤,同名命令弹出选择
```

### 前置依赖

无(可与 T1 并行)

---

## Task 6: Client - 命令加载 + WS 同步

**来源**:design.md #6

**说明**:应用启动时 fetch `GET /api/v1/commands` 加载远程命令,监听 WS `commands_updated` 事件后重新 fetch。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/client/src/lib/api.ts` | 修改(新增 `listCommands()` 方法) | ~15 行 |
| `packages/client/src/components/ChannelView.tsx` | 修改(启动加载 + WS 监听) | ~25 行 |

### 详细内容

- `api.ts`:
  - `listCommands()`: `GET /api/v1/commands` → 返回 `{ builtin, agent }` 结构
- `ChannelView.tsx`(或 App 级别):
  - `useEffect` 初次加载时调 `api.listCommands()`(传入当前 channelId)
  - 将 agent 命令 flatMap 为 `RemoteCommand[]`,调 `commandRegistry.setRemoteCommands()`
  - WS 监听 `commands_updated` 事件 → **debounce 300ms** 后重新 fetch + setRemoteCommands(防止多连接注册时的广播风暴触发 N 次 fetch)

### 验证方式

```bash
cd packages/client && npx tsc --noEmit
# 手动验证:Agent 注册命令后,前端不刷新即可在 / 面板看到新命令
```

### 前置依赖

T3, T5

---

## Task 7: Client - `/status` `/clear` `/nick` 内置命令

**来源**:design.md #7(B08 范围)

**说明**:在 `builtins.ts` 中注册 3 个新内置命令。UI 按线框图 10f/10g/10h 实现。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/client/src/commands/builtins.ts` | 修改(新增 3 个命令) | ~55 行 |
| `packages/client/src/components/ChannelStatusCard.tsx` | **新增**(/status 结果卡片,线框图 10f) | ~65 行 |
| `packages/client/src/components/ClearConfirmModal.tsx` | **新增**(/clear 确认弹窗,线框图 10g) | ~50 行 |
| `packages/client/src/lib/api.ts` | 修改(若缺少 `updateProfile`/`getChannelMembers`) | ~10 行 |

### 详细内容

- `/status`(线框图 10f):
  - 调 `api.getChannel()` + `api.getChannelMembers()`
  - 渲染 `ChannelStatusCard`:频道名、成员总数、在线/离线分组列表、Agent active/inactive 状态
  - ephemeral 消息(`INSERT_LOCAL_SYSTEM_MESSAGE`)
- `/clear`(线框图 10g):
  - 弹出 `ClearConfirmModal`:半透明遮罩 + 警告文案 + Cancel/Clear 按钮
  - 确认后 `dispatch({ type: 'CLEAR_LOCAL_MESSAGES' })`
  - 仅清除本地视图,不影响服务端
- `/nick`(线框图 10h):
  - 参数类型 `text`,placeholder "新显示名..."
  - 调 `api.updateProfile({ display_name: args })`
  - 成功后 `INSERT_LOCAL_SYSTEM_MESSAGE`:`✅ Nickname changed: Old → New`

### 验证方式

```bash
cd packages/client && npx tsc --noEmit
# 手动验证:/status 显示频道状态卡片,/clear 弹确认后清除,/nick 修改显示名
```

### 前置依赖

无(可与 T1-T4 并行)

---

## Task 8: 测试

**来源**:design.md #8

**说明**:单元测试 + 集成测试覆盖所有核心逻辑。

### 改动文件

| 文件 | 操作 | 预估行数 |
|------|------|----------|
| `packages/server/src/__tests__/command-store.test.ts` | **新增** | ~120 行 |
| `packages/server/src/__tests__/commands.test.ts` | **新增** | ~80 行 |
| `packages/server/src/__tests__/slash-commands-e2e.test.ts` | **新增** | ~150 行 |
| `packages/client/src/__tests__/command-registry.test.ts` | **新增** | ~100 行 |

### 详细内容

#### 单元测试:`command-store.test.ts`

- register → 查询返回已注册命令
- register 超过 100 条 → 抛错
- 内置同名命令 → skipped 返回
- unregisterByConnection → 仅清除该连接的命令
- getAll → 按 agentId 分组
- getByName → 返回所有匹配
- snapshot 语义:同连接注册两次 → 后者替换前者
- rebuildIndexes 一致性

#### API 测试:`commands.test.ts`

- `GET /api/v1/commands` 返回 builtin + agent 结构
- 无 Agent 命令时 agent 为空数组
- 需认证(401)

#### 前端单元测试:`command-registry.test.ts`

- resolve: builtin 命令 → type: 'builtin'
- resolve: 唯一 remote 命令 → type: 'remote'(含 agentId)
- resolve: 多 Agent 同名 → type: 'ambiguous'(含候选列表)
- resolve: 未知命令 → null
- resolve: builtin > remote 优先级
- search: prefix 过滤 + 分组正确性(System 组 + Agent 组)
- setRemoteCommands: 全量替换语义

#### E2E 集成测试:`slash-commands-e2e.test.ts`

- 完整流程:Agent WS 连接 → `register_commands` → `commands_updated` 广播 → `GET /api/v1/commands` 包含命令 → 用户发送 `content_type: 'command'` 消息 → Agent 收到 → Agent reply_to_id 回复
- WS 断开 → 命令自动清除 → `commands_updated` 广播
- 非 agent 角色发 `register_commands` → 拒绝
- 同一 Agent 多连接独立注册/清除
- 101 条命令 → 400 错误

### 验证方式

```bash
cd packages/server && npx vitest run src/__tests__/command-store.test.ts src/__tests__/commands.test.ts src/__tests__/slash-commands-e2e.test.ts
```

### 前置依赖

T1-T7 全部完成

---

## 汇总

| Task | 新增文件 | 修改文件 | 预估总行数 | 可并行 |
|------|----------|----------|------------|--------|
| T1 | `command-store.ts` | `types.ts` | ~120 | ✅ 并行组 A |
| T2 | - | `ws.ts` | ~55 | - |
| T3 | `routes/commands.ts` | `index.ts` | ~68 | - |
| T4a | - | `ws.ts`, `types.ts` | ~13 | ✅ 并行组 B |
| T4b | - | `ws.ts`, `queries.ts`; 新增 `CommandParamForm.tsx` | ~93 | - |
| T4c | `useCommandTracking.ts`, `CommandResultCard.tsx` | `MessageItem.tsx` | ~160 | - |
| T5 | - | `registry.ts`, `useSlashCommands.ts`, `SlashCommandPicker.tsx` | ~280 | ✅ 并行组 A |
| T6 | - | `api.ts`, `ChannelView.tsx` | ~40 | - |
| T7 | `ChannelStatusCard.tsx`, `ClearConfirmModal.tsx` | `builtins.ts`, `api.ts` | ~180 | ✅ 并行组 A |
| T8 | `command-store.test.ts`, `commands.test.ts`, `slash-commands-e2e.test.ts`, `command-registry.test.ts` | - | ~450 | - |

**新增文件**:9 个
**修改文件**:10 个
**预估总行数**:~1,489 行
**预估总工时**:~13h

### 建议实施顺序

```
第 1 轮(并行):T1 + T4a + T5 + T7
第 2 轮:T2(依赖 T1)
第 3 轮(并行):T3(依赖 T2)+ T4b(依赖 T4a)
第 4 轮(并行):T4c(依赖 T4b)+ T6(依赖 T3 + T5)
第 5 轮:T8(全部功能完成后)
```

---

## Review 修正记录

> 基于 CC Round 1 + CC Round 2 的 CRITICAL/HIGH 问题,对 task-breakdown 做了以下修正:

| # | 级别 | 问题 | 处理方式 |
|---|--------|------|----------|
| CC1-C1 | CRITICAL | 多参数命令完全缺失 | T1 `AgentCommand.params` 改为数组;T4b 新增 `CommandParamForm.tsx`(~70行)实现线框图 10d 的参数输入表单 |
| CC1-C2 | CRITICAL | 频道维度命令可见性缺失 | T3 `GET /api/v1/commands` 新增可选 `channelId` query 参数,按频道成员过滤 Agent 命令 |
| CC1-C4 | CRITICAL | 参数输入表单 UI 无对应 task | 合入 T4b(新增 `CommandParamForm.tsx`) |
| CC2-#1 | CRITICAL | role 判定时机与来源不清晰 | T2 明确:role 从 WS 握手 JWT claim 提取,不额外查 DB,连接期间不可变 |
| CC1-C3 | CRITICAL | 命令消息持久化 vs PRD "不持久化"矛盾 | **已决策（飞马 2026-04-24）**：持久化。`content_type: 'command'` 走正常消息存储。PRD "不持久化"指命令注册列表（内存），不是命令执行消息 |
| CC2-#2 | HIGH | WS 半开连接无 heartbeat 保护 | T2 新增 heartbeat/ping-pong 子项(30s ping + 10s pong timeout) |
| CC2-#3 | HIGH | 多连接注册广播风暴 | T6 WS 监听加 debounce 300ms |
| CC1-H3 / CC2-#5 | HIGH | 前端 CommandRegistry 单元测试完全缺失 | T8 新增 `command-registry.test.ts`(~100行),覆盖 resolve 三态 + search 分组 |
| CC2-#6 | HIGH | SlashCommandPicker 行数预估偏低 | ~80 → ~180(含键盘导航跨组、Agent 选择子面板独立状态) |
| CC2-#7 | HIGH | 总行数预估偏低 ~20-30% | 总行数从 ~1,189 修正为 ~1,489 |

### 已决策项（飞马 2026-04-24 确认）

- **CC1-C3**：命令消息持久化 → ✅ 持久化。`content_type: 'command'` 走正常消息存储，PRD "不持久化"指注册列表（内存）
- **CC1-H5**：PRD "删除单条" → ✅ Snapshot 全量替换。Agent 重发不含该命令的完整列表即为删除。PRD 由 PM 修正
- **CC1-H4**：command_id → ✅ 就是消息 ID 本身，不另加字段。Agent 用 `reply_to_id` 回复该消息即可关联，前端用消息 ID 追踪 pending
