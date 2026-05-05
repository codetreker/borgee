# COL-R01-TEST 测试迁移技术设计

> 生成日期: 2026-04-26
>
> TS 测试总计: 55 文件, ~500 it-cases
> Go 测试总计: 35 文件, ~208 Test* 函数

---

## TS vs Go 测试对照表

### Auth 认证

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| 基础认证 | auth.test.ts | 20 | auth_test.go, auth_coverage_test.go | 5+13=18 | ⚠️ 部分 | 部分注册/登录边界场景 |
| API Key 认证 | apikey-auth.test.ts | 9 | coverage_boost_test.go (TestAPIKeyAuth) | 1 | ⚠️ 部分 | API Key 多场景(过期、权限、轮换) |
| Dev 认证绕过 | dev-auth-bypass.test.ts | 5 | ws_internal_test.go (DevFallbacks) | 1 | ⚠️ 部分 | 多用户 dev bypass、header 优先级 |
| Token 轮换 WS | token-rotation-ws-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | Token 轮换后 WS 连接保持 |
| WS 认证 Header | ws-auth-header.test.ts | 5 | ws_coverage_test.go (WSAuthMethods) | 1 | ⚠️ 部分 | 多种 header 格式、无效 token |
| Stream 认证 Header | stream-auth-header.test.ts | 5 | — | 0 | ❌ 缺失 | SSE 流认证 header 处理 |
| WS Remote 认证 | ws-remote-auth-header.test.ts | 5 | ws_coverage_test.go (RemoteWS*) | 2 | ⚠️ 部分 | Remote WS 认证边界 |

### Channels 频道

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| 频道 CRUD | channels.test.ts | 49 | channels_test.go, coverage_boost*.go | 2+5=7 | ⚠️ 部分 | 私有频道、归档、slug 冲突、批量操作 |
| 频道生命周期 | channel-lifecycle.integration.test.ts | 9 | — | 0 | ❌ 缺失 | 创建→加入→发消息→归档→删除 完整流程 |
| 频道删除级联 | channel-delete-cascade-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | 删除频道后消息/成员/反应清理 |
| 频道隔离 | channel-isolation-e2e.test.ts | 4 | — | 0 | ❌ 缺失 | 非成员不可见/不可操作 |
| 频道排序 | channel-reorder.test.ts, channel-sort-e2e.test.ts | 8+8=16 | coverage_boost2 (Reorder) | 1 | ⚠️ 部分 | LexoRank 多场景排序 |
| 频道分组 | channel-groups.test.ts | 16 | coverage_boost2 (GroupNameTooLong) | 1 | ⚠️ 部分 | 分组 CRUD、频道归属、重命名 |
| 三频道一致性 | three-channel-consistency-e2e.test.ts | 1 | — | 0 | ❌ 缺失 | 多频道并发消息一致性 |

### Messages 消息

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| 消息 CRUD | messages.test.ts | 21 | messages_test.go, coverage_boost2 | 1+3=4 | ⚠️ 部分 | 编辑/删除权限、回复、mention 解析 |
| 并发消息 | concurrent-member-msg-e2e.test.ts | 2 | — | 0 | ❌ 缺失 | 并发发送消息一致性 |
| 分页+实时 | pagination-realtime-e2e.test.ts | 2 | — | 0 | ❌ 缺失 | 分页加载同时实时推送 |
| Workspace 消息 | workspace-message-e2e.test.ts | 1 | — | 0 | ❌ 缺失 | 跨 workspace 消息隔离 |

### Reactions 反应

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| 反应 CRUD | reactions.test.ts | 10 | reactions_test.go | 1 | ⚠️ 部分 | 重复反应、取消反应、多用户反应 |
| 双向反应 E2E | reaction-bidirectional-e2e.test.ts | 4 | — | 0 | ❌ 缺失 | 反应操作实时推送到 WS |

