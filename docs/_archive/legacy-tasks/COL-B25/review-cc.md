# COL-B25 技术设计评审

日期：2026-04-23 | 评审人：Claude Code

---

## P0 — 必须修复

### 1. 场景 5（Remote Explorer）与 B24 `remote-explorer.integration.test.ts` 高度重复

B24 已有的测试：
- `register Node → 201`
- `WS connect with valid token → open`
- `file proxy read → returns content via WS relay`
- `node offline → 503`
- `non-owner → 403`

B25 场景 5 的 5 个 test case 几乎逐一对应上述 B24 用例。PRD 明确要求"不与现有单测重复"。

**建议**：删除场景 5，或将其缩减为 B24 未覆盖的多步复合场景（如：注册 → 绑定频道 → 列目录 → 读文件 → 断连 → 重连 → 再读，一个连贯流程而非独立 case）。

### 2. 场景 6（Workspace）与 B24 `workspace-flow.integration.test.ts` 部分重复

B24 已覆盖：upload → list (user isolation) → rename。B25 场景 6 的 "上传 → 列表可见 → 下载 → 多用户隔离" 中，上传+列表+隔离已在 B24 中测过。

**建议**：保留下载验证和"消息引用附件"部分（B24 未覆盖），删除与 B24 重叠的基础 upload/list/isolation case。

### 3. `connectWS` 用法与 `ws-helpers.ts` 签名不匹配

设计文档中场景 2 和场景 17 使用：
```ts
connectWS(port, '/ws/plugin', { apiKey: agentApiKey })
```
但 `ws-helpers.ts` 的 `connectWS` 签名是：
```ts
connectWS(port: number, path: string, query?: Record<string, string>)
```
它将 query 拼到 URL query string：`/ws/plugin?apiKey=xxx`。而 Plugin WS 路由实际上可能通过 header 或 path 接收 apiKey（需确认）。如果服务端从 query string 读 `apiKey`，则签名匹配；否则连接会 4001 失败。

**建议**：确认 Plugin WS 路由的认证方式，并在设计文档中明确 `connectWS` 的 query 参数是否就是 apiKey 传递方式。

### 4. PRD 场景 1-10 在设计文档中只有 Task Breakdown 的场景 11-20，缺少场景 1-10 的 Task Breakdown

设计文档 §4 Task Breakdown 从 T1（场景 11-12）开始。场景 1-10 有完整的代码设计（§3.1-3.10），但没有对应的 task breakdown、预估和交付计划。

**建议**：补充场景 1-10 的 task breakdown，或明确说明场景 1-10 属于另一个 task（如 B24 重构后合入）。

---

## P1 — 应该修复

### 5. WS `waitForMessage` 默认超时 5s，部分场景可能不够

`waitForMessage` 默认 `timeoutMs = 5000`。场景 4（SSE/WS/Poll 三通道）中 SSE 手动设了 8s 超时，但 WS 侧用默认 5s。如果 SSE 连接建立慢（`sleep(500)` 之后才发消息），WS 可能在 SSE 就绪前就开始倒计时。

**建议**：场景 4 的 WS `waitForMessage` 也传 8000ms，与 SSE 超时对齐。

### 6. 场景 4 SSE 验证过于粗糙

SSE 验证只做了 `expect(sseData).toContain(msgId)`，是字符串子串匹配。没有 parse SSE event 格式（`data: ...`），无法确认 payload 结构与 WS/Poll 一致。PRD 要求"完全一致的 payload"。

**建议**：解析 SSE `data:` 行为 JSON，与 WS event payload 做结构性比较（至少比较 id、content、sender_id）。

### 7. 场景 4 Poll 验证中 `JSON.parse(e.payload)` 假设 payload 是字符串

Poll 验证：`JSON.parse(e.payload).id === msgId`。如果 poll API 返回的 events 中 payload 已经是 object（非序列化字符串），`JSON.parse` 会抛错。

**建议**：确认 poll API 的 payload 格式，必要时做兼容处理。

### 8. 场景 2 Agent WS 事件格式假设未验证

场景 2 假设 Plugin WS 的事件格式为 `{ type: 'event', kind: 'message' | 'mention', payload: {...} }`。B24 的 `plugin-comm.integration.test.ts` 和 `require-mention.integration.test.ts` 可能揭示了实际格式——需要与实际 ws-plugin 路由的事件格式交叉验证。

**建议**：阅读 `ws-plugin.ts` 源码确认事件结构，更新设计中的 filter 条件。

### 9. 场景 10（公开频道预览）假设了 `/api/v1/channels/:id/preview` 端点

该端点可能不存在。需确认是否已实现。如果未实现，此场景应标注为依赖前置实现，或从 B25 scope 移除。

**建议**：grep 代码库确认 `/preview` 路由是否存在。

### 10. 场景 8 DM 创建 API 路径存疑

设计中用 `POST /api/v1/dm/${memberBId}`，但实际 DM 路由可能是 `/api/v1/dm` + body `{ user_id }` 或其他格式。

**建议**：确认 `dm.ts` 路由定义的实际路径和参数。

### 11. `seedMessage` 在设计中被引用但用法有差异

设计中部分场景用 `seedMessage(testDb, channelId, adminId, 'content', Date.now() + i)`（5 参数），`setup.ts` 中签名是 `seedMessage(db, channelId, senderId, content, createdAt?, type?)`。用法匹配，但场景 7 批量 seed 150 条消息用循环 `seedMessage`——SQLite in-memory 单线程下 150 次同步写入是否会导致测试变慢？

