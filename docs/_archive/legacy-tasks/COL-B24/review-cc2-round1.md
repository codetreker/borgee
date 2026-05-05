# COL-B24 Task Breakdown Review — CC2 Round 1

Reviewer: CC2 | Date: 2026-04-23

---

## CRITICAL

### C1. `waitForMessage` 无超时保护 — 测试会永久挂起

**位置**: design.md §1.4, task-breakdown T1.2

`waitForMessage(ws, filter?)` 没有 timeout 参数。如果预期消息从未到达（回归 bug、race condition），Promise 永不 resolve，vitest 整个 worker 挂死直到 vitest 自身的全局超时（默认 5s）才报错，且报错信息无法定位原因。

`waitForClose(ws)` 同理。

**修改**: 加 `timeoutMs` 参数（默认 5000），超时后 reject 并附带 "Timed out waiting for WS message matching filter" 信息。`waitForClose` 同样处理。

---

### C2. WS 连接泄漏 — 测试失败时 WS 不会被清理

**位置**: 场景 5/8/9/14 所有使用 `connectWS` 的测试

当前模式是 test case 内手动 `ws.close()`。如果 assertion 失败或抛异常，`ws.close()` 不会执行，WS 连接泄漏，可能导致后续 test 端口占用或 `server.close()` 挂起。

**修改**: `connectWS` 返回值应自动注册到一个 cleanup 列表，在 `afterAll`/`afterEach` 中统一关闭。或使用 `try/finally` pattern。建议在 `TestContext` 或独立 helper 中维护 `openConnections: WebSocket[]`，`close()` 时批量清理。

---

### C3. 场景 8 Plugin 通信测试中 `channelId`/`msgId` 未定义

**位置**: design.md §2.8, task-breakdown T6.1

`plugin-comm.test.ts` 的多个 test case 引用 `channelId` 和 `msgId`，但这些变量在 `beforeAll` 或 describe scope 中没有声明/赋值。设计文档的代码示例不完整，task breakdown 也未明确要求创建 channel 和 seed message。

**修改**: task-breakdown T6.1 需明确：`beforeAll` 中通过 HTTP 注册用户、创建 channel、seed 初始 message，或使用 `TestContext` 与真实 server 结合的模式。当前设计中 `buildFullApp()` 和 `TestContext` 是分离的两套，需要明确如何桥接。

---

## HIGH

### H1. 与现有测试大量重复 — 认证/频道/消息/Reaction/Workspace/Slash

**位置**: T2.1, T2.2, T3.1, T3.2, T5.1, T4.2

现有测试已覆盖的场景（不应在集成测试中重复）：

| 计划的集成测试 case | 现有测试已覆盖 |
|---|---|
| 注册（有效/无效/已用邀请码）| `auth.test.ts` 完整覆盖 |
| 登录（正确/错误密码）| `auth.test.ts` 完整覆盖 |
| API Key 认证 | `ws-plugin.test.ts` 覆盖 |
| 过期/无效 token | `auth.test.ts` 覆盖 |
| admin/member 创建频道权限 | `channels.test.ts` 覆盖 |
| member 删自己/他人消息 | `messages.test.ts` 覆盖 |
| admin 删任何消息 | `messages.test.ts` 覆盖 |
| 频道创建（公开/私有）| `channels.test.ts` 覆盖 |
| 加入/踢出频道 | `channels.test.ts` membership 覆盖 |
| 软删除频道 | `admin-agents-dm.test.ts` force-delete 覆盖 |
| 公开频道预览 24h | `preview.test.ts` 覆盖 |
| 消息发送/编辑/删除 | `messages.test.ts` 覆盖 |
| @mention 写入 | 部分覆盖 |
| Reaction 增删+409 | `reactions.test.ts` 完整覆盖 |
| 分页 cursor | `messages.test.ts` 覆盖 |
| 附件 | `messages.test.ts` attachment auto-save 覆盖 |
| Workspace 全流程 | `workspace.test.ts` 几乎完整覆盖 |
| /topic | `slash-commands.test.ts` 覆盖 |
| WS Plugin 连接/认证/apiCall | `ws-plugin.test.ts` 覆盖 |
| Remote Node CRUD/WS/权限 | `remote.test.ts` 完整覆盖 |
| Agent 文件代理 | `agents-files.test.ts` 覆盖 |