### WebSocket

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| 核心 WS | core.test.ts | 17 | ws_test.go, ws_coverage*.go, hub_test.go, ws_client_test.go | 3+10+4+4+6=27 | ✅ 有 | — |
| WS 权限 | permission-ws-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | 权限变更后 WS 行为 |
| 多设备 | multi-device-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | 同用户多 WS 连接 |
| 快速发送 | rapid-fire-e2e.test.ts | 2 | — | 0 | ❌ 缺失 | 高频消息发送不丢失 |
| WS Plugin | ws-plugin.test.ts | 13 | ws_coverage (PluginWS*) | 2 | ⚠️ 部分 | Plugin WS 完整协议 |

### SSE 服务端推送

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| SSE | (在 core.test.ts 中) | ~3 | sse_test.go, error_branches (SSEBackfill) | 1+1=2 | ⚠️ 部分 | SSE 重连、backfill、断线恢复 |
| Poll | — | 0 | poll_test.go | 1 | ✅ Go only | — |

### DM 私信

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| DM 生命周期 | dm-lifecycle-e2e.test.ts | 3 | dm_test.go | 1 | ⚠️ 部分 | DM 创建→消息→关闭 完整流程 |
| 聊天生命周期 | chat-lifecycle-e2e.test.ts | 5 | — | 0 | ❌ 缺失 | 聊天全生命周期 |

### Workspace 工作区

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| Workspace CRUD | workspace.test.ts | 27 | workspace_test.go, coverage_boost*.go | 1+1+1=3 | ⚠️ 部分 | 文件上传、成员管理、设置 |
| Workspace 流程 | workspace-flow.integration.test.ts | 8 | — | 0 | ❌ 缺失 | 完整加入/邀请/退出流程 |
| 并发上传 | workspace-concurrent-upload-e2e.test.ts | 2 | — | 0 | ❌ 缺失 | 并发文件上传 |

### Remote 远程节点

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| Remote 基础 | remote.test.ts | 40 | remote_test.go, coverage_boost2 (RemoteNode*) | 1+1=2 | ❌ 缺失 | 节点注册/发现/代理/健康检查/断线重连 |
| Remote Explorer | remote-explorer*.test.ts | 1+8=9 | — | 0 | ❌ 缺失 | 远程文件浏览 |
| Remote Node Mgr | remote-node-manager.test.ts | 13 | — | 0 | ❌ 缺失 | 节点管理器生命周期 |

### Admin 管理

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| Admin+Agent DM | admin-agents-dm.test.ts | 32 | admin_test.go, agents_test.go | 8+6=14 | ⚠️ 部分 | Admin 面板操作、Agent 管理 |
| 用户删除级联 | user-delete-cascade-e2e.test.ts | ~3 | — | 0 | ❌ 缺失 | 删除用户后数据清理 |
| 公开预览 | public-preview-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | 未登录用户预览频道 |

### Agents 智能体

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| Agent-Human E2E | agent-human-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | Agent 与人类用户交互 |
| Agent 文件 | agents-files.test.ts | 10 | — | 0 | ❌ 缺失 | Agent 文件操作 |
| Require Mention | require-mention.integration.test.ts | 5 | — | 0 | ❌ 缺失 | @mention 触发 Agent 响应 |

### Commands 命令

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| 命令系统 | commands.test.ts, command-store.test.ts | 3+9=12 | ws_coverage (CommandStore) | 1 | ⚠️ 部分 | 命令注册/执行/参数解析 |
| Slash 命令 | slash-commands*.test.ts, slash-ws-e2e.test.ts | 3+5+2=10 | — | 0 | ❌ 缺失 | Slash 命令 E2E |

### Plugins 插件

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| Plugin 管理器 | plugin-manager.test.ts | 16 | — | 0 | ❌ 缺失 | 插件加载/卸载/生命周期 |
| Plugin 构建 | plugin-build.test.ts | 4 | — | 0 | ❌ 缺失 | 插件编译构建 |
| Plugin 通信 | plugin-comm.integration.test.ts | 9 | — | 0 | ❌ 缺失 | 插件间通信协议 |
| Plugin Mock | plugin-openclaw-mock.test.ts | 7 | — | 0 | ❌ 缺失 | OpenClaw 插件模拟 |

