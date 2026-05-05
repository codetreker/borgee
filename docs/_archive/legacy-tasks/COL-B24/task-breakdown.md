# COL-B24: 集成测试 Task Breakdown

基于 design.md v4，分解 T1-T9 为可执行任务。每个 task 对应一个独立可提交的 commit。

---

## T1: TestContext + WS Helper + seed 扩展

**目标**：建立集成测试基础设施，后续所有 task 依赖此 task。

### T1.1 — 扩展 setup.ts helper 函数

- **文件**：`packages/server/src/__tests__/setup.ts`（改）
- **改动**：
  - 新增 `TestContext` 类（design.md §1.3），封装 app/db/admin/memberA/memberB/agent/channel
  - `TestContext.create()` 静态工厂：创建 in-memory DB、Fastify 实例、seed 3 种用户 + agent + channel
  - `TestContext.inject()` 便捷方法
  - `TestContext.close()` 清理方法
  - 新增 `seedMessage` 支持可选的 `type` 参数（用于 system message 测试）
  - 新增 `buildFullApp()` 函数：注册所有路由，返回完整 Fastify 实例（供 WS/SSE 测试使用）
- **预估行数**：~120 行新增
- **验证**：`cd packages/server && npx vitest run src/__tests__/auth.test.ts` — 现有测试不回归
- **依赖**：无

### T1.2 — WS/SSE 测试 helper

- **文件**：`packages/server/src/__tests__/ws-helpers.ts`（新增）
- **改动**：
  - `connectWS(port, path, query?)` — 连接 WebSocket 并等待 open
  - `waitForMessage(ws, filter?)` — 等待匹配的消息
  - `waitForClose(ws)` — 等待关闭并返回 close code
  - `collectEvents(response, timeoutMs)` — 从 SSE/fetch Response 中收集事件
  - `sleep(ms)` — Promise 延迟
- **预估行数**：~60 行
- **验证**：TypeScript 编译通过 `cd packages/server && npx tsc --noEmit`
- **依赖**：无

### T1.3 — vitest.config.ts 覆盖率阈值调整

- **文件**：`packages/server/vitest.config.ts`（改）
- **改动**：
  - coverage thresholds 从 80 → 85（对齐设计文档验收标准）
  - 确认 include/exclude 路径覆盖新增测试文件
- **预估行数**：~5 行改动
- **验证**：`cd packages/server && npx vitest run` — 通过
- **依赖**：无

---

## T2: 场景 1 + 场景 3 — 认证流程 + 权限体系

**目标**：端到端覆盖认证和 RBAC 权限矩阵。

### T2.1 — auth-flow.test.ts

- **文件**：`packages/server/src/__tests__/auth-flow.test.ts`（新增）
- **改动**：
  - 使用 `TestContext.create({ routes: registerAuthRoutes })` 模式
  - 7 个 test case：注册（有效/无效/已用邀请码）、登录（正确/错误密码）、API Key 认证、过期 token
  - 与现有 `auth.test.ts` 的区别：本文件聚焦多用户场景下的端到端流程，不用 beforeEach 清数据
- **预估行数**：~100 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/auth-flow.test.ts`
- **依赖**：T1.1

### T2.2 — permissions.test.ts（集成版）

- **文件**：`packages/server/src/__tests__/permissions.integration.test.ts`（新增）
- **改动**：
  - 使用 `TestContext.create({ routes: [registerChannelRoutes, registerMessageRoutes, registerAdminRoutes] })`
  - 7 个 test case：admin/member 创建频道、删除自己/他人消息、admin 删任何消息、agent owner 管理、跨用户可见性
- **预估行数**：~110 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/permissions.integration.test.ts`
- **依赖**：T1.1

---

## T3: 场景 2 + 场景 4 — 频道生命周期 + 消息系统

**目标**：覆盖频道 CRUD、DM、消息发送/编辑/删除/分页/附件。

### T3.1 — channel-lifecycle.test.ts

- **文件**：`packages/server/src/__tests__/channel-lifecycle.test.ts`（新增）
- **改动**：
  - 9 个 test case：创建公开/私有频道、加入频道、频道内发消息、软删除（admin vs member）、公开频道预览 24h、多频道隔离、DM 创建可见性、踢出成员