**修改**: task-breakdown 需明确每个新测试与现有测试的差异化价值。当前 T2.1 auth-flow.test.ts 中的 7 个 case 与 `auth.test.ts` 几乎 1:1 重叠。建议：

1. **删除** 与现有单测完全重叠的 case（单 API 调用的 happy/error path）
2. **保留** 真正的多用户端到端 case（跨用户隔离、状态流转、多步骤 workflow）
3. 预估的 76 个 test case 中至少 30+ 是重复的，应精简到 ~45 个

---

### H2. `TestContext` 与 `buildFullApp()` 两套体系未统一

**位置**: T1.1, T4.1, T6.1, T6.2, T7.1

需要真实 HTTP server 的场景（WS/SSE）使用 `buildFullApp()` + `server.listen`，但同时也需要 `TestContext` 提供的多用户 seed 数据。当前设计中两者是独立的：

- `TestContext.create()` 创建自己的 Fastify 实例和 DB
- `buildFullApp()` 创建另一个 Fastify 实例和另一个 DB

场景 9 (remote-explorer) 的代码同时使用了两者（`server = buildFullApp()` + `ctx = TestContext.create()`），它们指向不同的 DB 实例，测试逻辑上不成立。

**修改**: `TestContext` 需要支持 `{ fullApp: true, listen: true }` 模式，内部调用 `buildFullApp()` 并 `listen({ port: 0 })`，确保 seed 数据和 server 共享同一个 DB。或者 `buildFullApp(db)` 接受外部 DB。

---

### H3. SSE `collectEvents` 实现不完整

**位置**: T1.2, design.md §1.4

`collectEvents(response, timeoutMs)` 是 task-breakdown 中列出的 helper，但 design.md 没有给出实现。SSE 收集涉及：

1. 读取 `ReadableStream` 并按 `\n\n` 分割
2. 解析 `event:`/`data:`/`id:` 字段
3. 超时后 abort stream 并返回已收集事件
4. 处理 fetch Response 被 `AbortSignal.timeout()` 中断后的 stream 状态

这不是 trivial 的，但 task-breakdown T1.2 只给了 ~60 行预估（含 connectWS/waitForMessage/waitForClose/sleep），`collectEvents` 的复杂度被低估。

**修改**: 单独预估 `collectEvents` 为 ~30-40 行，T1.2 总预估调整到 ~90 行。补充 edge case：空 data 字段、多行 data 字段、非 JSON data。

---

### H4. 并发测试（T9.1）在 SQLite in-memory 下无意义

**位置**: T9.1 concurrency.test.ts

SQLite in-memory 模式是单连接串行执行，`Fastify.inject()` 在同一进程内也是串行化的。`Promise.all` 发 5 个 inject 请求不会产生真正的并发竞争 — 它们会依次执行。

"邀请码并发消费只有 1 成功" 这个 case 在 SQLite 下恒为 true（因为根本没有并发），测试通过不代表生产环境（PostgreSQL/MySQL）安全。

**修改**: 要么标注此测试仅验证逻辑正确性（非真并发），要么使用 `server.listen` + 真实 HTTP 请求（多个 TCP 连接），至少能测到 Fastify 层的并发处理。

---

### H5. 场景 14 (file-link) 与场景 9 (remote-explorer) 的文件代理测试高度重叠

**位置**: T6.2 file-link.test.ts, T7.1 remote-explorer.test.ts

两者都测试：
- Agent/Node WS 转发文件读取
- 非 owner 403
- Agent/Node 离线 503
- 白名单外路径 403

且现有 `agents-files.test.ts` 和 `remote.test.ts` 已经分别覆盖了这些场景。

**修改**: file-link.test.ts 应聚焦于 **消息中嵌入文件路径 → 点击读取** 这个端到端 workflow，而非重复测试底层文件代理。将 owner/403/503/白名单 case 删除（已有覆盖），只保留消息 → 文件链接解析 → 代理读取的完整链路。

---

### H6. 覆盖率阈值 80→85 依据不足

**位置**: T1.3

当前阈值 80%，升到 85% 需要确认增量测试确实能带来 5% 提升。如果大量新测试是重复已覆盖路径（见 H1），实际覆盖率增量可能很小。应在 T9 完成后根据实际覆盖率数据决定阈值，而非预设。

**修改**: T1.3 中不修改阈值，改为 T9 之后新增一个验证步骤：运行覆盖率报告，确认达标后再调整阈值。
