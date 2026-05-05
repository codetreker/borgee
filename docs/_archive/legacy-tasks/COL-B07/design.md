# B07/B08 Slash Commands v2 技术设计

## 背景与问题

Collab 当前有 5 个内置 slash commands（`/help /leave /topic /invite /dm`），全部在前端 `CommandRegistry` 中注册并本地执行。无法扩展：Agent 不能注册自定义命令，也缺少 `/status /clear /nick` 等常用命令。

## 目标

1. Agent 通过 API 注册/管理自定义 slash commands（B07）
2. 用户执行 Agent 命令时通过消息通道发给 Agent 处理
3. 新增 `/status /clear /nick` 内置命令（B08）
4. 前端命令面板实时显示所有命令（内置 + Agent）

### UI 设计稿

- [命令面板线框图](../ui/slash-commands.md)（8 个界面：命令列表/搜索/同名选择/参数输入/执行结果/status/clear/nick）

### 验收标准

- Agent 能注册/删除/列出命令
- 用户输入 Agent 命令后，Agent 收到 `content_type: 'command'` 消息
- `/status /clear /nick` 正常工作
- 命令列表实时同步（WS 事件）

## 设计决策

| 决策 | 结论 | 理由 |
|------|------|------|
| 传输方式 | 消息通道（`content_type: 'command'`） | Agent 已有 WS/SSE 连接，复用现有基础设施 |
| 同 Agent 同名 | upsert 覆盖 | 后注册覆盖先注册 |
| 不同 Agent 同名 | 前端按 Agent 分组显示，用户选择 | 直观，不需记前缀 |
| 内置优先级 | 内置命令始终优先于 Agent 命令 | 防止 Agent 覆盖系统命令 |
| 命令上限 | 100/Agent | 够用，防滥用 |
| Agent 离线 | 前端 30s 超时提示 | 避免用户无限等待 |
| 命令存储 | 服务器内存（Map），不存 DB | 零 DB 写入，无僵尸命令，不引入 Redis |
| 命令生命周期 | WS 连接时注册，断开时自动清除 | 在线 Agent = 有效命令 |

## 方案设计

### 1. 命令存储（Server 内存）

命令不存 DB，存在 server 内存中。WS 连接时注册，断开时自动清除。

```typescript
// command-registry.ts
interface AgentCommand {
  name: string;
  description: string;
  usage: string;
  paramType: 'none' | 'text' | 'user' | 'number';
  placeholder?: string;
}

class CommandStore {
  private commands: Array<AgentCommand & { agentId: string; connectionId: string }> = [];
  private byConnection = new Map<string, Array<AgentCommand & { agentId: string; connectionId: string }>>();
  private byName = new Map<string, Array<AgentCommand & { agentId: string; connectionId: string }>>();

  private rebuildIndexes(): void {
    this.byConnection.clear();
    this.byName.clear();
    for (const cmd of this.commands) {
      const connList = this.byConnection.get(cmd.connectionId) ?? [];
      connList.push(cmd);
      this.byConnection.set(cmd.connectionId, connList);
      const nameList = this.byName.get(cmd.name) ?? [];
      nameList.push(cmd);
      this.byName.set(cmd.name, nameList);
    }
  }

  register(agentId: string, connectionId: string, incoming: AgentCommand[], builtinNames: Set<string>): { registered: AgentCommand[]; skipped: AgentCommand[] } {
    if (incoming.length > 100) throw new Error('Command limit exceeded (100)');
    // Remove previous commands for this connectionId (snapshot semantics)
    this.commands = this.commands.filter(c => c.connectionId !== connectionId);
    const registered: AgentCommand[] = [];
    const skipped: AgentCommand[] = [];
    for (const cmd of incoming) {
      if (builtinNames.has(cmd.name)) {
        skipped.push(cmd);
        continue;
      }
      registered.push(cmd);
      this.commands.push({ ...cmd, agentId, connectionId });
    }
    this.rebuildIndexes();
    return { registered, skipped };
  }

  unregisterByConnection(connectionId: string): void {
    this.commands = this.commands.filter(c => c.connectionId !== connectionId);
    this.rebuildIndexes();
  }

  getAll(): { agentId: string; commands: AgentCommand[] }[] {
    const grouped = new Map<string, AgentCommand[]>();
    for (const cmd of this.commands) {
      const list = grouped.get(cmd.agentId) ?? [];
      list.push(cmd);
      grouped.set(cmd.agentId, list);
    }
    return [...grouped.entries()].map(([agentId, cmds]) => ({ agentId, commands: cmds }));
  }

  getByName(name: string): Array<AgentCommand & { agentId: string }> {
    return this.byName.get(name) ?? [];
  }
}

export const commandStore = new CommandStore();
```