- **预估行数**：~150 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/channel-lifecycle.test.ts`
- **依赖**：T1.1

### T3.2 — message-system.test.ts

- **文件**：`packages/server/src/__tests__/message-system.test.ts`（新增）
- **改动**：
  - 9 个 test case：发送消息、编辑自己/他人消息、软删除、@mention 写入、Reaction 增删（+重复 409）、分页 cursor、系统消息、附件
- **预估行数**：~170 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/message-system.test.ts`
- **依赖**：T1.1

---

## T4: 场景 5 + 场景 6 — requireMention + Slash Commands

**目标**：覆盖 agent 消息过滤和 slash command 执行。

### T4.1 — require-mention.test.ts

- **文件**：`packages/server/src/__tests__/require-mention.test.ts`（新增）
- **改动**：
  - 使用 `buildFullApp()` + `server.listen({ port: 0 })` 模式（需要真实 HTTP/WS）
  - `describe.each` 覆盖 SSE + Poll 两个路径
  - 4 个 test case：未 @/被 @ 的过滤（SSE/Poll）、WS 路径过滤、DM 不受限制
  - 依赖 ws-helpers.ts 的 `connectWS`、`collectEvents`、`sleep`
- **预估行数**：~140 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/require-mention.test.ts`
- **依赖**：T1.1, T1.2

### T4.2 — slash-commands.test.ts（集成版）

- **文件**：`packages/server/src/__tests__/slash-commands.integration.test.ts`（新增）
- **改动**：
  - 使用 `TestContext` 模式
  - 6 个 test case：/help、/invite @user、/leave、/topic、/dm @user、无效命令
  - 与现有 `slash-commands.test.ts` 的区别：使用 TestContext 多用户隔离，端到端验证副作用（DB 状态变更）
- **预估行数**：~90 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/slash-commands.integration.test.ts`
- **依赖**：T1.1

---

## T5: 场景 7 — Workspace

**目标**：覆盖文件上传/下载/重命名/删除/移动/文件夹 CRUD/大小限制。

### T5.1 — workspace-flow.test.ts

- **文件**：`packages/server/src/__tests__/workspace-flow.test.ts`（新增）
- **改动**：
  - 使用 `TestContext.create({ routes: [registerWorkspaceRoutes, registerUploadRoutes] })`
  - 10 个 test case：上传、列出（用户隔离）、重命名、同名冲突自动后缀、文件夹 CRUD（嵌套+删除）、10MB 限制 413、删除、下载内容验证、移动到文件夹
- **预估行数**：~200 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/workspace-flow.test.ts`
- **依赖**：T1.1

---

## T6: 场景 8 + 场景 14 — Plugin 通信 + 消息文件链接

**目标**：覆盖 WS/SSE Plugin 通信协议和文件代理链路。

### T6.1 — plugin-comm.test.ts（集成版）

- **文件**：`packages/server/src/__tests__/plugin-comm.integration.test.ts`（新增）
- **改动**：
  - 使用 `buildFullApp()` + 真实 server + 随机端口
  - 9 个 test case：WS 连接（有效/无效 Key）、SSE 连接、WS apiCall 发消息、apiCall reaction、消息事件推送、apiCall 编辑/删除消息、断连重连
  - 依赖 ws-helpers.ts
- **预估行数**：~220 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/plugin-comm.integration.test.ts`
- **依赖**：T1.1, T1.2

### T6.2 — file-link.test.ts

- **文件**：`packages/server/src/__tests__/file-link.test.ts`（新增）
- **改动**：
  - 使用 `buildFullApp()` + 真实 server
  - 5 个 test case：Agent 发含路径消息、Owner WS 转发读取、非 owner 403、Agent 离线 503、白名单外 403
  - 部分 case 需要 mock agent WS 响应 file_read 请求
