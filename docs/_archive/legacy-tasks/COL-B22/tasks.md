# COL-B22: 消息路径文件链接 — Task 分解

依赖前提：B21（Plugin WS request/response 机制已就绪，`pluginManager.request()` 已可用）

---

## Task 1: Server API + Plugin 文件读取

**目标**：新增 `GET /api/v1/agents/:agentId/files?path=...` endpoint，通过 Plugin WS 读取 Agent 本地文件；Plugin 端新增 `read_file` handler + 白名单机制。

### 改动文件

| 包 | 文件 | 改动说明 | 预估行数 |
|----|------|----------|----------|
| server | `src/routes/agents.ts` | 新增 `GET /agents/:agentId/files` endpoint：校验 owner、检查 plugin 在线、调用 `pluginManager.request()` | +35 |
| plugin | `src/file-access.ts` | **新建**：白名单加载（`~/.config/collab/file-access.json`）、路径校验、`readFile` 函数（含 size 限制 + mime 检测） | +60 |
| plugin | `src/ws-client.ts` 或 `src/inbound.ts` | `onRequest` handler 增加 `read_file` action 分支，调用 `file-access.ts` | +15 |
| server | `src/__tests__/agents-files.test.ts` | **新建**：endpoint 测试（owner 校验、离线 503、正常读取 mock） | +80 |

**总计**：~190 行

### 验证方式

1. 单元测试：`pnpm --filter server test -- agents-files`
2. 手动：Plugin 在线时 `curl GET /api/v1/agents/:id/files?path=/workspace/foo.ts` 返回文件内容
3. 手动：Plugin 离线时返回 503 `agent_offline`
4. 手动：路径不在白名单返回 403 `path_not_allowed`

### 依赖关系

- 无前置 task 依赖（Task 2、3 依赖本 task 的 API）
- 依赖 B21 的 `pluginManager.request()` 已存在 ✅

---

## Task 2: 前端路径检测 + FileLink 组件

**目标**：Agent 消息中的绝对路径自动渲染为可点击 `<FileLink>`，点击后通过 Task 1 的 API 读取文件并用已有 FileViewer 展示。

### 改动文件

| 包 | 文件 | 改动说明 | 预估行数 |
|----|------|----------|----------|
| client | `src/components/FileLink.tsx` | **新建**：接收 `path` + `agentId`，点击调用 `api.getAgentFile()`，成功后打开 FileViewer overlay | +55 |
| client | `src/components/MessageItem.tsx` | 在 `renderedContent` 逻辑后增加 `renderWithFileLinks()`：对 agent 消息 + owner 身份时，用正则替换路径为 `<FileLink>` 组件（需从 `dangerouslySetInnerHTML` 改为混合渲染） | +40 |
| client | `src/lib/api.ts` | 新增 `getAgentFile(agentId, path)` 函数 | +10 |
| client | `src/lib/file-links.ts` | **新建**：`FILE_PATH_RE` 正则 + `parseFileLinks(content)` 工具函数，返回 text/path 片段数组 | +25 |
| client | `src/types.ts` | `User` 接口添加可选 `owner_id` 字段（或用已有 `api.Agent` 类型；需确保 MessageItem 能判断 agent owner 关系） | +1 |

**总计**：~131 行

### 关键设计决策

- **渲染方式改动**：当前 MessageItem 用 `dangerouslySetInnerHTML` 渲染 markdown HTML。对包含 FileLink 的 agent 消息，需改为 React 混合渲染（text 节点 + `<FileLink>` 组件交替），因为 FileLink 需要 React state 管理。
- **Owner 判断**：`state.users` 的 `User` 类型没有 `owner_id`。方案：在 `/api/v1/users` 返回中增加 `owner_id`（仅 agent 用户有值），或在前端通过 `fetchAgents()` 缓存 agent→owner 映射。推荐前者，改动更小。
- **FileViewer 复用**：现有 `FileViewer` 接收 `WorkspaceFile` + `channelId`，为 workspace 设计。FileLink 场景需要一个轻量版本，直接传 `{ name, content, mime_type, size }` 而非走 workspace download 流程。可以给 FileViewer 加一个 `content` prop 模式，或新建 `RemoteFileViewer`。推荐给 FileViewer 加 prop。

### 验证方式

1. 启动 dev server，登录 agent owner 账号
2. Agent 发一条包含 `/workspace/collab/package.json` 的消息
3. 确认路径渲染为蓝色可点击链接，非 owner 看到纯文本
4. 点击链接 → FileViewer 弹出并显示文件内容
5. 非绝对路径（`foo.ts`、`./bar`）不被检测

### 依赖关系

- **依赖 Task 1**：`api.getAgentFile()` 需要后端 endpoint
- Task 3 依赖本 task 的 FileLink 组件

---

## Task 3: 离线/错误处理

**目标**：Agent 离线、读取超时、白名单拒绝等异常场景的 UI 反馈。

### 改动文件

| 包 | 文件 | 改动说明 | 预估行数 |
|----|------|----------|----------|
| client | `src/components/FileLink.tsx` | 增加错误状态处理：503 → "Agent 离线" toast + 链接变灰；超时 → "读取超时"；`path_not_allowed` → "路径不在允许范围" | +30 |
| client | `src/components/Toast.tsx` | 如果当前 Toast 不支持 programmatic trigger，增加 `showToast()` 全局调用方式（或复用已有机制） | +10~20 |
| client | CSS（`App.css` 或对应样式文件） | `.file-link-disabled` 灰色样式、`.file-link-loading` 动画 | +15 |
| server | `src/routes/agents.ts` | 细化错误码：区分 plugin 离线 (503)、超时 (504)、path_not_allowed (403)、file_not_found (404) | +15 |

**总计**：~70~80 行

### 错误场景覆盖

| 场景 | Server 响应 | 前端行为 |
|------|-------------|----------|
| Plugin 未连接 | 503 `agent_offline` | Toast "Agent 离线，无法读取文件"，链接灰色 |
| Plugin 请求超时 | 504 `timeout` | Toast "文件读取超时" |
| 白名单拒绝 | 403 `path_not_allowed` | Toast "该路径不在允许读取范围" |
| 文件不存在 | 404 `file_not_found` | Toast "文件不存在" |
| 文件过大 | 413 `file_too_large` | Toast "文件过大，无法预览" |

### 验证方式

1. 停止 Plugin → 点击 FileLink → 看到 "Agent 离线" 提示
2. Plugin 运行但路径不在白名单 → 看到 "路径不在允许范围"
3. 路径指向不存在的文件 → 看到 "文件不存在"
4. 所有 toast 3 秒后自动消失
5. 离线状态链接显示为灰色、cursor 变 not-allowed

### 依赖关系

- **依赖 Task 1**：需要 server 端已实现错误码区分
- **依赖 Task 2**：需要 FileLink 组件已存在

---

## 实施顺序

```
Task 1 (Server + Plugin)  →  Task 2 (前端 FileLink)  →  Task 3 (错误处理)
```

严格顺序依赖，不可并行。

## 总改动量估算

| Task | 新增行数 | 新文件数 | 改动文件数 |
|------|----------|----------|------------|
| T1 | ~190 | 2 | 2 |
| T2 | ~131 | 2 | 3 |
| T3 | ~75 | 0 | 4 |
| **合计** | **~396** | **4** | **9** |
