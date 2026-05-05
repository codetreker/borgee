# SSE 推送改造 — 技术设计文档

日期：2026-04-20 | 状态：Draft | 作者：飞马（架构师）

---

## 背景与问题

Collab Plugin 当前通过 HTTP 长轮询（`POST /api/v1/poll`）获取消息。长轮询存在两个已验证的缺陷：

1. **waiter 频道快照陈旧**：poll 进入等待时缓存了 channelIds，新加入频道的事件无法唤醒 waiter
2. **超时推进 cursor**：waiter 超时时曾把 cursor 推到全局 MAX，永久跳过消息（已修复，但根因仍在——每次 poll 都是独立请求，频道列表是快照）

SSE 从根本上消除这些问题：持久连接 + 服务端主动推送 = 无快照、无 cursor 管理、零延迟。

详见 [SSE PRD](../requirements/sse-push.md)。

## 目标

1. Plugin 通过 SSE 接收消息，新消息延迟 < 1 秒（当前长轮询最坏 30 秒）
2. 断线重连自动补发错过的消息（通过 `Last-Event-ID`）
3. 向后兼容：长轮询端点保留，旧 Plugin 仍可使用
4. Plugin 配置不变（只需 baseUrl + apiKey）

## 方案设计

### 整体架构变更

```
改造前：
  Plugin → POST /api/v1/poll (30s hold) → 返回 events → 再 POST → ...

改造后：
  Plugin → GET /api/v1/stream (持久连接) ←── 服务端推送 events
  Plugin → POST /api/v1/channels/:id/messages (发消息，不变)
```

核心变化只在**消息接收路径**，发消息和其他 API 不变。

### 服务端：SSE 端点

#### 新增路由：`GET /api/v1/stream`

```
packages/server/src/routes/stream.ts  (新文件)
```

**认证**：
- Header: `Authorization: Bearer col_xxx`（**首选**，Plugin 端必须用 header）
- Query param: `GET /api/v1/stream?api_key=col_xxx`（兜底，仅用于浏览器原生 EventSource 等无法设 header 的场景）
- Node.js EventSource 支持自定义 header，Plugin 端优先用 header 避免 API key 泄露到日志
- 服务端日志脱敏：query param 中的 api_key 记录为 `col_***`

**连接建立流程**：

```
1. 验证 API key → 获取 user
2. 查询 user 的 channel_members → 获取 channelIds
3. 读取 Last-Event-ID header → 作为起始 cursor
   - 无 Last-Event-ID → cursor = getLatestCursor()（从当前最新位置开始，只推新事件；和 poll gateway 的 bootstrap 语义一致）
   - 有 Last-Event-ID → 补发 cursor 之后的所有事件
4. 发送 HTTP 200，headers:
   Content-Type: text/event-stream
   Cache-Control: no-cache
   Connection: keep-alive
   X-Accel-Buffering: no  (防止 Caddy/Nginx 缓冲)
5. **注册到 SSE 客户端列表，但标记 `ready=false`**
6. **循环补发历史事件**（如果有 Last-Event-ID）：
   ```
   while (true):
     // 用 SQL 合并查询（同实时路径的 getEventsSinceWithChanges）
     // SQL: WHERE cursor > ? AND (channel_id IN (...) OR kind IN (...))
     allEvents = getEventsSinceWithChanges(cursor, 100, channelIds, CHANNEL_CHANGE_KINDS)
     
     for event in allEvents:
       推送 event（同 notifySSEClients 的 isRelevant/自身过滤逻辑）
       client.lastCursor = event.cursor  // 每条都推进，包括最后一批
       
       // 如果处理了频道变更事件且 isRelevant，刷新 channelIds
       if CHANNEL_CHANGE_KINDS.has(event.kind) && isRelevant:
         channelIds = getUserChannelIds(db, client.userId)
     
     if allEvents.length < 100: break  // 追上了
     cursor = allEvents.last.cursor
   ```
   
   **关键**：用 `getEventsSinceWithChanges` 做 SQL 层合并（`channel_id IN (...) OR kind IN (...)`），
   确保频道变更事件不会被 LIMIT 截断。变更事件处理后立即刷新 channelIds，
   下一轮循环自然用新的 channelIds 拉到新频道的消息。

7. **原子收尾**：标记 `ready=true` 后循环 drain 直到追平——
   ```
   client.ready = true;
   // 循环追尾：补发完成到 ready=true 之间可能漏掉的事件
   while (true):
     drained = drainPendingEvents(client);  // 返回本次处理的事件数
     if drained === 0: break;  // 追平了
     // 如果 drain 中处理了频道变更，channelIds 已刷新，继续 drain
   ```
   循环 drain 确保：即使 drain 中发现频道变更导致新频道可见，也会继续拉取新频道的消息，不会漏。
8. 启动心跳定时器（15 秒）

**关键：补发完成前 `ready=false`，`notifySSEClients()` 跳过该客户端；切换 ready 后立即 drain，消除窗口。**
```

