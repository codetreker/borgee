# COL-B25 Task Breakdown Review — CC1

日期：2026-04-23 | 审阅人：Claude Code

仅列出 CRITICAL / HIGH 级问题。

---

## CRITICAL

### C1. task-breakdown.md 内部 Task 编号体系不一致

文件上半部分使用 T0–T7（8 个 task），下半部分 §4 "Task Breakdown" 表格使用 T0–T5（6 个 task），场景分组方式完全不同：

| 上半部分 | 下半部分 §4 |
|---------|------------|
| T0 基础设施 | 无对应 |
| T1 场景 1–2 | T0 场景 1–4 |
| T2 场景 3–4 | T0b 场景 5–10 |
| T3 场景 5–7 | T1 场景 11–12 |
| … | … |

这会导致实际开发时 commit 粒度和 PR scope 产生歧义。**必须统一为一套编号**。

### C2. 场景 1 Reaction 用例与场景 20 高度重复

- 场景 1（chat-lifecycle）的第 4 个 it：`Reaction 添加 → WS 收到 reaction_added`
- 场景 20（reaction-bidirectional）的第 1 个 it：`A 加 reaction → B WS 收到 reaction_added`

两者测试的是完全相同的路径（POST reaction → WS reaction_added）。场景 1 应删除 Reaction 用例，或场景 20 不再重复该基础 case，仅保留取消 / 多人 / HTTP GET 等差异化用例。

### C3. Task 0 基础设施改动在 §4 表格中被丢弃

上半部分明确定义了 Task 0（扩展 `ws-helpers.ts` 加 headers 支持），但 §4 表格没有 T0 基础设施行。场景 2/4/17 都依赖这个改动——如果遗漏，这三个场景会直接失败。

---

## HIGH

### H1. 三通道一致性（场景 4）技术风险标注不足

设计中 SSE 客户端使用 `require('http').get` + 手工解析 `data:` 行，存在以下未标注风险：
- SSE endpoint 路径 `/api/v1/stream` 是否存在？需确认。目前 B24 `plugin-comm` 测试中 SSE 用的是 Plugin WS 的 `connected` event，并非独立 SSE endpoint。
- SSE 认证方式用了 `Bearer ${agentApiKey}`，但 Poll 用了 `{ api_key: agentApiKey, cursor: 0 }`——两种认证模式混用，payload 结构可能不同，"一致性"断言 `ssePayload.id === msgId` 极有可能失败。
- 超时 8000ms 在 CI 环境可能不够（cold start + event-stream 建立）。

**建议**：将此场景标记为高风险，并在 Task 中增加"先 spike 确认 SSE/Poll endpoint 可用性"步骤。

### H2. 与 B24 `channel-lifecycle.integration.test.ts` 存在重叠

B24 已覆盖：
- 成员加入 + 发消息 + WS broadcast（与场景 1 部分重叠）
- DM 仅双方可见、第三方 403（与场景 8 的 it 3 完全重叠）
- 公开频道 preview 返回最近消息（与场景 10 的 it 1 重叠）
- kick 后失去访问权限（与场景 3 的 it 4 重叠）
- 多频道消息隔离（与场景 12 部分重叠）

task-breakdown 中场景 5/6 标注了与 B24 的去重说明，但场景 1/3/8/10/12 没有。需要逐一确认并加去重说明，否则 code review 时会被打回。

### H3. 并发竞态场景（15/16）的断言过于宽松

- 场景 15 "快速连续操作"：第 1 个 it 用 `Promise.all` 并发发送 5 条然后断言全部入库——但并发发送不保证服务端入库顺序，而第 2 个 it 用串行发送断言按序收到。两个 it 测的是不同东西，第 1 个实际上无法验证"按序"（PRD 要求"按序入库"）。
- 场景 16 "并发成员变更"：只断言 `status !== 500`，没有验证踢出后消息是否被正确拒绝（即 `msgRes.status === 403` 的情况下消息不应入库）。应增加断言：如果 msgRes 是 403，则 DB 中无该消息。

### H4. 场景 18（用户级联删除）缺少"删除用户 API 是否存在"的确认

PRD 列出"删除用户 → agent 停用 → 邀请码作废 → 频道权限清理 → WS 断开"。但 B24 测试中没有任何 `DELETE /api/v1/users/:id` 的调用。如果该 API 尚未实现，场景 18 需要标注为"blocked on API implementation"，否则会浪费开发时间写测试后发现 404。

### H5. 验收标准（§5）遗漏场景 1–10

§5 验收标准从"场景 11-12"开始，完全遗漏了场景 1–10 的验收条目。这可能是因为 §4 表格将场景 1–10 归入了不同的 Task 组，但验收标准应覆盖全部 20 个场景。

---

## 总结

| 级别 | 数量 | 核心问题 |
|------|------|---------|
| CRITICAL | 3 | Task 编号不一致、Reaction 用例重复、T0 基础设施遗漏 |
| HIGH | 5 | 三通道 spike 缺失、B24 去重不完整、竞态断言不足、用户删除 API 未确认、验收标准缺场景 1–10 |
