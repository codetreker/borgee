# COL-B24 Review — Round 2

审阅对象：`task-breakdown.md`（修正后）+ `design.md` v4

---

## CRITICAL

### C1 (原 CC-C1): DB 注入 — 部分解决，design.md 未同步

**状态**：⚠️ 未完全解决

**task-breakdown.md** 修正记录声称 TestContext.create() 内部通过 `vi.mock` 替换 `getDb()`（第 305 行），方向正确。

**但 design.md §1.3（第 33 行）仍然写的是**：
```typescript
ctx.app.decorate('testDb', ctx.db);
```

这是原来被判定为 CRITICAL 的错误代码。design.md 作为实现参考文档未被更新，实施者按 design.md 编码会重现 C1 问题。

此外，`vi.mock` 是 hoisted 的静态调用，不能在 `TestContext.create()` 内部动态执行。task-breakdown 说"内部通过 vi.mock"但未说明如何解决 hoisting 限制。可行方案是：
- 每个测试文件顶层 `vi.mock('../db.js', () => ({ getDb: () => testDb }))`
- `TestContext.create()` 内部修改已导出的 `testDb` 引用

**需要**：更新 design.md §1.3 的 TestContext 代码，明确 vi.mock hoisting 方案。

### C2 (原 CC-C2): sender_id NOT NULL — 部分解决，design.md 未同步

**状态**：⚠️ 未完全解决

**task-breakdown.md** 修正记录说"系统消息仍需 sender_id（用 agent 用户），测试方案已调整"（第 306 行）。

**但 design.md §2.4（第 401-408 行）仍然写**：
```typescript
const sysId = seedMessage(ctx.db, channelId, null, 'User joined', Date.now(), 'system');
// ...
expect(sysMsg.sender_id).toBeNull();
```

`sender_id` 传 `null` + 断言 `toBeNull()` 与 schema `NOT NULL` 约束直接冲突。实施者按此代码编写会导致 INSERT 失败。

**需要**：更新 design.md 系统消息测试用例，将 `null` 改为系统用户 ID，断言改为检查 `type === 'system'`。

### C3 (原 CC-C3): buildFullApp() — 部分解决，仍无实现规格

**状态**：⚠️ 未完全解决

**task-breakdown.md** T1.1 第 20 行提到"新增 `buildFullApp()` 函数：注册所有路由，返回完整 Fastify 实例"。

但仍缺少关键细节：
1. **注册哪些路由**？全部还是子集？需要列出具体的 `register*` 函数清单
2. **WS mock 策略**：`broadcastToChannel`/`broadcastToUser` 在 inject 模式下需要 mock（现有所有测试都 mock `../ws.js`）。buildFullApp() 用真实 server 时，WS 广播是否走真实路径？如果是，需要确认 WS upgrade 已正确注册
3. **与 TestContext 的关系**：design.md §2.9（第 829-831 行）同时使用 `buildFullApp()` 和 `TestContext.create()`，但两者创建了不同的 DB 实例，数据不共享

**需要**：在 design.md 或 task-breakdown 中给出 buildFullApp() 的伪代码实现，包含路由列表和 mock 策略。

---

## 新发现的 CRITICAL

### C4 (NEW): design.md 场景 9 中 buildFullApp() 与 TestContext 双 DB 实例

**位置**：design.md §2.9（第 828-831 行）

```typescript
server = await buildFullApp();  // 创建 DB 实例 A
ctx = await TestContext.create(); // 创建 DB 实例 B
```

`buildFullApp()` 内部会初始化自己的 DB（通过 `getDb()`），`TestContext.create()` 创建另一个 in-memory DB。测试通过 `ctx` seed 数据（写入 DB-B），但 HTTP 请求命中 `server`（读 DB-A）。**所有 seed 数据对 server 不可见，所有测试断言都会失败。**

同样的问题出现在 §2.14 file-link.test.ts（第 1239-1241 行）。

**修复**：buildFullApp() 场景下不应同时使用独立的 TestContext。应该让 buildFullApp() 接受外部 DB，或通过 vi.mock 共享同一 DB。

---

## 总结

| 编号 | 来源 | 状态 | 说明 |
|------|------|------|------|
| C1 | Round 1 CC-C1 | ⚠️ 未完全解决 | task-breakdown 修正方向正确，但 design.md 代码未更新，且 vi.mock hoisting 方案未明确 |
| C2 | Round 1 CC-C2 | ⚠️ 未完全解决 | task-breakdown 修正方向正确，但 design.md 代码未更新（仍传 null） |
| C3 | Round 1 CC-C3 | ⚠️ 未完全解决 | task-breakdown 给了一句话描述，缺实现规格 |
| C4 | NEW | 🔴 新发现 | buildFullApp() + TestContext 双 DB 实例导致数据不共享 |

**根本问题**：task-breakdown.md 修正记录表中声称已解决，但 design.md（实际实现参考）中的代码未同步更新。修正仅停留在"声明已修复"层面，未落实到可执行代码。需要更新 design.md 中所有受影响的代码示例。