**建议**：考虑用 prepared statement + transaction 批量插入 150 条消息。

---

## P2 — 可以改进

### 12. 场景 3 和场景 4 的踢出后 WS 验证依赖 `collectMessages` 的固定等待

`collectMessages(ws, 1500)` 等待 1.5s 然后检查没有收到消息。这是"absence proof by timeout"——如果 CI 机器慢，可能产生 false positive（消息在 1.5s 后才到达被错过）或 false negative 极少。反之，每个测试固定等 1.5s 会拖慢整体运行时间（20 个场景 × 多个 collectMessages ≈ 较大的累积等待）。

**建议**：文档中注明 collectMessages 的 timeout 选择理由，并考虑在 CI 中适当放大。

### 13. 每个 test case 都新建 WS 连接

场景 1 的 4 个 it 块各自 `connectAuthWS` + `subscribeToChannel`。真正的集成测试可能应该在一个 it 块中串联完整流程（发 → 编辑 → 删除 → reaction），减少连接开销并更接近真实用户行为。

**建议**：P0 场景 1 考虑合并为 1-2 个 it 块的串联流程测试。独立 it 块适合单元测试粒度，不适合集成测试风格。

### 14. afterAll 清理 wsConnections 数组但 afterEach 未清理

如果某个 it 块抛异常，WS 连接已 push 到 `wsConnections` 但可能处于半开状态。afterAll 会尝试 close，但不保证顺序。afterEach 中清理更安全。

**建议**：在标准模板中加 `afterEach` 清理当前 test 新增的 WS 连接。

### 15. 场景 17 `connectWS` reject 的断言方式

```ts
await expect(connectWS(...)).rejects.toThrow();
```
`connectWS` 在 WS `error` 事件时 reject，但 WS close with code 4001 可能先触发 `close` 而非 `error`。实际上 `ws` 库在服务端拒绝 upgrade 时行为取决于 HTTP 状态码——可能是 error（如 401 upgrade fail）也可能是正常 close。

**建议**：改用 `connectWS` + `waitForClose` 并检查 close code，而非依赖 reject。

### 16. 设计文档缺少"如何运行"说明

没有 vitest 配置说明（如 `--pool=forks` 避免 WS 端口冲突、`--test-timeout` 设置）。

**建议**：加一节简要说明运行命令和 vitest 配置要点。

---

## 覆盖率总结

| PRD 场景 | 设计覆盖 | 与 B24 重复 | 备注 |
|----------|---------|-------------|------|
| 1. 完整聊天 + WS 推送 | ✅ 5 cases | ❌ 无重复 | 场景 1 单 API 消息发送在 B24 `channel-lifecycle` 有，但 B25 加了 edit/delete/reaction WS 验证，不算重复 |
| 2. Agent-Human 往返 | ✅ 3 cases | ⚠️ 部分 | B24 `plugin-comm` 和 `require-mention` 覆盖了 WS 连接和 mention 过滤，B25 增加了 apiCall 回复链路 |
| 3. 权限动态变化 | ✅ 4 cases | ❌ | |
| 4. SSE/WS/Poll 三通道 | ✅ 1 case | ❌ | SSE 验证需加强（见 P1 #6） |
| 5. Remote Explorer | ✅ 5 cases | ⛔ **高度重复** | 见 P0 #1 |
| 6. Workspace + 消息 | ✅ 3 cases | ⚠️ 部分重复 | 见 P0 #2 |
| 7. 分页 + 实时 | ✅ 2 cases | ❌ | |
| 8. DM 完整链路 | ✅ 3 cases | ❌ | API 路径需确认 |
| 9. Slash Commands | ✅ 2 cases | ❌ | |
| 10. 公开频道预览 | ✅ 3 cases | ❌ | 需确认 /preview 端点存在 |
| 11. 多设备同一用户 | ✅ 3 cases | ❌ | |
| 12. 频道隔离交叉 | ✅ 4 cases | ❌ | |
| 13. 频道删除级联 | ✅ 3 cases | ❌ | |
| 14. 成员变更系统消息 | ✅ 3 cases | ❌ | |
| 15. 快速连续操作 | ✅ 2 cases | ❌ | |
| 16. 并发成员变更 | ✅ 2 cases | ❌ | |
| 17. Token 轮换 WS | ✅ 3 cases | ❌ | |
| 18. 级联删除完整性 | ✅ 4 cases | ❌ | |
| 19. 并发上传 | ✅ 2 cases | ❌ | |
| 20. Reaction 双向 | ✅ 4 cases | ❌ | 场景 1 已有 reaction_added，B25 场景 20 增加了 removed + 多人 + HTTP 验证，不算重复 |

**20 个场景全覆盖** ✅ — 每个 PRD 场景在设计中都有对应的 test case。

---

## 汇总

| 级别 | 数量 | 关键点 |
|------|------|--------|
| P0 | 4 | 场景 5/6 与 B24 重复；connectWS 签名需确认；场景 1-10 缺 task breakdown |
| P1 | 7 | WS 超时对齐；SSE 验证不够；Poll payload 格式；Agent 事件格式；/preview 和 /dm 路由确认；批量 seed 性能 |
| P2 | 5 | collectMessages 超时、it 粒度、afterEach 清理、connectWS reject 方式、运行说明 |