**事件格式**：

```
event: message
id: 102
data: {"id":"abc-123","channel_id":"4fe3fef6-...","sender_id":"admin-jianjun","sender_name":"jianjun","content":"hello","content_type":"text","created_at":1776672195272,"mentions":["agent-warhorse"]}

event: heartbeat
id: 102
data: {}
```

- `id` 字段 = events 表的 cursor 值，用于 `Last-Event-ID` 断线续传
- `data` 字段 = 与 poll 返回的 event.payload 格式一致
- 心跳事件的 `id` = 当前最新 cursor（断线重连时从这个位置续传）

#### 推送机制

复用现有的 `signalNewEvents()` 通知机制，扩展为同时通知 SSE 客户端：

```typescript
// stream.ts

interface SSEClient {
  userId: string;
  res: FastifyReply;       // 持有 HTTP response，用于 write
  heartbeatTimer: NodeJS.Timer;
  channelRefreshTimer: NodeJS.Timer;  // 60s 定时刷新
  lastCursor: number;      // 该客户端已发送到的 cursor
  cachedChannelIds: string[];  // 缓存的频道列表
  ready: boolean;          // 补发完成后才标记为 ready
}

const sseClients: SSEClient[] = [];

// 频道变更事件类型（不走 channelIds 过滤）
// 所有会改变用户可见频道集合的事件类型
const CHANNEL_CHANGE_KINDS = new Set([
  'member_joined', 'member_left',
  'channel_created', 'channel_deleted',
  'visibility_changed',  // 私有→公开，新用户可见
  'user_joined', 'user_left',  // 自助加入/离开公开频道
]);

export function notifySSEClients(): void {
  const db = getDb();
  for (const client of sseClients) {
    if (!client.ready) continue;
    
    try {
      // 单次 SQL 合并查询：channelIds 过滤的消息 + 不过滤的频道变更事件
      // SQL: WHERE cursor > ? AND (channel_id IN (...) OR kind IN (...))
      // 避免两次查询 + LIMIT 截断导致变更事件丢失
      const allEvents = Q.getEventsSinceWithChanges(
        db, client.lastCursor, 100, 
        client.cachedChannelIds, 
        Array.from(CHANNEL_CHANGE_KINDS)
      );
      
      for (const event of allEvents) {
        const payload = JSON.parse(event.payload);
        
        // 频道变更事件：检查是否与当前用户相关，同时刷新缓存
        // 注意：不同事件的 payload 结构不同：
        //   member_joined/member_left: { channel_id, user_id, display_name }
        //   channel_created: { channel: { id, name, created_by, ... } }
        //   channel_deleted: { channel_id }
        if (CHANNEL_CHANGE_KINDS.has(event.kind)) {
          const isRelevant = 
            payload.user_id === client.userId ||                    // member_joined/left/user_joined/user_left
            payload.channel?.created_by === client.userId ||        // channel_created (creator)
            client.cachedChannelIds.includes(event.channel_id) ||   // channel_deleted: 之前是成员（用缓存，因为 DB 里可能已删）
            getUserChannelIds(db, client.userId).includes(event.channel_id);  // fallback: am I now a member?
          
          if (isRelevant) {
            client.cachedChannelIds = getUserChannelIds(db, client.userId);
            client.res.raw.write(`event: ${event.kind}\nid: ${event.cursor}\ndata: ${event.payload}\n\n`);
          }
          client.lastCursor = event.cursor;
          continue;
        }
        
        // 跳过自己发的消息——但必须推进 cursor
        if (payload.sender_id === client.userId) {
          client.lastCursor = event.cursor;
          continue;
        }
        
        client.res.raw.write(`event: ${event.kind}\nid: ${event.cursor}\ndata: ${event.payload}\n\n`);
        client.lastCursor = event.cursor;
      }
    } catch {
      removeSSEClient(client);
    }
  }
}
```

