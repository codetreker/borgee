# COL-B19: Remote Explorer — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 概述

远程文件浏览器。用户在 Collab 浏览器端查看远程机器上的文件。机器上跑 Remote Agent，WS 连到 Server。

## 2. 架构

```
浏览器 → Collab Server → WebSocket → Remote Agent → 文件系统
                                                      ↓
                                              ls / readFile / stat
```

## 3. 数据库

```sql
-- Migration: B19
CREATE TABLE IF NOT EXISTS remote_nodes (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  machine_name TEXT NOT NULL,
  connection_token TEXT NOT NULL UNIQUE,
  last_seen_at TEXT,
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_remote_nodes_user ON remote_nodes(user_id);

CREATE TABLE IF NOT EXISTS remote_bindings (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES remote_nodes(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES channels(id),
  path TEXT NOT NULL,
  label TEXT,
  created_at TEXT DEFAULT (datetime('now')),
  UNIQUE(node_id, channel_id, path)
);
```

## 4. Server API

### 4.1 Node 管理

```
GET    /api/v1/remote/nodes              -- 当前用户的 node 列表
POST   /api/v1/remote/nodes              -- 创建 node（生成 connection token）
DELETE /api/v1/remote/nodes/:id          -- 删除 node
```

### 4.2 Binding 管理

```
GET    /api/v1/remote/nodes/:nodeId/bindings           -- 该 node 的绑定
POST   /api/v1/remote/nodes/:nodeId/bindings           -- 绑定目录到 channel
DELETE /api/v1/remote/nodes/:nodeId/bindings/:id       -- 解绑
GET    /api/v1/channels/:channelId/remote-bindings     -- 该 channel 的绑定（owner only）
```

### 4.3 文件操作（通过 Node WS 代理）

```
GET /api/v1/remote/nodes/:nodeId/ls?path=/workspace    -- 列目录
GET /api/v1/remote/nodes/:nodeId/read?path=/foo.ts     -- 读文件
```

Server 收到请求 → 通过 RemoteNodeManager 转发给 Remote Agent → 等响应 → 返回。

所有 API 校验 `user_id === req.userId`（owner only）。

## 5. Server WS Endpoint（Remote Agent 连接）

```
GET /ws/remote?token=xxx
```

Remote Agent 用 connection token 认证连接。

### 5.1 RemoteNodeManager

类似 PluginManager，管理 Remote Agent 连接：

```typescript
class RemoteNodeManager {
  private connections: Map<string, WebSocket>; // nodeId → ws
  
  register(nodeId: string, ws: WebSocket): void;
  unregister(nodeId: string): void;
  isOnline(nodeId: string): boolean;
  
  async request(nodeId: string, data: any, timeoutMs?: number): Promise<any>;
}
```

### 5.2 WS 消息协议

复用 B21 的 request/response 格式：

```json
// Server → Agent: 请求
{ "type": "request", "id": "req_001", "data": { "action": "ls", "path": "/workspace" } }

// Agent → Server: 响应
{ "type": "response", "id": "req_001", "data": { "entries": [...] } }
```

支持的 action：
- `ls`：列目录 → `{ entries: [{ name, isDirectory, size, mtime }] }`
- `read`：读文件 → `{ content, mimeType, size }`
- `stat`：文件信息 → `{ size, mtime, isDirectory }`

## 6. Remote Agent（独立 npm 包）

`@collab/remote-agent`：

```bash
npx @collab/remote-agent \
  --server wss://collab.codetrek.cn \
  --token xxx \
  --dirs /workspace/collab,/var/log
```

### 6.1 功能

- WS 连接 + token 认证
- 接收 request，执行 ls/read/stat
- 目录白名单（只暴露 --dirs 指定的）
- 自动重连（指数退避）
- 心跳

### 6.2 实现

独立包 `packages/remote-agent/`：

```typescript
class RemoteAgent {
  connect(serverUrl: string, token: string): void;
  private handleRequest(data: any): Promise<any>;
  private isPathAllowed(path: string): boolean;
}
```

## 7. 前端

### 7.1 Remote Tab（侧边栏）

ChannelView 已有 chat/workspace tab（B20），加 "Remote" tab：
- 显示当前 channel 绑定的远程目录
- 文件树浏览
- 点击文件 → FileViewer 预览

### 7.2 Node 管理页面

`/settings/remote-nodes`（或在 channel 设置里）：
- 添加 node（生成 token）
- 查看 node 在线状态
- 绑定目录到 channel

### 7.3 组件

- `RemotePanel.tsx`：Remote tab 内容
- `RemoteTree.tsx`：远程文件树
- `NodeManager.tsx`：node 管理 UI

## 8. 改动文件

### Server
| 文件 | 改动 |
|------|------|
| `src/db.ts` | remote_nodes + remote_bindings 迁移 |
| `src/routes/remote.ts` | 新建：node/binding CRUD + 文件代理 API |
| `src/routes/ws-remote.ts` | 新建：Remote Agent WS endpoint |
| `src/remote-node-manager.ts` | 新建：连接管理 |

### Remote Agent（新包）
| 文件 | 改动 |
|------|------|
| `packages/remote-agent/src/index.ts` | 入口 + CLI |
| `packages/remote-agent/src/agent.ts` | WS 连接 + request handler |
| `packages/remote-agent/src/fs-ops.ts` | ls/read/stat 实现 |

### Client
| 文件 | 改动 |
|------|------|
| `components/RemotePanel.tsx` | 新建 |
| `components/RemoteTree.tsx` | 新建 |
| `components/NodeManager.tsx` | 新建 |
| `components/ChannelView.tsx` | 加 Remote tab |

## 9. Task Breakdown

### T1: 数据库 + Node CRUD API
- remote_nodes + remote_bindings 表
- Node 创建/删除/列表 API
- Binding CRUD API

### T2: Remote Agent WS + RemoteNodeManager
- `/ws/remote` endpoint
- RemoteNodeManager 类
- token 认证

### T3: Remote Agent 包
- `packages/remote-agent/` 新建
- WS 连接 + 重连
- ls/read/stat handler
- 目录白名单

### T4: 文件代理 API
- `GET /remote/nodes/:id/ls`
- `GET /remote/nodes/:id/read`
- 通过 RemoteNodeManager.request 转发

### T5: 前端 Remote Tab + 文件树
- RemotePanel + RemoteTree
- ChannelView 加 Remote tab
- FileViewer 复用

### T6: Node 管理 UI
- NodeManager 组件
- 添加 node（token 生成 + 显示）
- 在线状态 + 绑定管理

## 10. 验收标准

- [ ] 创建 node 生成 token
- [ ] Remote Agent 用 token 连接成功
- [ ] 浏览远程文件树
- [ ] 点击文件预览（FileViewer）
- [ ] 只有 owner 可见
- [ ] Agent 离线时提示
- [ ] 目录白名单生效
