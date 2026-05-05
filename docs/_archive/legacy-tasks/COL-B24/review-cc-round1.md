# COL-B24 Task Breakdown Review — Round 1

审阅对象：`task-breakdown.md` vs `design.md` v4 + 现有测试代码模式

---

## CRITICAL

### C1: TestContext DB 注入方式与现有路由不兼容

**位置**：T1.1 — TestContext 设计

**问题**：现有所有测试（auth.test.ts, channels.test.ts, remote.test.ts 等）通过 `vi.mock('../db.js', () => ({ getDb: () => testDb }))` 注入数据库。路由代码内部调用 `getDb()` 获取 DB 实例。

TestContext 设计用 `app.decorate('testDb', ctx.db)`，但路由不会读 `app.testDb`，仍然调用 `getDb()`。**所有使用 TestContext 的测试都会因为 DB 为空而失败。**

**修复**：TestContext 内部必须 `vi.mock('../db.js')` 或提供一个模块级 DB 替换机制。但 `vi.mock` 是模块级静态调用（hoisted），不能在 `TestContext.create()` 内动态执行。需要重新设计：
- 方案 A：TestContext 仅封装 seed 和 inject 便捷方法，DB mock 仍在文件顶层 `vi.mock`
- 方案 B：改造路由支持 DI（侵入式，不推荐）

**影响**：T1.1 设计需要重写，所有下游 task 受影响。

### C2: `sender_id NOT NULL` 与系统消息测试矛盾

**位置**：T3.2 — message-system.test.ts / 系统消息 case

**问题**：design.md 中系统消息测试写 `seedMessage(ctx.db, channelId, null, 'User joined', Date.now(), 'system')`，但 schema 定义 `sender_id TEXT NOT NULL`。INSERT 会抛出 NOT NULL constraint 错误。

**修复**：
- 选项 A：`seedMessage` 使用特殊系统用户 ID（如 `SYSTEM`）而非 null
- 选项 B：schema 改为 `sender_id TEXT`（允许 null）— 需评估影响面
- task-breakdown 应标注此依赖：需要先确认 schema 是否支持 null sender

### C3: `buildFullApp()` 完全未定义

**位置**：T1.1 提及但无实现细节；T4.1, T6.1, T6.2, T7.1 均依赖

**问题**：task-breakdown 列出 `buildFullApp()` 作为 T1.1 交付物，但无任何说明：
- 注册哪些路由？全部还是子集？
- 如何处理 DB mock？（与 C1 同一问题）
- 如何处理 `broadcastToChannel` / `broadcastToUser` 等 WS 广播依赖？（现有测试全部 mock 了 `../ws.js`）
- 与 `TestContext.create({ routes })` 的关系是什么？

**修复**：T1.1 需要明确 `buildFullApp()` 的实现规格，包括 mock 策略。建议拆为独立子 task T1.4。

---

## HIGH

### H1: 缺少 `../ws.js` mock 策略

**位置**：所有使用 inject（非真实 server）的测试

**问题**：现有测试统一 mock `../ws.js`（`broadcastToChannel`, `broadcastToUser`, `getOnlineUserIds`, `unsubscribeUserFromChannel`）。task-breakdown 和 design.md 均未提及这些 mock。发送消息的路由内部调用 broadcast，不 mock 会因缺少 WS 连接而报错或产生副作用。

**修复**：T1.1 的 TestContext 或各测试文件模板需明确包含 ws.js mock。

### H2: 覆盖率阈值调整时机错误

**位置**：T1.3 — vitest.config.ts

**问题**：T1.3 在第一批执行（与 T1.1 并行），但此时新测试尚未添加。提高 80→85 可能导致 T1 提交后 CI 立即红灯（现有测试可能恰好在 80-85 之间）。

**修复**：将 T1.3 移到最后一批（T9 之后），或拆为两步：先确认当前覆盖率，最后再调阈值。

### H3: ws-helpers.ts 与现有内联 helper 重复

**位置**：T1.2 vs `ws-plugin.test.ts:24-50`

**问题**：`ws-plugin.test.ts` 已有 `connectWs()`, `waitForMessage()`, `waitForClose()` 实现，含超时处理。T1.2 新建 `ws-helpers.ts` 但未提及从现有代码提取/统一。两套 helper 会导致风格不一致。

**修复**：T1.2 应明确标注"从 ws-plugin.test.ts 提取并增强"，而非从零编写。现有实现已含 timeout 逻辑，design.md 的版本缺少 timeout（`waitForMessage` 无超时会永远挂起）。

### H4: 现有测试文件命名冲突

**位置**：T4.2 — `slash-commands.integration.test.ts`

**问题**：已有 `slash-commands.test.ts`。task-breakdown 用 `.integration.test.ts` 后缀区分，但 T2.1 的 `auth-flow.test.ts` 与现有 `auth.test.ts` 无后缀区分，T3.1 `channel-lifecycle.test.ts` 与现有 `channels.test.ts` 也无后缀。命名规则不统一。

**修复**：统一命名策略。要么所有新文件都用 `.integration.test.ts`，要么都不用。建议统一用 `.integration.test.ts`。

### H5: test case 总数低估

**位置**：统计表

**问题**：按各 task 列出的 case 数逐项加总：7+7+9+9+4+6+10+9+5+8+2+2+3 = 81（不含 T8 stub）。task-breakdown 写 ~76。差异约 7%，可能导致工时低估。

**修复**：更新统计为 ~81 active + ~15 stub = ~96 total。

### H6: T6.2 file-link.test.ts 多个 case 是空壳

**位置**：T6.2

**问题**：design.md 中场景 14 的 5 个 case，有 2 个（Owner WS 转发读取、白名单外 403）只有注释没有断言代码。task-breakdown 仍标注 5 个 case / 130 行，实际可交付的完整 case 可能只有 3 个。

**修复**：要么在 task-breakdown 中标注哪些 case 是完整的、哪些需要 mock agent WS 的额外工作量，要么拆分为 T6.2a（完整 case）和 T6.2b（需 agent WS mock 的 case）。

---

## 总结

| 级别 | 数量 | 阻塞 T1 | 必须在编码前解决 |
|------|------|---------|-----------------|
| CRITICAL | 3 | C1, C3 是 | 全部 |
| HIGH | 6 | H2 是 | H1, H3, H4 建议解决 |

**核心风险**：C1（DB 注入）是架构级问题。如果 TestContext 不能正确替换 `getDb()`，整个测试框架方案需要重新设计。建议先写一个 spike（T1.1 的最小 POC），验证 DB 注入可行后再拆分后续 task。