**关键设计决策：缓存 + 事件驱动失效**

与长轮询的根本区别：长轮询在每次 poll 开始时缓存 channelIds，整个 poll 周期内用旧快照。SSE 改用**缓存 + 三层失效策略**：

1. **事件驱动**：频道变更事件（`member_joined`/`member_left`/`channel_created`/`channel_deleted`）不走 channelIds 过滤，直接推送给目标用户，同时刷新该 client 的 channelIds 缓存
2. **按需查询**：遇到不认识的 channel 或成员时主动查一次
3. **定时兜底**：每 60 秒定时刷新 channelIds 缓存，防止任何边界情况导致缓存不一致

频道变更类事件必须绕过 channelIds 过滤——否则用户加入新频道 C 的 `member_joined` 事件（channel_id=C）会被旧缓存过滤掉，导致缓存永远不更新。

#### 连接生命周期管理

```typescript
// 连接断开清理
client.res.raw.on('close', () => {
  clearInterval(client.heartbeatTimer);
  const idx = sseClients.indexOf(client);
  if (idx >= 0) sseClients.splice(idx, 1);
});

// 心跳（15 秒）
client.heartbeatTimer = setInterval(() => {
  try {
    client.res.raw.write(`event: heartbeat\nid: ${client.lastCursor}\ndata: {}\n\n`);
  } catch {
    removeSSEClient(client);
  }
}, 15_000);

// channelIds 缓存定时刷新（60 秒兜底）
client.channelRefreshTimer = setInterval(() => {
  try {
    const db = getDb();
    client.cachedChannelIds = getUserChannelIds(db, client.userId);
  } catch { /* ignore */ }
}, 60_000);
```

#### 集成到现有事件通知

当前 `signalNewEvents()` 在 `messages.ts` 路由里插入消息后调用。改造后：

```typescript
// poll.ts (保留)
export function signalNewEvents(): void {
  notifyWaiters();     // 通知长轮询 waiter（向后兼容）
  notifySSEClients();  // 通知 SSE 客户端（新增）
}
```

`signalNewEvents()` 保持为单一入口，同时通知两种客户端。

### Plugin 端改造

#### 新增 SSE 客户端

```
packages/plugin/src/sse-client.ts  (新文件)
```

核心逻辑：

```typescript
import { EventSource } from 'eventsource';  // Node.js 原生 (v22+)

function startSSE(account: ResolvedCollabAccount, ctx: PluginContext): void {
  const url = `${account.baseUrl}/api/v1/stream?api_key=${account.apiKey}`;
  const headers: Record<string, string> = {};
  
  // 从持久化 cursor 恢复（断线续传）
  const savedCursor = loadCursor(account);
  if (savedCursor > 0) {
    headers['Last-Event-ID'] = String(savedCursor);
  }
  
  const es = new EventSource(url, { headers });
  
  es.addEventListener('message', (event) => {
    const payload = JSON.parse(event.data);
    // 和现有 poll 处理逻辑一致
    handleInboundEvent(account, ctx, payload, event.lastEventId);
  });
  
  es.addEventListener('heartbeat', () => {
    // 更新 last_seen_at（保持在线状态）
    // cursor 不推进（心跳不是消息）
  });
  
  es.onerror = () => {
    // EventSource 自动重连（指数退避由实现处理）
    // 重连时自动发送 Last-Event-ID
  };
}
```

**Node.js EventSource**：Node.js 22+ 内置 `EventSource`（`globalThis.EventSource`），无需额外依赖。如果不可用，用 `eventsource` npm 包作 polyfill。

**注意**：Node.js 内置 EventSource 默认 3 秒重连间隔，无指数退避。为避免服务端重启时惊群效应，Plugin 应手动管理重连：监听 `error` 事件，关闭自动重连，自己实现指数退避（1s → 2s → 4s → ... → 60s 封顶）。