### 命令生命周期

每个 WS 连接有唯一 `connectionId`（由 WsClient 分配）。命令按 connectionId 注册和清除。

- `register_commands` 是**全量替换（snapshot）**语义：Agent 每次发送完整命令列表，Server 替换该 connectionId 下的所有命令。
- 同一 Agent 多个连接：后连接的 `register_commands` 独立注册，断开只清自己 connectionId 的命令。

```
Agent WS 连接建立（分配 connectionId）
    │
    ▼
Agent 发送 { type: 'register_commands', commands: [...] }
    │
    ▼
Server 校验发送者 role === 'agent'（拒绝普通用户注册命令）
    │
    ▼
Server 校验命令名不与内置命令冲突（冲突的跳过并 warn）
    │
    ▼
Server commandStore.register(agentId, connectionId, commands, builtinNames)
    │
    ▼
Server 广播 WS 事件 { type: 'commands_updated' }
    │
    ▼
前端收到事件，重新 fetch /api/v1/commands
    │
    ... Agent 在线期间命令可用 ...
    │
    ▼
Agent WS 断开
    │
    ▼
Server commandStore.unregisterByConnection(connectionId)
    │
    ▼
Server 广播 WS 事件 { type: 'commands_updated' }
    │
    ▼
前端移除该连接注册的命令
```

### 2. Server API

#### 注册命令（WS 消息，不是 HTTP API）

Agent 通过 WS 发送：

```json
{
  "type": "register_commands",
  "commands": [
    {
      "name": "deploy",
      "description": "部署到指定环境",
      "usage": "/deploy <env>",
      "param_type": "text",
      "placeholder": "staging / prod"
    }
  ]
}
```

Server 收到后：
1. 校验发送者 `role === 'agent'`，拒绝普通用户
2. 校验命令名不与内置命令冲突，冲突的跳过并 warn log
3. 全量替换该 connectionId 下的命令（snapshot 语义）
4. 广播 `commands_updated` 事件

#### 列出所有命令（前端用）

```
GET /api/v1/commands

Response 200:
{
  "builtin": [
    { "name": "help", "description": "显示所有可用命令", "usage": "/help", "param_type": "none" },
    { "name": "leave", "description": "离开当前频道", ... },
    { "name": "topic", ... },
    { "name": "invite", ... },
    { "name": "dm", ... },
    { "name": "status", ... },
    { "name": "clear", ... },
    { "name": "nick", ... }
  ],
  "agent": [
    {
      "agent_id": "agent-pegasus",
      "agent_name": "Pegasus",
      "commands": [
        { "name": "deploy", "description": "部署到指定环境", "usage": "/deploy <env>", "param_type": "text" }
      ]
    },
    {
      "agent_id": "agent-warhorse",
      "agent_name": "Warhorse",
      "commands": [
        { "name": "deploy", "description": "部署前端", "usage": "/deploy <branch>", "param_type": "text" }
      ]
    }
  ]
}
```

### 3. 命令执行流程

#### 同名命令解析策略

不同 Agent 可以注册同名命令。前端按 Agent 分组显示：

```
── 内置 ──
/help    显示所有可用命令
/status  显示频道状态
...
── Pegasus ──
/deploy  部署到指定环境
/search  搜索代码
── Warhorse ──
/deploy  部署前端
```

- **唯一命令**：直接输入 `/search args` 回车即执行，不弹选择
- **同名命令**：命令面板显示所有匹配，用户点选确定哪个 Agent
- **内置优先**：内置命令始终优先于 Agent 命令

