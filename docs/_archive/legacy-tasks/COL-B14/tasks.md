Now I have the full picture. Here's the task breakdown:

---

## COL-B14: Plugin Reactions 支持 — Task List

### T1: api-client.ts — 新增 reaction / edit / delete API 方法

| 项目 | 内容 |
|---|---|
| **文件** | `packages/plugin/src/api-client.ts` |
| **改动** | 新增 4 个 standalone helper 函数：`addReaction`, `removeReaction`, `editMessage`, `deleteMessage`；`CollabApiClient` 类也加对应方法 |
| **预估** | ~60 行 |
| **验证** | 单元测试 mock HTTP，确认正确调用 `PUT/DELETE /api/v1/messages/:id/reactions` 和 `PUT/DELETE /api/v1/messages/:id` |
| **依赖** | 无（P5 和 B10 的服务端 API 已就绪） |

---

### T2: outbound.ts — 处理 add_reaction / remove_reaction

| 项目 | 内容 |
|---|---|
| **文件** | `packages/plugin/src/outbound.ts` |
| **改动** | 在 `sendCollabText` 旁新增 `handleCollabReaction(params)` 函数，根据 type=`add_reaction`/`remove_reaction` 调用 T1 的 API 方法 |
| **预估** | ~40 行 |
| **验证** | 集成测试：Agent 发 `add_reaction` → 消息上出现 reaction；发 `remove_reaction` → reaction 移除 |
| **依赖** | T1 |

---

### T3: outbound.ts — 处理 edit_message / delete_message

| 项目 | 内容 |
|---|---|
| **文件** | `packages/plugin/src/outbound.ts` |
| **改动** | 新增 `handleCollabMessageEdit` / `handleCollabMessageDelete`，调用 T1 的 edit/delete API |
| **预估** | ~40 行 |
| **验证** | Agent 编辑/删除自己的消息成功；编辑/删除别人消息返回 403 |
| **依赖** | T1 |

---

### T4: SSE inbound — 放行 reaction_update 事件

| 项目 | 内容 |
|---|---|
| **文件** | `packages/plugin/src/sse-client.ts` (L250), `packages/plugin/src/gateway.ts` (L62), `packages/plugin/src/types.ts` (L81) |
| **改动** | `dispatchSSEEvent` 和 `runPollLoop` 的 kind 过滤条件加入 `"reaction_update"`；`types.ts` CollabEvent kind union 加 `'reaction_update'` |
| **预估** | ~10 行（3 处各改 1-3 行） |
| **验证** | 另一用户给消息加 reaction → Agent SSE/poll 收到 `reaction_update` 事件并转发给 inbound handler |
| **依赖** | 无（服务端已推送 `reaction_update`） |

---

### T5: inbound.ts — 解析 reaction_update 事件

| 项目 | 内容 |
|---|---|
| **文件** | `packages/plugin/src/inbound.ts` |
| **改动** | `handleCollabInbound` 增加对 `reaction_update` kind 的处理分支，构造通知传给 Agent |
| **预估** | ~20 行 |
| **验证** | Agent 收到结构化的 reaction 变更通知（含 message_id, emoji, user_id, action） |
| **依赖** | T4 |

---

### 依赖关系图

```
T1 (api-client 新方法)
 ├── T2 (outbound reactions)
 └── T3 (outbound edit/delete)

T4 (SSE/poll 放行 reaction_update)
 └── T5 (inbound 解析 reaction_update)
```

T1 和 T4 可并行开发。T2/T3 依赖 T1，T5 依赖 T4。总预估 ~170 行新增代码。