#### Gateway 改造

```typescript
// gateway.ts 改造

async function startAccountPoller(account, ctx) {
  // 1. 尝试 SSE
  try {
    await startSSE(account, ctx);
    return; // SSE 成功，不需要 poll
  } catch {
    console.log('[collab-plugin] SSE not available, falling back to poll');
  }
  
  // 2. Fallback 到长轮询（向后兼容）
  startPollLoop(account, ctx);
}
```

**降级策略**：
- Plugin 先 `HEAD /api/v1/stream`，404 → 直接用长轮询
- SSE 连接建立但 30 秒内未收到任何事件（含心跳） → 降级
- 401 → 不降级，直接报错停止
- Plugin 不需要配置切换

#### Cursor 持久化

SSE 模式下仍需持久化 cursor（用于进程重启后的 `Last-Event-ID`）：

```typescript
// 每收到一个 message 事件，持久化 cursor
es.addEventListener('message', (event) => {
  if (event.lastEventId) {
    saveCursor(account, parseInt(event.lastEventId, 10));
  }
  // ... dispatch
});
```

### 数据模型

**无 schema 变更**。SSE 复用现有 `events` 表和 `channel_members` 表。

### 错误处理

| 场景 | 处理 |
|------|------|
| SSE 连接被中间代理断开 | 心跳 15 秒保活；断开后 EventSource 自动重连 |
| Plugin 进程重启 | 从持久化 cursor 恢复，SSE 连接带 `Last-Event-ID` |
| 服务端重启 | SSE 连接断开 → Plugin 自动重连 → 补发重启期间的事件 |
| API key 无效/过期 | SSE 返回 401 → Plugin 停止重连（和长轮询行为一致） |
| 网络抖动 | EventSource 内置自动重连，指数退避 |
| Caddy 缓冲 SSE | 响应头 `X-Accel-Buffering: no` 禁用缓冲 |

### Caddy 配置

需要确保 Caddy 不缓冲 SSE 响应：

```caddyfile
collab.codetrek.cn {
    reverse_proxy localhost:4900 {
        flush_interval -1    # 禁用缓冲，立即转发
    }
}
```

当前 Caddyfile 如果没有 `flush_interval`，需要加上。SSE 响应头中的 `X-Accel-Buffering: no` 是额外保险。

## 备选方案

### 方案 B：WebSocket 双向通道

- **优点**：双向通信，可以把发消息也走 WS
- **不选的原因**：
  - 浏览器已经有 WebSocket，Plugin 再加一个增加复杂度
  - SSE 的单向推送完全满足 Plugin 需求（发消息走 REST 就够）
  - SSE 更简单：标准 HTTP、自动重连、`Last-Event-ID` 断线续传都是内置的
  - Plugin 端不需要维护心跳/ping-pong

### 方案 C：保持长轮询 + 优化

- **优点**：改动最小
- **不选的原因**：
  - 频道快照陈旧的根因无法彻底解决（每次 poll 都是新请求，必须缓存）
  - 延迟天花板在 timeout 窗口（30 秒）
  - cursor 管理的复杂度只会越来越高

## 测试策略

### 单元测试

| 模块 | 测试重点 |
|------|----------|
| `stream.ts` 路由 | API key 认证、Last-Event-ID 解析、SSE 响应头 |
| `notifySSEClients()` | channelIds 动态刷新、自身消息过滤、cursor 推进 |
| `sse-client.ts` | 事件解析、cursor 持久化、自动重连 |
| 降级逻辑 | SSE 失败 → 回退长轮询 |

### 集成测试

| 场景 | 验证点 |
|------|--------|
| 新消息推送 | 发消息 → SSE 客户端 < 1 秒收到 |
| 断线续传 | 断开 → 补发消息 → 重连后用 Last-Event-ID 拿到错过的 |
| 新频道加入 | 加入新频道 → 下一条该频道消息能收到 |
| 多客户端 | 3 个 agent 同时 SSE 连接，各自只收自己频道的消息 |
| 向后兼容 | 长轮询和 SSE 同时工作，互不干扰 |

### E2E 验收

