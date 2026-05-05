# COL-B25: 复杂场景集成测试 — Task Breakdown

日期：2026-04-23

## 总览

20 个场景，分 8 个 task，每个可独立 commit。  
测试目录：`packages/server/src/__tests__/`  
所有测试：零 mock（仅 `vi.mock('../db.js')` 注入 in-memory SQLite），真实 server + 真实 HTTP/WS。

---

## Task 0：扩展基础设施（前置）

**目的**：补齐 ws-helpers / setup 中设计文档依赖但尚未实现的能力。

拆为两个子任务：

### T0a：WS/SSE helpers + connectWS headers 支持

| 文件 | 变更 | 行数 |
|------|------|------|
| `ws-helpers.ts` | `connectWS` 增加 `options.headers` 支持（当前仅支持 query params，场景 2/4/17 需要 `Authorization` header）；新增 SSE 连接 helper `connectSSE` 供场景 4 使用 | ~60 行 |

### T0b：createTestDb 补 remote_nodes + workspace tmpdir

| 文件 | 变更 | 行数 |
|------|------|------|
| `setup.ts` | `createTestDb` 补充 `remote_nodes` 表初始化（场景 5 Remote Explorer 依赖）；新增临时目录 setup/cleanup helper 供 workspace 相关场景使用 | ~40 行 |

**T0 预估总行数：~100 行**

### 验证

- 现有 B24 测试全部通过（`vitest run --grep integration`）
- `connectWS(port, '/ws/plugin', { headers: { authorization: 'Bearer xxx' } })` 可正常连接
- `createTestDb` 返回的 db 包含 `remote_nodes` 表
- workspace tmpdir helper 创建/清理正常

### 依赖

无

---

## Task 1：P0 场景 1–2（聊天生命周期 + Agent-Human）

### 场景 1：完整聊天 + WS 推送

| 项目 | 内容 |
|------|------|
| 文件 | `chat-lifecycle-e2e.test.ts`（新增） |
| 行数 | ~120 行 |
| 用例 | 5 个：发消息→WS new_message、编辑→WS message_edited、删除→WS message_deleted、Reaction→WS reaction_added、非成员不收到推送 |
| 验证 | `vitest run chat-lifecycle-e2e` 全过 |

### 场景 2：Agent-Human 完整往返

| 项目 | 内容 |
|------|------|
| 文件 | `agent-human-e2e.test.ts`（新增） |
| 行数 | ~110 行 |
| 用例 | 3 个：人 @agent → Plugin WS 收到 message+mention、Agent apiCall 回复 → 人 WS 收到、不 @ 则无 mention |
| 验证 | `vitest run agent-human-e2e` 全过 |

### 依赖

Task 0（connectWS headers 支持）

---

## Task 2：P0 场景 3–4（权限隔离 + 三通道一致性）

### 场景 3：权限动态变化 + WS 隔离

| 项目 | 内容 |
|------|------|
| 文件 | `permission-ws-e2e.test.ts`（新增） |
| 行数 | ~100 行 |
| 用例 | 4 个：非成员 HTTP 403/404、非成员 WS 不收到事件、邀请后收到、踢出后不收到 |
| 验证 | `vitest run permission-ws-e2e` 全过 |

### 场景 4：SSE/WS/Poll 三通道一致性

| 项目 | 内容 |
|------|------|
| 文件 | `three-channel-consistency-e2e.test.ts`（新增） |
| 行数 | ~160 行 |
| 用例 | 1 个（重量级）：同时建 WS、SSE（http.get text/event-stream）、Poll 客户端，发消息后三端 payload 一致 |
| 验证 | `vitest run three-channel-consistency-e2e` 全过；关注 SSE 解析逻辑和超时 |

### 依赖

Task 0

---

## Task 3：P1 场景 5–7（Remote Explorer + Workspace + 分页）

### 场景 5：Remote Explorer 复合流程

| 项目 | 内容 |
|------|------|
| 文件 | `remote-explorer-e2e.test.ts`（新增） |
| 行数 | ~100 行 |
| 用例 | 1 个长链：注册→WS连接→列目录→读文件→断连503→重连→再读成功 |
| 验证 | `vitest run remote-explorer-e2e` 全过 |
| 注意 | B24 `remote-explorer.integration.test.ts` 已有基础 case，本场景仅覆盖连贯多步生命周期，不重复 |

### 场景 6：Workspace 消息引用附件 + 下载

