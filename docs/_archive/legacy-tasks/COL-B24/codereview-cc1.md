# COL-B24 Code Review — CC1

日期：2026-04-23 | Reviewer：Claude Code

仅列出 CRITICAL 和 HIGH 级别问题。

---

## CRITICAL

### C1: requireMention 场景 5 测试严重缩水 — 核心功能未覆盖

**设计文档要求**：场景 5 需要测试 SSE/Poll/WS 三条路径的消息推送过滤 + DM 不受限制（4 个 test case，需要真实 server + `buildFullApp`）。

**实际实现** (`require-mention.integration.test.ts`)：仅测试了 `require_mention` 数据库字段的 CRUD（通过 admin API 读写），完全没有测试消息推送过滤逻辑。没有 SSE 连接、没有 WS 连接、没有 Poll、没有 DM 豁免验证。

**影响**：requireMention 是 agent 消息过滤的核心机制，当前测试只验证了"字段能存进数据库"，未验证"过滤是否真正生效"。这是设计文档 14 个场景中覆盖度最差的一个。

---

### C2: TestContext 定义但从未使用

**设计文档**要求 `TestContext` 作为多用户集成测试的核心基础设施，所有场景共享。

**实际实现**：`TestContext` 类在 `setup.ts` 中定义完整（包含 `create()`、`inject()`、`close()`），但 **零个测试文件使用它**。所有测试文件都手动创建 `testDb`、`app`、本地 `inject()` 函数，逻辑完全重复。

**影响**：
- 约 200 行死代码（TestContext 类 + 相关 import）
- 每个测试文件重复 ~30 行样板代码（vi.mock + Fastify setup + authMiddleware hook）
- 违反了 design.md 和 task-breakdown.md 的设计决策 CC-C1

---

### C3: vi.mock('../ws.js') 与 buildFullApp 冲突 — WS/SSE 测试可能行为不正确

**问题**：需要真实 WS 的测试（plugin-comm、file-link、remote-explorer）同时做了两件矛盾的事：
1. 文件顶层 `vi.mock('../ws.js')` 替换了 broadcastToChannel 等广播函数为 `vi.fn()`
2. `buildFullApp()` 导入并注册了 `registerWsPluginRoutes`、`registerStreamRoutes` 等需要真实 WS 广播的路由

**影响**：所有通过 WS/HTTP 发消息后"验证另一个 WS 客户端收到推送"的测试，实际上广播函数是 mock 的空函数，推送不会真正发生。`plugin-comm.integration.test.ts` 中的 "message event pushed to connected plugin WS" 测试实际上绕过了这个问题（它只验证了 api_response，没有验证 event push），但这掩盖了测试的不完整性。

---

### C4: 并发编辑测试使用了错误的 API 路径

**文件**：`concurrency.integration.test.ts:67`

**问题**：编辑消息使用 `PUT /api/v1/messages/${msgId}`，但 `buildFullApp()` 注册的消息路由来自 `registerMessageRoutes`。需要确认实际路由是否匹配 — 设计文档中使用 `PATCH /api/v1/channels/${channelId}/messages/${msgId}`，而实现中既没有带 channelId 也没有用 PATCH。

**影响**：如果路由不匹配，5 个并发请求全部 404，`successes.length >= 1` 断言可能仍然通过（如果某个 edit 碰巧走了别的路径），或者测试变得无意义。

---

## HIGH

### H1: 场景 6 (Slash Commands) 实际未测试 slash command 执行

**设计文档要求**：测试 `/help`、`/invite @user`、`/leave`、`/topic`、`/dm @user`、无效命令 — 都是通过发送消息（POST messages with content="/help"）触发。

**实际实现** (`slash-commands.integration.test.ts`)：测试的是 REST API 端点（`PUT /channels/:id/topic`、`POST /channels/:id/members`、`POST /channels/:id/leave` 等），不是通过消息系统触发 slash command。

**差异**：设计验证的是"发送 /topic New Topic 消息后 topic 是否更新"，实现验证的是"直接调用 topic API 是否工作"。这两者测试的是不同的代码路径。

---

### H2: 消息系统测试缺少附件场景

**设计文档场景 4** 包含"附件自动存入"测试（发送带附件消息后通过消息 ID 获取附件）。

**实际实现** (`message-system.integration.test.ts`)：没有附件相关测试。

---

### H3: 系统消息测试偏离设计 — sender_id 处理不一致

**设计文档**：系统消息 `sender_id=null`，`type=system`。
**Task breakdown 修正 CC-C2**：系统消息仍需 sender_id（用 agent 用户）。
**实际实现**：`seedMessage(testDb, channelId, agentId, 'User joined', undefined, 'system')`，然后验证 `sysMsg.content_type === 'system'`。