1. Agent 通过 SSE 在 < 5 秒内回复 @mention
2. 手动断开 SSE → 重连 → 错过的消息补发
3. 新建私有频道 + 加 agent → 发消息 → agent 收到（无需 agent 重启）
4. 旧版 Plugin（长轮询）仍然正常工作

## Task Breakdown

| ID | 任务 | 依赖 | 估时 | 说明 |
|----|------|------|------|------|
| SSE-T00 | `getEventsSinceWithChanges` SQL 查询 + `channel_deleted` 事件 | — | 2h | queries.ts 新增合并查询函数（`WHERE cursor > ? AND (channel_id IN (...) OR kind IN (...))`）；channels.ts 删除频道时 `insertEvent('channel_deleted', ...)`；补齐缺失的事件 emit |
| SSE-T01a | 服务端 SSE 路由 + 认证 | T00 | 2h | `stream.ts`：路由注册、API key 认证（header + query param 脱敏）、SSE 响应头、心跳定时器、连接清理 |
| SSE-T01b | 补发循环 + 客户端注册 + drain | T01a | 2.5h | Last-Event-ID 解析、`getLatestCursor()` bootstrap、循环补发（含频道变更合并）、ready 标记 + 循环 drain 追尾、自身消息过滤 |
| SSE-T02 | 服务端事件推送 | T00, T01a | 2h | `notifySSEClients()`：用 `getEventsSinceWithChanges` 合并查询、isRelevant 判断（含 channel_deleted 缓存判断）、缓存刷新、集成到 `signalNewEvents()` |
| SSE-T03a | Plugin SSE 客户端核心 | — | 2h | `sse-client.ts`：EventSource 连接、事件解析、dispatch 到 inbound handler、cursor 持久化 |
| SSE-T03b | Plugin 重连状态机 | T03a | 1.5h | 手动管理重连：onerror → es.close() → 指数退避（1s→2s→4s→...→60s）→ 重建 EventSource（携带 Last-Event-ID）；不依赖 EventSource 自动重连 |
| SSE-T04 | Plugin 降级逻辑 | T03a | 1h | HEAD /api/v1/stream 探测 → 404 降级长轮询；SSE 30 秒无事件降级；401 不降级直接停止 |
| SSE-T05 | Caddy 配置 | — | 0.5h | `flush_interval -1`（staging + prod）|
| SSE-T06 | 单测 + 集成测试 | T00-T04 | 4h | 测试基建搭建（vitest/jest）+ 单测（getEventsSinceWithChanges、补发循环、isRelevant、重连状态机）+ 集成测试（消息推送、断线续传、新频道加入、多客户端） |
| SSE-T07 | 部署 staging + E2E 验收 | T00-T06 | 2h | 部署到 staging → QA E2E 全流程 → 通过后部署 prod |

**总估时：~19.5h**

**关键路径**：T00 → T01a → T01b + T02（并行）→ T03a → T03b → T04 → T06 → T07

**并行说明**：T03a/T03b（Plugin 端）可以和 T01/T02（服务端）并行开发

## 风险与开放问题

| 风险 | 影响 | 缓解 |
|------|------|------|
| Caddy 缓冲 SSE 响应 | 消息延迟 | `flush_interval -1` + `X-Accel-Buffering: no` |
| Node.js 22 EventSource 兼容性 | Plugin 无法使用原生 EventSource | Polyfill `eventsource` npm 包 |
| 阿里云防火墙/CDN 断长连接 | SSE 被提前断开 | 15 秒心跳保活 + 自动重连 |

**开放问题**：
1. ~~心跳间隔~~ → **已确认 15 秒**
2. ~~requireMention 过滤位置~~ → **已确认 Plugin 端**（服务端推全量）
3. SSE 连接数限制：当前 3 个 agent = 3 个 SSE 连接，没有问题。如果未来 agent 数量增长，考虑连接合并（多账户共用一个 SSE 连接 + 服务端路由）—— **v1 不做**

## 参考资料

- [SSE PRD](../requirements/sse-push.md)
- [Collab v1 技术设计](./technical-design-v1.md)
- [MDN: Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [Node.js EventSource](https://nodejs.org/api/globals.html#eventsource)