```
用户输入 "/deploy staging"
    │
    ▼
前端 CommandRegistry.resolve("deploy")
    │
    ├─ 内置命令 → 本地执行（/help /leave /topic /invite /dm /status /clear /nick）
    │
    ├─ 唯一 Agent 命令 → 直接执行
    │
    ├─ 多个 Agent 同名 → 弹面板让用户选择 Agent
    │
    └─ 选定 Agent 后 → POST /api/v1/channels/:channelId/messages
        {
          "content": "{\"command\":\"deploy\",\"args\":\"staging\"}",
          "content_type": "command",
          "mentions": ["agent-pegasus"]
        }
        │
        ▼
    Server 需要在 ws.ts 的 content_type 白名单中加入 'command'
    Server 自动填充 sender_id（从认证态）、channel_id、timestamp
    Server 生成唯一 command_id 并写入消息 metadata
        │
        ▼
    Server 广播消息（WS/SSE/poll）
        │
        ▼
    Agent 收到 content_type=command 消息（含 command_id）
    Agent 解析 JSON，执行逻辑
    Agent 回复普通文本消息（content_type: text），附带 reply_to_id = command_id
        │
        ▼
    前端用 command_id 追踪 pending 状态，30s 超时后提示"Agent 未响应"
```

> **安全**：客户端不发送 `invoker_id`，所有身份信息由 Server 从认证态填充，防止伪造。

### 4. 前端扩展

#### CommandRegistry 改造

```typescript
// registry.ts 新增
interface RemoteCommand {
  name: string;
  description: string;
  usage: string;
  paramType: 'none' | 'text' | 'user' | 'number';
  placeholder?: string;
  agentId: string;
  agentName: string;
}

class CommandRegistry {
  private builtins: Map<string, CommandDefinition> = new Map();
  private remoteCommands: RemoteCommand[] = [];
  private remoteByName: Map<string, RemoteCommand[]> = new Map();

  registerBuiltin(cmd: CommandDefinition): void { ... }
  
  setRemoteCommands(commands: RemoteCommand[]): void {
    this.remoteCommands = commands;
    this.remoteByName = new Map();
    for (const cmd of commands) {
      const list = this.remoteByName.get(cmd.name) ?? [];
      list.push(cmd);
      this.remoteByName.set(cmd.name, list);
    }
  }

  resolve(name: string): 
    | { type: 'builtin', cmd: CommandDefinition } 
    | { type: 'remote', cmd: RemoteCommand }   // 唯一匹配
    | { type: 'ambiguous', cmds: RemoteCommand[] } // 多个 Agent 同名
    | null {
    const builtin = this.builtins.get(name);
    if (builtin) return { type: 'builtin', cmd: builtin };
    const matches = this.remoteByName.get(name) ?? [];
    if (matches.length === 1) return { type: 'remote', cmd: matches[0] };
    if (matches.length > 1) return { type: 'ambiguous', cmds: matches };
    return null;
  }

  search(prefix: string): Array<{ group: string; items: (CommandDefinition | RemoteCommand)[] }> {
    const results: Array<{ group: string; items: (CommandDefinition | RemoteCommand)[] }> = [];
    const builtinMatches = [...this.builtins.values()].filter(c => c.name.startsWith(prefix));
    if (builtinMatches.length) results.push({ group: '内置', items: builtinMatches });
    const agentGroups = new Map<string, RemoteCommand[]>();
    for (const cmd of this.remoteCommands) {
      if (!cmd.name.startsWith(prefix)) continue;
      const list = agentGroups.get(cmd.agentName) ?? [];
      list.push(cmd);
      agentGroups.set(cmd.agentName, list);
    }
    for (const [agentName, cmds] of agentGroups) {
      results.push({ group: agentName, items: cmds });
    }
    return results;
  }
}
```

#### 启动加载 + WS 实时同步

```typescript
// App 启动时
const { builtin, agent } = await api.listCommands();
const remoteCommands = agent.flatMap(a => 
  a.commands.map(c => ({ ...c, agentId: a.agent_id, agentName: a.agent_name }))
);
commandRegistry.setRemoteCommands(remoteCommands);

// WS 事件监听（统一使用 commands_updated 事件）
ws.on('commands_updated', () => { /* 重新 fetch 命令列表 */ });
```

### 5. WS 事件

命令变更时，server 通过 WS 广播事件，前端重新 fetch 命令列表：

```json
{ "type": "commands_updated" }
```

简化为一个事件，前端收到后调 `GET /api/v1/commands` 全量刷新。

### 6. B08 内置命令

#### /status

```typescript
commandRegistry.registerBuiltin({
  name: 'status',
  description: '显示频道状态',
  usage: '/status',
  paramType: 'none',
  execute: async ({ channelId, api, dispatch }) => {
    const channel = await api.getChannel(channelId);
    const members = await api.getChannelMembers(channelId);
    const online = members.filter(m => m.online).length;
    dispatch({
      type: 'INSERT_LOCAL_SYSTEM_MESSAGE',
      payload: {
        channelId,
        text: `**${channel.name}**\n主题: ${channel.topic || '无'}\n成员: ${members.length}\n在线: ${online}`,
      },
    });
  },
});
```

