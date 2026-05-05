# COL-B25 Review — CC2 独立评审

日期：2026-04-23 | 评审对象：task-breakdown.md vs design.md

---

## CRITICAL-1: `connectWS` 不支持 headers，Task 0 修复量被严重低估

**现状**：`ws-helpers.ts:2-20` 中 `connectWS(port, path, query?)` 第三个参数是 `Record<string, string>`，只拼 query string。design.md 场景 2/4/17 全部使用 `connectWS(port, '/ws/plugin', { headers: { authorization: ... } })`，这个 API 不存在。

**影响**：Task 0 说"~8 行"改动。实际需要重构 `connectWS` 签名（第三参数从 `query` 改为 `options: { query?, headers? }`），同时保持 B24 现有调用者兼容。这不是 8 行能搞定的事——需要改签名、改所有现有调用点、加 overload 或 union type。

**建议**：Task 0 预估调整为 30-50 行，包含 B24 回归验证。

---

## CRITICAL-2: `createTestDb()` 缺少 remote_nodes 表

**现状**：`setup.ts` 的 `createTestDb()` 没有建 `remote_nodes` 表（及相关表如 `remote_sessions`）。场景 5 Remote Explorer 通过 HTTP API `POST /api/v1/remote/nodes` 注册节点，route handler 必然会 INSERT 到这个表。

**影响**：场景 5 会直接 500（table not found）。

**建议**：Task 0 必须将 remote 相关表加入 `createTestDb()`，或场景 5 单独处理。这会增加 T0 工作量。

---

## HIGH-1: 场景 4 三通道一致性——复杂度被显著低估

**问题清单**：

1. **ESM 兼容性**：design.md 代码用 `require('http').get`，但项目是 ESM（`import` 语法 + `.js` 后缀）。`require` 在 ESM 上下文中不可用，需改为 `import('node:http')` 或 `import http from 'node:http'`。
2. **SSE 解析无 helper**：手动 buffer 拼接 + `split('\n')` + `data:` 行解析，容易因 chunk 边界切割出 bug。B24 基础设施没有任何 SSE 相关 helper。
3. **SSE 认证方式不明**：代码用 `Authorization: Bearer ${agentApiKey}` 连 SSE，但 `httpJson` 全部用 cookie 认证。SSE 端点是否接受 Bearer token 未经 B24 验证。
4. **三端超时对齐**：WS 默认 5s，SSE 硬编码 8s，Poll 用 `timeout_ms: 2000`。超时不对齐会导致断言在不同负载下不确定性失败。
5. **100 行预估**：包含 WS setup + SSE 原始 HTTP 客户端 + SSE 解析 + Poll 调用 + 三端 payload 结构对比，实际需要 150-180 行。

**建议**：
- 在 T0 中新增 `connectSSE` helper（~30 行），封装 ESM-compatible HTTP streaming + 事件解析
- 场景 4 预估调整为 150+ 行
- 确认 SSE `/api/v1/stream` 端点的认证方式

---

## HIGH-2: `collectMessages` 基于 sleep 的负面断言天然 flaky

**现状**：`ws-helpers.ts:91-103` 的 `collectMessages` 实现是 `sleep(timeoutMs)` 后返回收集到的消息。

**影响**：至少 8 个场景（1、3、8、11、12 等）依赖"收集 N 毫秒内的消息，断言某消息不在其中"来验证隔离性。这种模式有两个问题：
- timeout 太短（如 1000-1500ms）→ 消息还在路上就断言了 → false positive
- timeout 足够长 → 20 个测试文件累计等待时间爆炸（保守估计 30+ 秒纯 sleep）

**建议**：引入"先发一条 sentinel 消息，等到 sentinel 到达后再断言目标消息不存在"模式（proof-of-delivery），替代盲等。

---

## HIGH-3: task-breakdown.md 与 design.md 的 Task 编号体系完全不同

**task-breakdown.md**：T0(infra) → T1(场景1-2) → T2(场景3-4) → ... → T7(场景17-20)，共 8 个 task。

**design.md 第 4 节**：T0(场景1-4) → T0b(场景5-10) → T1(场景11-12) → ... → T5(场景19-20)，共 6 个 task group。

两份文档对同一场景的 Task 归属、预估时间、分组逻辑完全不同。执行时会产生歧义。

**建议**：统一为一套编号，design.md 第 4 节作为权威来源（它有更详细的时间预估），task-breakdown.md 对齐。

---

## HIGH-4: 每测试文件一个 `buildFullApp()` + `listen({port:0})`——20 个 server 实例的资源压力

**现状**：20 个测试文件，每个 `beforeAll` 都 `buildFullApp()` + `app.listen({port:0})`。`buildFullApp` 动态 import 14 个 route 模块 + 注册 WebSocket + Fastify 实例化。

**风险**：
- Vitest 默认并行跑测试文件。20 个并发 Fastify 实例 + 各自的 WS 连接 → 端口耗尽和内存压力
- 每个 `buildFullApp` 都做 14 次动态 `import()`，ESM 模块缓存在 `vi.mock` 下行为不确定
- `vi.mock('../db.js')` 是文件级别的，并行文件间的 mock 可能互相干扰

**建议**：
- 显式设置 `vitest.config` 中 `pool: 'forks'` 或 `singleThread: true` 确保文件间隔离
- 在 CI 中限制并发：`--maxWorkers=4`
- task-breakdown 中应明确提到并行策略

---

## HIGH-5: 场景 5 Remote Explorer 缺少 workspace_storage 目录和文件系统依赖

**现状**：场景 6 的 workspace upload/download 和场景 19 的并发上传需要文件系统写入（`workspace_files` 表 + 实际文件存储）。`createTestDb` 只建了表，没有设置 `WORKSPACE_DIR` 或临时目录。

**影响**：upload 路由可能写磁盘失败或写到非临时目录。并发测试场景 19 尤其危险——同名文件写到同一路径可能文件系统级冲突。

**建议**：T0 需要增加 `tmp` 目录 setup/teardown，或确认 workspace 路由在测试模式下使用内存存储。

---

## 汇总

| ID | 级别 | 影响范围 | 修复成本 |
|----|------|---------|---------|
| C-1 | CRITICAL | 场景 2/4/17 全部无法运行 | T0 +20 行 |
| C-2 | CRITICAL | 场景 5 直接 500 | T0 +30 行 DDL |
| H-1 | HIGH | 场景 4 实现会卡住 | T0 +30 行 helper + 场景 4 +50 行 |
| H-2 | HIGH | 8+ 场景 flaky | 全局模式修改 |
| H-3 | HIGH | 执行混乱 | 文档对齐 |
| H-4 | HIGH | CI 不稳定 | 配置 + 文档 |
| H-5 | HIGH | 场景 6/19 文件操作失败 | T0 +15 行 |

**总体评估**：Task 0（基础设施扩展）被严重低估。当前预估 8 行，实际需要 80-120 行，涵盖 `connectWS` 重构、`createTestDb` 补表、SSE helper、workspace 临时目录。建议将 T0 拆为 T0a（WS/SSE helpers）和 T0b（DB schema + workspace setup），分别验证后再启动场景实现。