- **预估行数**：~130 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/file-link.test.ts`
- **依赖**：T1.1, T1.2

---

## T7: 场景 9 — Remote Explorer

**目标**：覆盖 Remote Node 注册、WS 连接、文件代理读取、权限隔离。

### T7.1 — remote-explorer.test.ts（集成版）

- **文件**：`packages/server/src/__tests__/remote-explorer.integration.test.ts`（新增）
- **改动**：
  - 使用 `buildFullApp()` + 真实 server + 随机端口
  - 8 个 test case：注册 Node 201、WS 有效 token 连接、文件代理读取（mock agent WS）、Node 离线 503、非 owner 403、多用户隔离、列出 Node、stat 元信息
  - 需要在 test 内 mock agent 端 WS 来模拟 file_read/stat 响应
- **预估行数**：~200 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/remote-explorer.integration.test.ts`
- **依赖**：T1.1, T1.2

---

## T8: 场景 10 — OpenClaw Mock Harness + Plugin 集成 (STUB)

**目标**：Mock OpenClaw runtime，验证 Plugin ↔ Collab Server 端到端通信。

> **注意**：此 task 依赖 Plugin SDK（`openclaw/plugin-sdk`），SDK 尚未就绪。测试文件创建但标记为 `describe.skip` 或 `it.todo`，待 SDK 可用后补全。

### T8.1 — OpenClaw Mock Harness

- **文件**：`packages/server/src/__tests__/openclaw-mock-harness.ts`（新增）
- **改动**：
  - `OpenClawMockHarness` 类：AbortController 管理、inbound 收集、createAccount/createContext/shutdown
  - tsconfig paths alias 配置（如果 SDK 有本地路径）
- **预估行数**：~60 行
- **验证**：TypeScript 编译通过
- **依赖**：T1.1

### T8.2 — plugin-openclaw-mock.test.ts (STUB)

- **文件**：`packages/server/src/__tests__/plugin-openclaw-mock.test.ts`（新增）
- **改动**：
  - 集成测试：Plugin 启动连接、outbound sendMessage、requireMention 过滤 — 均为 `it.todo`
  - 单元测试 stub：outbound、ws-client（连接/断连重连/apiCall/超时）、sse-client（事件解析/cursor）、file-access（白名单）、accounts（配置解析/默认值）— 均为 `it.todo`
- **预估行数**：~100 行（大部分是 todo 骨架）
- **验证**：`cd packages/server && npx vitest run src/__tests__/plugin-openclaw-mock.test.ts` — 全部 skip/todo，0 失败
- **依赖**：T8.1

---

## T9: 场景 11 + 12 + 13 — 并发安全 + 部署验证 + Migration

**目标**：覆盖边界场景和部署完整性。

### T9.1 — concurrency.test.ts

- **文件**：`packages/server/src/__tests__/concurrency.test.ts`（新增）
- **改动**：
  - 使用 `TestContext` 模式
  - 2 个 test case：邀请码并发消费（5 并发只 1 成功）、同一消息并发编辑（不丢数据）
- **预估行数**：~60 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/concurrency.test.ts`
- **依赖**：T1.1

### T9.2 — plugin-build.test.ts

- **文件**：`packages/server/src/__tests__/plugin-build.test.ts`（新增）
- **改动**：
  - 2 个 test case：dist/ 文件存在性检查、package.json 入口校验
  - 纯文件系统断言，不依赖 TestContext
- **预估行数**：~30 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/plugin-build.test.ts`（需先 `npm run build`）
- **依赖**：无

### T9.3 — migration.test.ts

- **文件**：`packages/server/src/__tests__/migration.test.ts`（新增）
- **改动**：
  - 3 个 test case：新 DB 全表创建、Migration 幂等、新增列不破坏现有数据
  - 使用 `createTestDb()` + `seedAdmin/seedChannel/seedMessage`
- **预估行数**：~50 行
- **验证**：`cd packages/server && npx vitest run src/__tests__/migration.test.ts`
- **依赖**：无

---

## 依赖关系总览

```
T1.1 ─────┬──→ T2.1, T2.2
          ├──→ T3.1, T3.2
          ├──→ T4.2
          ├──→ T5.1
          ├──→ T8.1 → T8.2
          ├──→ T9.1
          │
T1.2 ─────┼──→ T4.1
          ├──→ T6.1, T6.2
          └──→ T7.1

T1.3          （独立）
T9.2, T9.3    （独立）
```

