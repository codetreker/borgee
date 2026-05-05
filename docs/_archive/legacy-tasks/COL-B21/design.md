# COL-B21: Plugin SSE → WS 升级 — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 概述

将 Plugin 连接从 SSE（单向）升级到 WebSocket（双向），支持 server 主动向 Plugin 发请求。保留 SSE 作为降级。

## 2. WS 协议设计

### 2.1 消息格式

所有 WS 消息都是 JSON，统一格式：

```typescript
interface WsMessage {
  type: 'event' | 'request' | 'response' | 'api_request' | 'api_response';
  id?: string;        // request/response 的关联 ID
  event?: string;     // type=event 时的事件名
  data?: any;         // 负载
  error?: string;     // response 错误
}
```

### 2.2 消息类型

**Server → Plugin**：
- `event`：消息推送（和 SSE 格式兼容）
  ```json
  { "type": "event", "event": "new_message", "data": { ... } }
  ```
- `request`：server 向 Plugin 发请求（如读文件）
  ```json
  { "type": "request", "id": "req_abc123", "data": { "action": "read_file", "path": "/workspace/foo.ts" } }
  ```

**Plugin → Server**：
- `response`：响应 server 请求
  ```json
  { "type": "response", "id": "req_abc123", "data": { "content": "..." } }
  ```
- `api_request`：Plugin 主动调 API（发消息等）
  ```json
  { "type": "api_request", "id": "api_001", "data": { "method": "POST", "path": "/api/v1/messages", "body": { ... } } }
  ```

**Server → Plugin（API 响应）**：
- `api_response`：
  ```json
  { "type": "api_response", "id": "api_001", "data": { "status": 201, "body": { ... } } }
  ```

### 2.3 认证

WS 连接时通过 query parameter 传 API key：
```
wss://collab.codetrek.cn/ws/plugin?apiKey=xxx
```

Server 端验证 API key，提取 userId + agentId，注入到 WS 连接上下文。

## 3. Server 端实现

### 3.1 WS Endpoint

用 `@fastify/websocket`：

```typescript
fastify.register(require('@fastify/websocket'));

fastify.get('/ws/plugin', { websocket: true }, (socket, req) => {
  const apiKey = req.query.apiKey;
  // 验证 apiKey → 获取 agent
  // 注册到连接管理器
  pluginManager.register(agent.id, socket);
  
  socket.on('message', (raw) => {
    const msg = JSON.parse(raw);
    handlePluginMessage(agent, msg);
  });
  
  socket.on('close', () => {
    pluginManager.unregister(agent.id);
  });
});
```

### 3.2 连接管理器 PluginManager

```typescript
class PluginManager {
  private connections: Map<string, WebSocket>; // agentId → ws
  
  register(agentId: string, ws: WebSocket): void;
  unregister(agentId: string): void;
  
  // 推送事件（替代 SSE）
  pushEvent(agentId: string, event: string, data: any): void;
  
  // 向 Plugin 发请求并等待响应
  async request(agentId: string, data: any, timeoutMs?: number): Promise<any>;
  
  // 广播（给所有连接的 Plugin）
  broadcast(event: string, data: any): void;
}
```

`request()` 实现：生成 requestId → 发送 → 注册 pending Promise → 等 response 或超时。

### 3.3 处理 Plugin 的 API 请求

Plugin 通过 WS 发 `api_request`，server 路由到内部处理器（复用现有 route handler 逻辑）：

```typescript
async function handleApiRequest(agent: Agent, msg: WsMessage) {
  const { method, path, body } = msg.data;
  // 调用内部 API（inject 或直接调 service 层）
  const result = await internalApi(agent, method, path, body);
  send(agent.ws, { type: 'api_response', id: msg.id, data: result });
}
```

### 3.4 保留 SSE 降级

现有 SSE endpoint 不删除。Plugin 连 WS 优先，失败降级 SSE。

## 4. Plugin 端实现

### 4.1 WS Client

```typescript
class PluginWsClient {
  private ws: WebSocket;
  private pendingRequests: Map<string, { resolve, reject, timer }>;
  
  connect(serverUrl: string, apiKey: string): void;
  
  // 监听事件（和 SSE onmessage 兼容）
  onEvent(handler: (event: string, data: any) => void): void;
  
  // 响应 server 请求
  onRequest(handler: (data: any) => Promise<any>): void;
  
  // 主动调 API
  async apiCall(method: string, path: string, body?: any): Promise<any>;
  
  // 自动重连（指数退避）
  private reconnect(): void;
}
```

### 4.2 重连策略

断连后指数退避重连：1s → 2s → 4s → 8s → 16s → 30s（cap）。

### 4.3 Plugin outbound 适配

现有 outbound handler（send_message / add_reaction 等）从 HTTP 改为走 WS `api_request`：

```typescript
// 之前
await fetch(`${serverUrl}/api/v1/messages`, { method: 'POST', body });

// 之后
await wsClient.apiCall('POST', '/api/v1/messages', body);
```

## 5. 改动文件

### Server
| 文件 | 改动 |
|------|------|
| `package.json` | 加 `@fastify/websocket` |
| `src/routes/ws-plugin.ts` | 新建：WS endpoint |
| `src/plugin-manager.ts` | 新建：连接管理器 |
| `src/routes/stream.ts` | 改造：推送事件也走 PluginManager |

### Plugin（packages/plugin 或独立包）
| 文件 | 改动 |
|------|------|
| `src/ws-client.ts` | 新建：WS 客户端 |
| `src/outbound.ts` | HTTP → WS apiCall |
| `src/index.ts` | 连接方式从 SSE → WS |

## 6. Task Breakdown

### T1: Server WS endpoint + PluginManager
- `@fastify/websocket` 集成
- WS endpoint `/ws/plugin`（认证 + 连接管理）
- PluginManager 类（register/unregister/pushEvent）

### T2: 事件推送迁移
- 现有 SSE 推送逻辑接入 PluginManager
- 所有事件类型通过 WS 推送
- SSE 保留不动

### T3: Server→Plugin 请求通道
- PluginManager.request() 实现
- requestId 关联 + 超时机制
- Plugin 端 onRequest handler

### T4: Plugin WS Client
- WS 连接 + 认证
- 事件监听（兼容 SSE 格式）
- 自动重连（指数退避）

### T5: Plugin API 请求走 WS
- outbound handler HTTP → WS apiCall
- Server 端 handleApiRequest 路由

## 7. 验收标准

- [ ] Plugin 通过 WS 连接 server
- [ ] 所有消息事件通过 WS 推送
- [ ] Server 能向 Plugin 发请求并获得响应
- [ ] Plugin 通过 WS 发消息/reaction 等 API 调用
- [ ] 断连自动重连
- [ ] SSE 仍可用（降级）