### 其他

| 功能域 | TS 文件 | TS 用例数 | Go 文件 | Go 用例数 | 覆盖状态 | 缺失场景 |
|--------|---------|-----------|---------|-----------|----------|----------|
| LexoRank | lexorank.test.ts | 14 | lexorank_test.go | 6 | ⚠️ 部分 | 边界值排序 |
| 迁移 | migration.integration.test.ts | 3 | migrations_test.go | 6 | ✅ 有 | — |
| Preview | preview.test.ts | 5 | — | 0 | ❌ 缺失 | 链接预览解析 |
| 文件链接 | file-link.integration.test.ts | 5 | upload_test.go | 1 | ⚠️ 部分 | 文件链接关联 |
| 并发控制 | concurrency.integration.test.ts | 2 | — | 0 | ❌ 缺失 | 并发写入控制 |
| 成员变更消息 | member-change-sysmsg-e2e.test.ts | 3 | — | 0 | ❌ 缺失 | 系统消息生成 |
| 服务器基础 | — | — | server_test.go, config_test.go | 18+5=23 | ✅ Go only | — |
| Store 层 | — | — | store_*.go, query_gap*.go | 3+22+10+6=41 | ✅ Go only | — |

---

## 需要迁移的场景清单（按优先级）

### P0 — 关键路径（影响核心功能正确性）

| # | 场景 | TS 来源 | 原因 |
|---|------|---------|------|
| 1 | 频道生命周期完整流程 | channel-lifecycle.integration.test.ts | 核心 CRUD 流程 |
| 2 | 消息 CRUD 完整测试 | messages.test.ts (21 cases) | 消息是核心功能，Go 仅 4 个测试 |
| 3 | 频道删除级联 | channel-delete-cascade-e2e.test.ts | 数据一致性 |
| 4 | DM/Chat 生命周期 | dm-lifecycle-e2e.test.ts, chat-lifecycle-e2e.test.ts | DM 是核心功能 |
| 5 | 反应完整 CRUD + 实时推送 | reactions.test.ts, reaction-bidirectional-e2e.test.ts | 反应实时性 |
| 6 | 频道隔离（权限） | channel-isolation-e2e.test.ts | 安全性关键 |
| 7 | Token 轮换 WS 保持 | token-rotation-ws-e2e.test.ts | 认证连续性 |
| 8 | API Key 完整场景 | apikey-auth.test.ts (9 cases) | API 接入安全 |
| 9 | 用户删除级联 | user-delete-cascade-e2e.test.ts | 数据清理 |

### P1 — 重要功能（影响用户体验）

| # | 场景 | TS 来源 | 原因 |
|---|------|---------|------|
| 10 | Workspace 完整流程 | workspace-flow.integration.test.ts | 工作区管理 |
| 11 | 频道排序（LexoRank 多场景） | channel-reorder.test.ts, channel-sort-e2e.test.ts | 排序正确性 |
| 12 | 频道分组 CRUD | channel-groups.test.ts | 分组管理 |
| 13 | SSE 重连 + backfill | stream-auth-header.test.ts | 离线恢复 |
| 14 | 多设备 WS | multi-device-e2e.test.ts | 多端同步 |
| 15 | Admin 管理面板 | admin-agents-dm.test.ts (32 cases) | 管理功能完整性 |
| 16 | Remote 节点基础 | remote.test.ts (40 cases) | 分布式基础 |
| 17 | Slash 命令 E2E | slash-commands-e2e.test.ts, slash-ws-e2e.test.ts | 命令系统 |
| 18 | WS 权限变更 | permission-ws-e2e.test.ts | 实时权限 |
| 19 | 分页+实时推送 | pagination-realtime-e2e.test.ts | 数据加载 |
| 20 | 公开预览 | public-preview-e2e.test.ts | 未登录访问 |

### P2 — 边缘场景（增强健壮性）