| 项目 | 内容 |
|------|------|
| 文件 | `workspace-message-e2e.test.ts`（新增） |
| 行数 | ~60 行 |
| 用例 | 1 个：上传→发消息引用→下载内容一致 |
| 验证 | `vitest run workspace-message-e2e` 全过 |
| 注意 | B24 `workspace-flow.integration.test.ts` 已有 upload/list/rename，不重复 |

### 场景 7：分页 + 实时消息共存

| 项目 | 内容 |
|------|------|
| 文件 | `pagination-realtime-e2e.test.ts`（新增） |
| 行数 | ~70 行 |
| 用例 | 2 个：150 条消息分页（100+50+hasMore）、分页期间 WS 实时到达 |
| 验证 | `vitest run pagination-realtime-e2e` 全过 |

### 依赖

Task 0

---

## Task 4：P1 场景 8–10（DM + Slash + 公开频道）

### 场景 8：DM 完整链路

| 项目 | 内容 |
|------|------|
| 文件 | `dm-lifecycle-e2e.test.ts`（新增） |
| 行数 | ~70 行 |
| 用例 | 3 个：创建 DM、DM 发消息对方 WS 收到、第三方看不到 |
| 验证 | `vitest run dm-lifecycle-e2e` 全过 |

### 场景 9：Slash Commands + WS 推送

| 项目 | 内容 |
|------|------|
| 文件 | `slash-ws-e2e.test.ts`（新增） |
| 行数 | ~70 行 |
| 用例 | 2 个：/topic → channel_updated、/invite → 新成员 WS 收到事件 |
| 验证 | `vitest run slash-ws-e2e` 全过 |

### 场景 10：公开频道预览 + 加入

| 项目 | 内容 |
|------|------|
| 文件 | `public-preview-e2e.test.ts`（新增） |
| 行数 | ~70 行 |
| 用例 | 3 个：未加入用户看 24h 预览、自助加入后收 WS、加入后分页可见旧消息 |
| 验证 | `vitest run public-preview-e2e` 全过 |

### 依赖

Task 0

---

## Task 5：P1 场景 11–12（多设备 + 频道隔离交叉）

### 场景 11：多设备同一用户

| 项目 | 内容 |
|------|------|
| 文件 | `multi-device-e2e.test.ts`（新增） |
| 行数 | ~90 行 |
| 用例 | 3 个：两连接都收到、断一个另一个不受影响、两连接订阅不同频道互不干扰 |
| 验证 | `vitest run multi-device-e2e` 全过 |

### 场景 12：DM + 公开 + 私有频道隔离交叉

| 项目 | 内容 |
|------|------|
| 文件 | `channel-isolation-e2e.test.ts`（新增） |
| 行数 | ~110 行 |
| 用例 | 4 个：公开只推公开订阅者、私有不泄漏到 DM/公开、DM 仅双方、三频道同时发各自独立 |
| 验证 | `vitest run channel-isolation-e2e` 全过 |

### 依赖

Task 0

---

## Task 6：P2 场景 13–16（级联 + 系统消息 + 竞态）

### 场景 13：频道删除级联

| 项目 | 内容 |
|------|------|
| 文件 | `channel-delete-cascade-e2e.test.ts`（新增） |
| 行数 | ~60 行 |
| 用例 | 3 个：删频道→WS channel_deleted、成员列表 404、消息 404 |
| 验证 | `vitest run channel-delete-cascade-e2e` 全过 |

### 场景 14：成员变更系统消息

| 项目 | 内容 |
|------|------|
| 文件 | `member-change-sysmsg-e2e.test.ts`（新增） |
| 行数 | ~60 行 |
| 用例 | 3 个：加入→WS system 消息、离开→WS system 消息、HTTP 历史可查 |
| 验证 | `vitest run member-change-sysmsg-e2e` 全过 |

### 场景 15：快速连续操作

| 项目 | 内容 |
|------|------|
| 文件 | `rapid-fire-e2e.test.ts`（新增） |
| 行数 | ~60 行 |
| 用例 | 2 个：连发 5 条全部入库且按序、WS 按序收到 |
| 验证 | `vitest run rapid-fire-e2e` 全过 |

### 场景 16：并发成员变更 + 消息

| 项目 | 内容 |
|------|------|
| 文件 | `concurrent-member-msg-e2e.test.ts`（新增） |
| 行数 | ~50 行 |
| 用例 | 2 个：踢出与发消息并发→无 500、DB 状态一致 |
| 验证 | `vitest run concurrent-member-msg-e2e` 全过 |

### 依赖

Task 0

---

## Task 7：P2 场景 17–20（Token + 用户级联 + 并发上传 + Reaction）