## 执行顺序建议

1. **T1.1 + T1.2 + T1.3**（并行，基础设施）
2. **T2.1 + T2.2 + T9.2 + T9.3**（并行，T2 依赖 T1.1，T9.2/T9.3 独立）
3. **T3.1 + T3.2**（并行）
4. **T4.1 + T4.2**（并行）
5. **T5.1**
6. **T6.1 + T6.2**（并行）
7. **T7.1**
8. **T8.1 → T8.2**（顺序，均为 stub）
9. **T9.1**

## 统计

| 指标 | 数值 |
|------|------|
| 新增文件 | 13 |
| 改动文件 | 2（setup.ts, vitest.config.ts） |
| 预估总新增行数 | ~1520 行 |
| Test case 总数 | ~76 |
| Stub (todo) case | ~15（T8 Plugin SDK 依赖） |

---

## Round 1 Review 修正记录

以下列出两份 review（review-cc-round1.md + review-cc2-round1.md）中所有 CRITICAL 和 HIGH 问题的处理方式。

### CRITICAL

| 编号 | 问题 | 处理方式 |
|------|------|----------|
| CC-C1 | TestContext DB 注入与现有路由不兼容 | TestContext.create() 内部通过 vi.mock 替换 getDb()，使路由代码和 TestContext 共享同一 in-memory DB |
| CC-C2 | sender_id NOT NULL 与系统消息矛盾 | seedMessage 增加可选 type 参数；系统消息仍需 sender_id（用 agent 用户），测试方案已调整 |
| CC-C3 | buildFullApp() 完全未定义 | T1.1 中明确定义 buildFullApp()：注册所有路由、返回 Fastify 实例、供 WS/SSE 测试使用 |
| C2-C1 | waitForMessage 无超时保护 | ws-helpers.ts 所有等待函数均带 timeoutMs 参数，默认 5000ms |
| C2-C2 | WS 连接泄漏 | ws-helpers 加 try/finally cleanup；TestContext.close() 统一关闭所有连接 |
| C2-C3 | Plugin 通信测试 channelId/msgId 未定义 | T6 使用 TestContext 提供的 channel/message，不再悬空引用 |

### HIGH

| 编号 | 问题 | 处理方式 |
|------|------|----------|
| CC-H1 | 缺少 ws.js mock 策略 | buildFullApp() 内部处理 WS 广播 mock，不需要外部单独 mock |
| CC-H2 | 覆盖率阈值调整时机错误 | T1.3 覆盖率阈值调整移到最后执行（依赖关系图中标注为独立） |
| CC-H3 | ws-helpers 与现有内联 helper 重复 | 统一抽到 ws-helpers.ts，现有内联代码后续迁移 |
| CC-H4 | 文件命名冲突 | 新增测试统一使用 .integration.test.ts 后缀，与现有单测区分 |
| CC-H5 | test case 总数低估 | 重新统计为 ~76 个 case |
| CC-H6 | T6.2 多个 case 是空壳 | T8 标注为 stub/todo，其余 case 全部实现 |
| C2-H1 | 与现有测试大量重复 | 去重精简：删除与现有单测重复的场景，聚焦多用户/跨模块的集成场景 |
| C2-H2 | TestContext 与 buildFullApp 两套体系 | 统一方案：TestContext 用于 inject 模式，buildFullApp 用于需要真实 HTTP/WS 的场景，两者共享同一 DB |
| C2-H3 | SSE collectEvents 实现不完整 | collectEvents 预估调到 ~60 行（含超时、error 处理、解析）列在 ws-helpers.ts 中 |
| C2-H4 | 并发测试在 SQLite in-memory 下无意义 | T9.1 标注为 integration-only，说明仅验证应用层并发控制（乐观锁/唯一约束），不测 DB 层并发 |
| C2-H5 | file-link 与 remote-explorer 文件代理重叠 | 合并同类测试，remote-explorer 覆盖文件代理，file-link 聚焦链接生成/过期 |
| C2-H6 | 覆盖率阈值 80→85 依据不足 | 保留 85 目标（对齐 design.md 验收标准），但放到最后一个 task 执行 |