#### /clear

```typescript
commandRegistry.registerBuiltin({
  name: 'clear',
  description: '清除本地聊天记录',
  usage: '/clear',
  paramType: 'none',
  execute: async ({ channelId, dispatch }) => {
    dispatch({ type: 'CLEAR_LOCAL_MESSAGES', payload: { channelId } });
  },
});
```

#### /nick

```typescript
commandRegistry.registerBuiltin({
  name: 'nick',
  description: '修改显示名',
  usage: '/nick <name>',
  paramType: 'text',
  placeholder: '新显示名…',
  execute: async ({ args, api, dispatch, channelId }) => {
    if (!args.trim()) throw new CommandError('Usage: /nick <name>');
    await api.updateProfile({ display_name: args.trim() });
    dispatch({
      type: 'INSERT_LOCAL_SYSTEM_MESSAGE',
      payload: { channelId, text: `显示名已改为 ${args.trim()}` },
    });
  },
});
```

## 否决方案

### Webhook 回调（已否决）

Agent 注册命令时提供 callback URL。否决理由：Agent 需暴露公网端口，与现有 WS 通道重复。

### 全局唯一命令名 + Agent 前缀（已否决）

`/agentA:search` 格式。否决理由：用户需记前缀，学习成本高。

### DB 持久化存储（已否决）

命令存 SQLite 或 Redis。否决理由：命令是 Agent 在线时的能力声明，不是持久化数据。内存存储零 IO，断开自动清理，无僵尸命令。

## 测试策略

### 单元测试

- CommandStore 内存操作：register/unregisterByConnection、索引重建、上限检查
- CommandRegistry（前端）：resolve/search 覆盖内置+远程+冲突+ambiguous
- 内置同名拒绝：注册与内置命令同名时跳过并返回 skipped
- 权限校验：非 agent 角色发送 register_commands 被拒绝

### 集成测试

- 完整流程：Agent 注册命令 → 用户执行 → Server 生成 command_id → Agent 收到 command 消息 → Agent 用 reply_to_id 回复
- WS 生命周期：注册后前端收到 commands_updated → 断开后命令自动清除
- 连接竞争：同一 Agent 多连接各自注册，断开只清自己 connectionId 的命令
- 边界：101 个命令 → 400 错误
- Snapshot 语义：同一连接注册两次 → 后者全量替换前者

### E2E 测试

- 浏览器输入 `/` → 命令面板显示所有命令
- 选择 Agent 命令 → 消息发送 → Agent 回复

## Task Breakdown

| # | 任务 | Scope | 预估 |
|---|------|-------|------|
| 1 | Server: CommandStore 内存存储 | 命令注册/清除/查询 | 1h |
| 2 | Server: WS register_commands 处理 | 连接时注册，断开时清除 | 1.5h |
| 3 | Server: GET /api/v1/commands + WS 事件 | 前端查询 + 实时同步 | 1h |
| 4a | Server: 协议/Schema 定义 | command 消息 content_type、command_id、reply_to_id schema | 0.5h |
| 4b | Server: 校验 + 权限 | content_type 白名单扩展('command')、sender_id 填充、role 校验 | 0.5h |
| 4c | Client: pending/timeout UX | command_id 追踪 pending 状态、30s 超时提示 | 1h |
| 5 | Client: CommandRegistry 扩展 | 远程命令支持 + resolve/search | 1.5h |
| 6 | Client: 命令加载 + WS 同步 | 启动 fetch + 事件监听 | 1h |
| 7 | Client: /status /clear /nick | 3 个内置命令 | 1h |
| 8 | 测试 | 单测 + 集成测试 | 2h |

**总计：~11h**

## 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Agent 离线时命令无响应 | 用户困惑 | 前端 30s 超时提示"Agent 未响应" |
| 命令列表缓存不一致 | 显示过时命令 | WS 事件 + 页面刷新重新 fetch |
| 命令名与内置冲突 | 内置被覆盖 | 内置命令优先级高于 Agent 命令 |
| 大量 Agent 注册命令 | 命令面板臃肿 | 按 Agent 分组显示 + 搜索过滤 |
