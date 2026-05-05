# COL-B25 Code Review — CC1

日期：2026-04-23 | 审查人：Claude Code

---

## CRITICAL

### C1: 场景 18（用户级联删除）完全未实现

`user-delete-cascade-e2e.test.ts` 仅 15 行，只有一个 `it.skip`。设计文档要求 4 个用例（WS 断开、agent 不可用、邀请码作废、频道成员清除），全部缺失。这是 20 个场景中唯一没有实际测试的。

**建议**：如果 DELETE /users/:id API 不存在，应在 design.md 中标注为 blocked，而非占位。当前状态会给人已覆盖的错觉。

### C2: 场景 3 缺少设计文档中的 "非成员 HTTP 访问私有频道 → 403/404" 用例

设计文档 §3.3 定义了 4 个用例，实际 `permission-ws-e2e.test.ts` 只有 3 个。缺失的是 HTTP 层面的权限校验（`GET /channels/:id/messages` 返回 403/404）。当前只测了 WS 隔离，没有测 HTTP 隔离。

### C3: 场景 14（成员变更系统消息）断言基本无效

`member-change-sysmsg-e2e.test.ts:54-73` — "add member" 测试的核心断言是：
```typescript
if (relevant.length > 0) {
  expect(relevant[0]).toBeDefined();
}
```
这是一个空断言：如果服务器不发系统消息，测试照样通过。设计文档明确要求 "WS 收到 system 类型消息"，但实现允许 0 条消息也算通过。第三个用例 "HTTP 历史接口查到系统消息" 也被替换为仅检查成员列表，完全偏离设计意图。

### C4: 场景 17（Token 轮换）缺少 "旧 WS 连接收到关闭码 4001" 核心用例

设计文档要求 rotate-key 后旧 WS 收到 close code 4001。实际实现的第一个测试（`token-rotation-ws-e2e.test.ts:52`）只验证了新 key 不同于旧 key，没有验证旧 WS 被断开。第三个测试尝试验证旧 key 被拒，但用 catch-all 吞掉异常 + setTimeout fallback，断言 `code !== -1` 意味着连接成功但被关闭也算通过——未验证具体关闭码。

---

## HIGH

### H1: 场景 8（DM）第三方隔离测试改为检查 DM 列表而非消息访问

设计文档要求 `GET /channels/:dmId/messages` 对第三方返回 403/404。实际实现 (`dm-lifecycle-e2e.test.ts:69`) 改为检查 `GET /api/v1/dm` 列表不包含该 DM。这是一个更弱的断言——列表不包含不等于无法访问消息。

### H2: 场景 20（Reaction 双向）WS 事件类型与设计文档不一致

设计文档定义 `reaction_added` / `reaction_removed` 两种事件类型。实际实现统一使用 `reaction_update`。如果这是有意的服务端设计变更，应更新 design.md。如果是实现偏差，需要修正测试以匹配真实事件类型。场景 1 的 reaction 测试也用了 `reaction_update`，需确认这是实际服务端行为。

### H3: 场景 1（聊天生命周期）WS payload 结构与设计文档不一致

设计文档中 WS 事件结构为 `event.payload.content`，实际代码使用 `event.message.content`。这意味着设计文档和代码至少有一方是错的。需要统一。同样的偏差出现在场景 3、8、9 等多个文件中。

### H4: collectMessages 负面断言有 flaky 风险

多个场景用 `collectMessages(ws, 500-1500)` 配合 `expect(...).toHaveLength(0)` 来断言"不应收到消息"。短超时（500ms）在 CI 高负载下可能导致误过（消息延迟到达），但更严重的是在本地可能因太快而漏收消息。

受影响文件：`permission-ws-e2e.test.ts`（500ms）、`chat-lifecycle-e2e.test.ts`（800ms）、`channel-isolation-e2e.test.ts`（1500ms + 额外 sleep）。

### H5: 场景 15（rapid-fire）第一个测试断言顺序性但 Promise.all 不保证顺序

`rapid-fire-e2e.test.ts:50-62` 用 `Promise.all` 并发发 5 条消息，然后检查全部入库。设计文档说 "全部入库且按序"，但并发发送本身就不保证到达顺序。第二个测试用顺序发送来测序，但第一个测试的 "按序" 断言实际上被静默移除了（只检查 count=5），这与设计文档不一致。

### H6: 每个测试创建新 WS 连接但只在 afterAll 清理

多数测试文件中，每个 `it()` 块都创建新的 WS 连接并 push 到 `wsConnections`，但只在 `afterAll` 统一关闭。这意味着：
- 场景 1 有 5 个 test，会累积 5+ 个 WS 连接直到 suite 结束
- 如果中间某个 test 失败，后续 test 可能受到前面遗留连接的干扰（收到非预期消息）

### H7: 场景 4（三通道一致性）SSE 实现依赖 collectSSEEvents helper

`three-channel-consistency-e2e.test.ts` 使用了 `collectSSEEvents` helper，但设计文档中用的是原生 `http.get` + 手动解析。如果 helper 实现有 bug（如吞异常、提前关连接），测试可能误过。需要确认 helper 的实现与设计文档意图一致。

### H8: WS 连接在 test 间未隔离可能导致串扰

场景 20 `reaction-bidirectional-e2e.test.ts:85-109` 直接用 `ws.on('message', handler)` 手动收集消息，配合 `sleep(2000)` 等待。如果前一个 test 的 WS 连接还活着且订阅了同一频道，可能收到非预期的 reaction_update 事件。

---

## 覆盖率总结

| # | 场景 | 状态 | 备注 |
|---|------|------|------|
| 1 | 完整聊天 + WS | ✅ 5/5 | payload 结构与 design.md 不一致 (H3) |
| 2 | Agent-Human | ✅ 3/3 | |
| 3 | 权限隔离 + WS | ⚠️ 3/4 | 缺 HTTP 403 用例 (C2) |
| 4 | 三通道一致性 | ✅ 1/1 | |
| 5 | Remote Explorer | ✅ 1/1 | |
| 6 | Workspace 附件 | ✅ 1/1 | |
| 7 | 分页 + 实时 | ✅ 2/2 | |
| 8 | DM 链路 | ⚠️ 3/3 | 第三方隔离断言偏弱 (H1) |
| 9 | Slash + WS | ✅ 2/2 | |
| 10 | 公开频道预览 | ✅ 3/3 | |
| 11 | 多设备 | ✅ 3/3 | |
| 12 | 频道隔离交叉 | ✅ 4/4 | |
| 13 | 频道删除级联 | ✅ 3/3 | |
| 14 | 成员变更系统消息 | ❌ 0/3 | 断言全部无效 (C3) |
| 15 | 快速连续 | ⚠️ 2/2 | 顺序断言与设计不一致 (H5) |
| 16 | 并发成员+消息 | ✅ 2/2 | |
| 17 | Token 轮换 | ❌ 1/3 | 核心用例缺失 (C4) |
| 18 | 用户级联删除 | ❌ 0/4 | 全部 skip (C1) |
| 19 | 并发上传 | ✅ 2/2 | |
| 20 | Reaction 双向 | ⚠️ 4/4 | 事件类型偏差 (H2) |

**零 ws.js mock**：✅ 全部 20 个文件只 mock `../db.js`，无 ws module mock。

**资源清理**：✅ 19/20 文件有 afterAll 关闭 WS + app + db。`user-delete-cascade-e2e.test.ts` 无 afterAll（因为整个文件是 skip）。

**与 B24 重复**：设计文档已做去重说明，实际实现也遵循——B25 测多用户/WS 推送/复合流程，B24 测 CRUD。无明显重复。