### 场景 17：Token 轮换中的 WS

| 项目 | 内容 |
|------|------|
| 文件 | `token-rotation-ws-e2e.test.ts`（新增） |
| 行数 | ~60 行 |
| 用例 | 3 个：rotate-key→旧 WS 收到 4001、新 key 重连成功、旧 key 重连失败 |
| 验证 | `vitest run token-rotation-ws-e2e` 全过 |

### 场景 18：级联删除完整性

| 项目 | 内容 |
|------|------|
| 文件 | `user-delete-cascade-e2e.test.ts`（新增） |
| 行数 | ~70 行 |
| 用例 | 4 个：删用户→WS 断开、agent 不可用、邀请码作废、频道成员列表清除 |
| 验证 | `vitest run user-delete-cascade-e2e` 全过 |

### 场景 19：Workspace 文件并发上传

| 项目 | 内容 |
|------|------|
| 文件 | `workspace-concurrent-upload-e2e.test.ts`（新增） |
| 行数 | ~50 行 |
| 用例 | 2 个：同名并发上传→两个都成功 ID 不同、列表包含两个文件无损坏 |
| 验证 | `vitest run workspace-concurrent-upload-e2e` 全过 |

### 场景 20：消息 Reaction + WS 双向

> **与场景 1 的区别**：场景 1 是基础聊天生命周期综合测试（发消息+编辑+删除+reaction 各一个基础 case）；场景 20 专注于 reaction 的完整双向行为——取消 reaction、多人不同 emoji 独立性、WS 双向通知（reaction_added + reaction_removed），是 reaction 子系统的深度覆盖。

| 项目 | 内容 |
|------|------|
| 文件 | `reaction-bidirectional-e2e.test.ts`（新增） |
| 行数 | ~90 行 |
| 用例 | 4 个：加 reaction→WS reaction_added、取消→WS reaction_removed、多人不同 emoji→各自独立、HTTP GET 包含 reactions |
| 验证 | `vitest run reaction-bidirectional-e2e` 全过 |

### 依赖

Task 0

---

## 汇总

| Task | 场景 | 优先级 | 新增文件数 | 预估总行数 | 依赖 |
|------|------|--------|-----------|-----------|------|
| T0 | 基础设施扩展（T0a+T0b） | — | 0（改 2） | ~100 | 无 |
| T1 | 1–2 | P0 | 2 | ~230 | T0 |
| T2 | 3–4 | P0 | 2 | ~260 | T0 |
| T3 | 5–7 | P1 | 3 | ~230 | T0 |
| T4 | 8–10 | P1 | 3 | ~210 | T0 |
| T5 | 11–12 | P1 | 2 | ~200 | T0 |
| T6 | 13–16 | P2 | 4 | ~230 | T0 |
| T7 | 17–20 | P2 | 4 | ~270 | T0 |
| **合计** | **20 场景** | | **20 新 + 2 改** | **~1672** | |

## 建议执行顺序

```
T0 → T1 → T2 → T3 → T4 → T5 → T6 → T7
          ↑ P0 完成 ↑    ↑ P1 完成 ↑    ↑ P2 完成 ↑
```

T1–T7 之间无互相依赖（仅依赖 T0），同优先级内可并行开发。建议每个 Task 完成后独立 commit + 跑一次全量 `vitest run`。

---

## Review 修正记录

### CRITICAL
- C1: Task 编号统一为 T0-T7（无冲突表格，已确认一致）
- C2: 场景 20 与场景 1 Reaction 去重说明已添加
- C3: T0 补全 ws-helpers 扩展（SSE helper）
- C4: connectWS headers 支持纳入 T0a
- C5: createTestDb 补 remote_nodes 纳入 T0b

### HIGH
- H1: T0 行数修正（8→100），拆为 T0a+T0b
- H2: 场景 4 行数修正（100→160）
- H3: B24 重叠：场景 1/3/8/10/12 与 B24 有重叠但测试维度不同（B25 测多用户+WS 推送，B24 测 CRUD）
- H4: 并发断言：场景 15/16 不断言顺序，只断言完整性和无 500
- H5: collectMessages flaky：负面断言用 short timeout（200ms），标注已知限制
- H6: 场景 18 用户级联：需确认 DELETE /users/:id API 是否存在，不存在则 skip
- H7: CI 资源：建议串行跑集成测试（vitest --pool=forks --poolOptions.forks.singleFork）
- H8: workspace tmpdir：T0b 添加临时目录 setup/cleanup helper
