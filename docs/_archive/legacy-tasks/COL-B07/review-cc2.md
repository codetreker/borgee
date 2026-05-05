# B07 Slash Commands — Review (CC Round 2)

> 独立审查 task-breakdown.md vs design.md，仅列 CRITICAL / HIGH 问题。

---

## 1. CommandStore 内存模型

### [HIGH] rebuildIndexes 全量重建性能不必要但更重要的是——snapshot register 存在 O(n) 扫描

`register()` 每次调用都 `this.commands.filter()` + `rebuildIndexes()`，后者遍历全部命令重建两个 Map。当多个 Agent 高频注册时（如部署后批量重连），这是 O(total_commands) × 注册次数。

**建议**：`register` 中只删除目标 connectionId 的条目并局部更新索引，不做全量 rebuild。或者至少在 `byConnection` Map 已经存在的情况下直接 `delete` + 局部插入。

### [HIGH] 内存泄漏风险——异常断开未触发 cleanup 的保护缺失

design.md 依赖 `socket.on('close')` 触发 `unregisterByConnection`。task-breakdown 未提及以下 edge case：
- **Server 进程重启**：内存 store 丢失，但 Agent 重连后会重新 register，所以可接受——但 task-breakdown 应显式说明这一点。
- **WS ping/pong 超时**：如果 server 没有配置 heartbeat，TCP 半开连接不会触发 `close` 事件，命令会成为僵尸条目。task-breakdown 未提及是否已有 heartbeat 机制或是否需要新增。

**建议**：T2 中增加一个子项：确认 ws.ts 已有 heartbeat/ping-pong（或新增），确保半开连接能被检测并触发 cleanup。

---

## 2. WS 注册协议 Edge Cases

### [CRITICAL] `role` 判定时机与来源不清晰

task-breakdown T2 写"连接建立时查询 user.role，存入 WsClient"，但 design.md 中 `role` 是指 Agent vs 普通用户。问题：
1. **role 来源**：是从 DB user 表查的？还是 WS 握手参数？还是 JWT claim？task-breakdown 未明确。如果是 DB 查询，每次 WS 连接都要一次 DB roundtrip。
2. **role 可变性**：如果用户在连接期间被提升/降级为 agent，已缓存的 role 不会更新。是否可接受？

**建议**：T2 中明确 role 的来源（推荐从 JWT/认证 token 中提取，避免额外 DB 查询），并注明 role 在连接生命周期内不可变。

### [HIGH] 多连接 + snapshot 语义下的广播风暴

同一 Agent 有 N 个连接，每个连接各自 `register_commands`，每次注册都 `broadcastAll({ type: 'commands_updated' })`。如果 Agent 启动时快速建立多个连接并各自注册，会导致前端在短时间内收到 N 次 `commands_updated`，触发 N 次 `GET /api/v1/commands`。

**建议**：前端 T6 的 WS 监听应加 debounce（200-500ms），task-breakdown 中未提及。

### [HIGH] `broadcastAll` 不区分频道

design.md 未限定 `commands_updated` 的广播范围。task-breakdown 新增 `broadcastAll()` 广播给**所有已连接客户端**。如果命令是全局的（不按频道隔离），这是合理的——但应显式确认命令是全局作用域还是频道作用域。当前设计隐含全局，但未明确声明。

---

## 3. 前端 CommandRegistry + 命令面板 UX

### [HIGH] resolve() 在用户直接回车时的 ambiguous 处理缺失

design.md 描述：用户输入 `/deploy staging` 回车 → 如果多 Agent 同名 → 弹面板选择。但 task-breakdown T5 中 `resolve()` 返回 `ambiguous` 后的**交互流程**未明确：
1. 用户已经按了回车，输入框中的文本（`/deploy staging`）是保留还是清空？
2. 弹出 Agent 选择面板后，用户选择 Agent，参数 `staging` 是否自动带入？
3. 如果用户取消选择，输入框恢复到什么状态？

**建议**：T5 或 T4c 中增加 ambiguous 场景的状态机描述（输入保留 → 弹选择 → 选中后发送 / 取消后恢复输入框）。

### [HIGH] SlashCommandPicker 120 行预估偏低

T5 预估 `SlashCommandPicker.tsx` 净增 ~80 行，但需要实现：
- 分组渲染（内置 + 多个 Agent 组）
- 键盘导航（上下选择跨组）
- 同名命令点击后弹出 Agent 选择子面板
- Agent 选择卡片 UI

仅 Agent 选择子面板（线框图 10c）就需要独立状态管理和渲染逻辑。实际预估应在 150-200 行。

---

## 4. 测试策略

### [HIGH] 前端 CommandRegistry 单元测试完全缺失

design.md 测试策略明确列出"CommandRegistry（前端）：resolve/search 覆盖内置+远程+冲突+ambiguous"，但 task-breakdown T8 只有 3 个 server 端测试文件，**没有任何前端测试文件**。

`registry.ts` 的 `resolve()` 三态逻辑和 `search()` 分组逻辑是核心业务逻辑，必须有单元测试。

**建议**：T8 新增 `packages/client/src/__tests__/command-registry.test.ts`，覆盖：
- resolve: builtin > remote 优先级
- resolve: 唯一 remote → type: 'remote'
- resolve: 多 Agent 同名 → type: 'ambiguous'
- resolve: 未知命令 → null
- search: prefix 过滤 + 分组正确性

### [HIGH] E2E 测试缺少"命令执行后 Agent 回复"的 reply_to_id 匹配验证

T8 E2E 测试列出"Agent reply_to_id 回复"，但 task-breakdown 未说明如何在测试中模拟 Agent 回复。需要：
1. 测试 WS client 模拟 Agent 发送 `content_type: 'text'` + `reply_to_id: command_id`
2. 验证 `command_id` 是 server 生成的 UUID（不是客户端传入的）

---

## 5. 预估行数

### [HIGH] 总行数偏低约 20-30%

| 区域 | task-breakdown 预估 | 实际预估 | 差异原因 |
|------|---------------------|----------|----------|
| T5 SlashCommandPicker | 80 行净增 | 150-200 行 | Agent 选择子面板、键盘导航跨组 |
| T4c CommandResultCard | 55 行 | 80-100 行 | 状态机 + 4 种状态渲染 + 动画 |
| T7 ChannelStatusCard | 65 行 | 90-110 行 | 在线/离线分组 + Agent 状态 + 响应式 |
| T8 前端测试 | 0 行 | ~100 行 | 完全缺失（见上） |

合理总预估：~1,450-1,550 行（vs task-breakdown 的 1,189 行）。

---

## 汇总

| # | 级别 | 问题 | 所属 Task |
|---|------|------|-----------|
| 1 | CRITICAL | role 判定来源未明确，可能引入额外 DB 查询或安全问题 | T2 |
| 2 | HIGH | WS 半开连接无 heartbeat 保护，可能产生僵尸命令 | T2 |
| 3 | HIGH | 多连接注册广播风暴，前端缺 debounce | T2, T6 |
| 4 | HIGH | ambiguous 命令的交互状态机未定义 | T5 |
| 5 | HIGH | 前端 CommandRegistry 单元测试完全缺失 | T8 |
| 6 | HIGH | SlashCommandPicker 行数预估偏低（80 → 150-200） | T5 |
| 7 | HIGH | 总行数预估偏低 ~20-30% | 全局 |
| 8 | HIGH | broadcastAll 全局作用域未显式声明 | T2, T3 |