但断言只检查了 `content_type`，没有验证系统消息在 API 响应中是否被正确标记（如 `type` 字段）。且设计文档的字段名是 `type`，实现检查的是 `content_type` — 需要确认哪个是实际 schema。

---

### H4: plugin-comm "message event pushed" 测试名不副实

**文件**：`plugin-comm.integration.test.ts:148`

**测试名**："message event pushed to connected plugin WS"
**实际行为**：发送 api_request 发消息，然后只验证 api_response 的 status=201。完全没有验证是否收到了 event push。

**影响**：测试声称验证了事件推送，但实际只验证了消息发送成功。结合 C3（ws.js 被 mock），真实的事件推送逻辑完全没有被测试覆盖。

---

### H5: 覆盖率阈值提升到 85% 缺少验证

**文件**：`vitest.config.ts:14`

将 statements 阈值从 80 → 85 是在第一个 commit 就完成的（T1.3），但 task-breakdown 明确说"放到最后执行"（修正 CC-H2/C2-H6）。如果新增的集成测试没有实际提升覆盖率到 85%，CI 会因为阈值提升而失败。

---

### H6: remote-explorer 手动创建 remote_nodes 表 — 与 createTestDb schema 不一致

**文件**：`remote-explorer.integration.test.ts:30`

`addRemoteTables()` 手动 `CREATE TABLE remote_nodes/remote_bindings`，说明 `createTestDb()` 的 schema 中不包含这些表。这意味着：
1. 如果 production schema 实际包含这些表，测试的 schema 与 production 不一致
2. 如果 production 也不包含，那么 `registerRemoteRoutes` 引用这些表时可能在 production 也会出问题

需要确认 remote_nodes 表是否应该在 createTestDb 的 schema 中。

---

### H7: 场景 14（消息文件链接）缺少 "Agent 发含文件路径消息 → 存储" 测试

**设计文档**场景 14 的第一个 case："Agent 发含文件路径的消息 → 存储成功"。

**实际实现** (`file-link.integration.test.ts`)：直接测试了文件代理读取（agent online/offline/path_not_allowed），但没有测试通过消息发送文件路径引用的场景。

---

### H8: workspace 10MB 大小限制测试被 skip

**文件**：`workspace-flow.integration.test.ts:141`

`it.skip('10MB size limit → requires real HTTP server to test streaming limit')`

设计文档明确要求验证超限 413 响应。虽然 inject 模式确实可能绕过 multipart 大小限制，但这是可以通过 buildFullApp + listen 解决的。标记为 skip 意味着这个边界条件没有覆盖。

---

### H9: collectEvents (SSE helper) 未实现

**Task breakdown C2-H3** 要求在 ws-helpers.ts 中实现 `collectEvents` 用于 SSE 测试。

**实际实现**：ws-helpers.ts 中有 `collectMessages`（WS 版），但没有 SSE 的 `collectEvents`。这与 C1 相关 — requireMention 的 SSE/Poll 路径测试缺失，所以 SSE helper 也没有实现。

---

## 场景覆盖矩阵

| # | 场景 | 测试文件 | 覆盖度 |
|---|------|---------|--------|
| 1 | 认证流程 | auth-flow.integration.test.ts | 7/7 |
| 2 | 频道生命周期 | channel-lifecycle.integration.test.ts | 9/9 |
| 3 | 权限体系 | permissions.integration.test.ts | 7/7 (status code 微调) |
| 4 | 消息系统 | message-system.integration.test.ts | 7/9 (缺附件、系统消息断言弱) |
| 5 | requireMention | require-mention.integration.test.ts | **1/4** (仅 DB CRUD) |
| 6 | Slash Commands | slash-commands.integration.test.ts | 5/6 (测 API 非 slash) |
| 7 | Workspace | workspace-flow.integration.test.ts | 9/10 (skip 10MB) |
| 8 | Plugin 通信 | plugin-comm.integration.test.ts | 7/9 (event push 假通过) |
| 9 | Remote Explorer | remote-explorer.integration.test.ts | 7/8 (缺 stat) |
| 10 | Plugin OpenClaw Mock | plugin-openclaw-mock.test.ts | 全 todo (符合预期) |
| 11 | 并发安全 | concurrency.integration.test.ts | 2/2 (路径可能错误) |
| 12 | Plugin 部署验证 | plugin-build.test.ts | 2/2 (检查项偏离设计) |
| 13 | Migration | migration.integration.test.ts | 3/3 |
| 14 | 文件链接 | file-link.integration.test.ts | 3/5 (缺消息存储+白名单) |

**总结**：14 个场景全部有对应文件，但场景 5 (requireMention) 覆盖度严重不足，场景 6/8 存在测试与设计意图偏离。