| # | 场景 | TS 来源 | 原因 |
|---|------|---------|------|
| 21 | 快速发送不丢失 | rapid-fire-e2e.test.ts | 高频场景 |
| 22 | 并发消息一致性 | concurrent-member-msg-e2e.test.ts | 并发安全 |
| 23 | 三频道一致性 | three-channel-consistency-e2e.test.ts | 多频道并发 |
| 24 | 并发上传 | workspace-concurrent-upload-e2e.test.ts | 文件并发 |
| 25 | 并发写入控制 | concurrency.integration.test.ts | 锁竞争 |
| 26 | 成员变更系统消息 | member-change-sysmsg-e2e.test.ts | 系统消息 |
| 27 | 链接预览 | preview.test.ts | 非核心功能 |
| 28 | Agent 交互 | agent-human-e2e.test.ts, agents-files.test.ts | Agent 子系统 |
| 29 | Plugin 系统 | plugin-*.test.ts (36 cases total) | 插件子系统 |
| 30 | Remote Explorer | remote-explorer*.test.ts | 远程浏览 |
| 31 | Remote Node Manager | remote-node-manager.test.ts | 节点管理 |

---

## 覆盖状态汇总

| 状态 | 数量 | 占比 |
|------|------|------|
| ✅ 已覆盖 | 2 功能域 (WS 核心, 迁移) | ~8% |
| ⚠️ 部分覆盖 | 13 功能域 | ~50% |
| ❌ 完全缺失 | 11 功能域 | ~42% |

TS 侧约 500 个 it-case，Go 侧约 208 个 Test 函数。考虑到 Go 的 table-driven 测试风格（一个 Test 函数内含多个 sub-test），实际覆盖的场景比 208 略多，但 **E2E 集成场景几乎全部缺失**。

---

## 迁移建议

### 1. 测试框架

- 使用 Go 标准库 `testing` + `net/http/httptest`，与现有 Go 测试保持一致
- E2E 测试使用 `internal/testutil` 中已有的 `TestServer` 辅助启动完整服务
- WS 测试使用 `gorilla/websocket` 客户端（已在 ws_test.go 中使用）
- SSE 测试使用 `bufio.Scanner` 解析 event-stream

### 2. 隔离策略

- 每个 Test 函数使用独立的 SQLite 内存库 (`:memory:`)，已有模式
- 用户/workspace 在每个 test 内独立创建，避免状态泄漏
- WS/SSE 连接在 `t.Cleanup` 中关闭
- 使用 `t.Parallel()` 加速，但同一 TestServer 内的测试串行执行

### 3. 迁移策略

**不建议 1:1 翻译 TS 测试为 Go**。原因：
- TS 测试使用 Jest/Vitest 风格（describe/it 嵌套），Go 推荐 table-driven + subtests
- 很多 TS E2E 测试验证的是 WS 事件推送，Go 侧需要重新设计 WS 客户端辅助函数

**推荐方案**：
1. **按功能域逐个迁移**，从 P0 开始
2. **先建 E2E 测试基础设施**：封装 `TestServer` + WS 客户端 + SSE 客户端 helper
3. **每个功能域写一个集成测试文件**，覆盖 TS 中的核心场景
4. **用 golden test (已有 golden_test.go) 验证 API 响应格式兼容性**

### 4. 优先级执行计划

| 阶段 | 内容 | 预计工作量 |
|------|------|-----------|
| Phase 0 | 搭建 E2E 测试基础设施 (WS/SSE helper) | 1-2d |
| Phase 1 | P0 场景迁移 (9 项) | 3-4d |
| Phase 2 | P1 场景迁移 (11 项) | 4-5d |
| Phase 3 | P2 场景迁移 (11 项) | 3-4d |

### 5. 特别关注

- **Plugin 系统** (36 TS cases): Go 服务器当前是否需要插件支持？如不需要可跳过
- **Remote 节点** (62 TS cases): 这是最大的缺失块，需确认 Go 服务器的 remote 架构是否与 TS 一致
- **Agent 系统**: 需确认 Go 侧 agent 功能范围后再决定测试迁移范围
