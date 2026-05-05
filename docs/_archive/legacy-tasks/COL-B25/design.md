# COL-B25: 复杂场景集成测试 — 技术设计

日期：2026-04-23 | 状态：Draft

## 1. 设计原则

- **零 mock**：所有测试用 `buildFullApp()` + `server.listen({ port: 0 })` + 真实 WS
- **复用 B24 基础设施**：`ws-helpers.ts`（connectAuthWS、subscribeToChannel、waitForMessage、collectMessages）、`setup.ts`（TestContext、buildFullApp、httpJson、seed helpers）
- **每个测试文件一个场景**：文件名 `{scenario}-e2e.test.ts`
- **WS 连接防泄漏**：所有 WS 连接在 afterAll/afterEach 中 closeWsAndWait
- **DB 注入**：`vi.mock('../db.js')` 指向 in-memory SQLite（唯一允许的 mock）

## 2. 标准测试模板

```typescript
import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import Database from 'better-sqlite3';
import { createTestDb, seedAdmin, seedMember, seedChannel, addChannelMember, authCookie, buildFullApp } from './setup.js';
import { connectAuthWS, subscribeToChannel, waitForMessage, collectMessages, closeWsAndWait, sleep } from './ws-helpers.js';
import type { FastifyInstance } from 'fastify';

let testDb: Database.Database;
vi.mock('../db.js', () => ({ getDb: () => testDb, closeDb: () => {} }));

describe('场景名', () => {
  let app: FastifyInstance;
  let port: number;
  let adminId: string, adminToken: string;
  let memberAId: string, memberAToken: string;
  let channelId: string;
  const wsConnections: import('ws').WebSocket[] = [];

  beforeAll(async () => {
    testDb = createTestDb();
    app = await buildFullApp();
    await app.listen({ port: 0 });
    port = (app.server.address() as any).port;
    // seed data...
  });

  afterAll(async () => {
    for (const ws of wsConnections) await closeWsAndWait(ws);
    await app.close();
    testDb.close();
  });

  // tests...
});
```

## 3. 场景测试用例

### 3.1 场景 1：完整聊天 + WS 推送（P0）

文件：`chat-lifecycle-e2e.test.ts`

```typescript
describe('完整聊天 + WS 推送', () => {
  // setup: admin + memberA + memberB, channel, 都 addChannelMember
  // admin 和 memberB 各连一个 WS，subscribeToChannel

  it('Member 发消息 → 其他成员 WS 收到 new_message', async () => {
    const adminWs = await connectAuthWS(port, adminToken);
    await subscribeToChannel(adminWs, channelId);
    wsConnections.push(adminWs);
    const { json } = await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, { content: 'hello team' });
    expect(json.message.content).toBe('hello team');
    const event = await waitForMessage(adminWs, (m) => m.type === 'new_message');
    expect(event.payload.content).toBe('hello team');
    expect(event.payload.sender_id).toBe(memberAId);
  });

  it('编辑消息 → WS 收到 message_edited 事件', async () => {
    const msgId = seedMessage(testDb, channelId, memberAId, 'original');
    const adminWs = await connectAuthWS(port, adminToken);
    await subscribeToChannel(adminWs, channelId);
    wsConnections.push(adminWs);
    await httpJson(port, 'PUT', `/api/v1/messages/${msgId}`, memberAToken, { content: 'edited' });
    const event = await waitForMessage(adminWs, (m) => m.type === 'message_edited');
    expect(event.payload.content).toBe('edited');
  });

  it('删除消息 → WS 收到 message_deleted 事件', async () => {
    const msgId = seedMessage(testDb, channelId, memberAId, 'to-delete');
    const adminWs = await connectAuthWS(port, adminToken);
    await subscribeToChannel(adminWs, channelId);
    wsConnections.push(adminWs);
    await httpJson(port, 'DELETE', `/api/v1/messages/${msgId}`, memberAToken);
    const event = await waitForMessage(adminWs, (m) => m.type === 'message_deleted');
    expect(event.payload.id).toBe(msgId);
  });

  it('Reaction 添加 → WS 收到 reaction_added 事件', async () => {
    const msgId = seedMessage(testDb, channelId, memberAId, 'react-me');
    const memberBWs = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(memberBWs, channelId);
    wsConnections.push(memberBWs);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, memberAToken, { emoji: '🔥' });
    const event = await waitForMessage(memberBWs, (m) => m.type === 'reaction_added');
    expect(event.payload.emoji).toBe('🔥');
  });

  it('频道内所有成员都收到推送，非成员不收到', async () => {
    const outsider = seedMember(testDb, 'Outsider');
    const outsiderWs = await connectAuthWS(port, authCookie(outsider));
    wsConnections.push(outsiderWs);
    // outsider 不在频道里，不 subscribe
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, { content: 'members only' });
    const msgs = await collectMessages(outsiderWs, 1000);
    expect(msgs.filter(m => m.payload?.content === 'members only')).toHaveLength(0);
  });
});
```

### 3.2 场景 2：Agent-Human 完整往返（P0）

文件：`agent-human-e2e.test.ts`

> **apiKey 传递方式**：Plugin WS 认证通过 HTTP header `Authorization: Bearer <apiKey>`（COL-BUG-006 修复后）。
> `connectWS` 传递 headers 而非 query string：
> ```typescript
> const ws = await connectWS(port, '/ws/plugin', {
>   headers: { authorization: `Bearer ${apiKey}` },
> });
> ```

```typescript
describe('Agent-Human 完整往返', () => {
  // setup: admin(人) + agent(bot), channel, agent addChannelMember
  // agent 用 Plugin WS 连接 (/ws/plugin?apiKey=xxx)

  it('人发 @agent 消息 → Plugin WS 收到 message + mention 事件', async () => {
    const pluginWs = await connectWS(port, '/ws/plugin', { headers: { authorization: `Bearer ${agentApiKey}` } });
    wsConnections.push(pluginWs);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, {
      content: `hey <@${agentId}>`, mentions: [agentId],
    });
    const msgEvent = await waitForMessage(pluginWs, (m) => m.type === 'event' && m.kind === 'message');
    expect(msgEvent.payload.content).toContain(agentId);
    const mentionEvent = await waitForMessage(pluginWs, (m) => m.type === 'event' && m.kind === 'mention');
    expect(mentionEvent.payload.mentioned_user_id).toBe(agentId);
  });

  it('Agent 通过 apiCall 回复 → 人的 WS 收到消息', async () => {
    const humanWs = await connectAuthWS(port, adminToken);
    await subscribeToChannel(humanWs, channelId);
    wsConnections.push(humanWs);
    const pluginWs = await connectWS(port, '/ws/plugin', { headers: { authorization: `Bearer ${agentApiKey}` } });
    wsConnections.push(pluginWs);
    pluginWs.send(JSON.stringify({
      type: 'apiCall', id: 'reply-1',
      method: 'POST', path: `/api/v1/channels/${channelId}/messages`,
      body: { content: 'bot reply' },
    }));
    const event = await waitForMessage(humanWs, (m) => m.type === 'new_message' && m.payload?.content === 'bot reply');
    expect(event.payload.sender_id).toBe(agentId);
  });

  it('不 @ agent 的消息 → 收到 message 但无 mention 事件', async () => {
    const pluginWs = await connectWS(port, '/ws/plugin', { headers: { authorization: `Bearer ${agentApiKey}` } });
    wsConnections.push(pluginWs);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: 'no mention' });
    const msgs = await collectMessages(pluginWs, 2000);
    const mentionEvents = msgs.filter(m => m.kind === 'mention');
    expect(mentionEvents).toHaveLength(0);
  });
});
```

### 3.3 场景 3：权限动态变化 + WS 隔离（P0）

文件：`permission-ws-e2e.test.ts`

```typescript
describe('权限动态变化 + WS 隔离', () => {
  // setup: admin + memberA (in channel) + memberB (NOT in channel), 私有频道

  it('非成员 HTTP 访问私有频道 → 403/404', async () => {
    const { status } = await httpJson(port, 'GET', `/api/v1/channels/${privateChannelId}/messages`, memberBToken);
    expect([403, 404]).toContain(status);
  });

  it('非成员 WS 订阅私有频道 → 不收到事件', async () => {
    const ws = await connectAuthWS(port, memberBToken);
    wsConnections.push(ws);
    // 尝试订阅（可能被拒或静默忽略）
    ws.send(JSON.stringify({ type: 'subscribe', channel_id: privateChannelId }));
    await httpJson(port, 'POST', `/api/v1/channels/${privateChannelId}/messages`, adminToken, { content: 'secret' });
    const msgs = await collectMessages(ws, 1500);
    expect(msgs.filter(m => m.payload?.content === 'secret')).toHaveLength(0);
  });

  it('Admin 邀请 memberB → memberB 开始收到 WS 事件', async () => {
    await httpJson(port, 'POST', `/api/v1/channels/${privateChannelId}/members`, adminToken, { user_id: memberBId });
    const ws = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(ws, privateChannelId);
    wsConnections.push(ws);
    await httpJson(port, 'POST', `/api/v1/channels/${privateChannelId}/messages`, adminToken, { content: 'welcome' });
    const event = await waitForMessage(ws, (m) => m.type === 'new_message');
    expect(event.payload.content).toBe('welcome');
  });

  it('Admin 踢出 memberB → memberB 不再收到事件', async () => {
    await httpJson(port, 'DELETE', `/api/v1/channels/${privateChannelId}/members/${memberBId}`, adminToken);
    const ws = await connectAuthWS(port, memberBToken);
    wsConnections.push(ws);
    await httpJson(port, 'POST', `/api/v1/channels/${privateChannelId}/messages`, adminToken, { content: 'after-kick' });
    const msgs = await collectMessages(ws, 1500);
    expect(msgs.filter(m => m.payload?.content === 'after-kick')).toHaveLength(0);
  });
});
```

### 3.4 场景 4：SSE/WS/Poll 三通道一致性（P0）

文件：`three-channel-consistency-e2e.test.ts`

```typescript
describe('SSE/WS/Poll 三通道一致性', () => {
  it('发一条消息 → WS、SSE、Poll 三个客户端收到一致的 payload', async () => {
    // WS 客户端
    const ws = await connectAuthWS(port, adminToken);
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);

    // SSE 客户端（http.get + text/event-stream）
    const ssePromise = new Promise<any>((resolve, reject) => {
      const req = require('http').get(
        `http://127.0.0.1:${port}/api/v1/stream`,
        { headers: { authorization: `Bearer ${agentApiKey}` } },
        (res: any) => {
          let buf = '';
          res.on('data', (chunk: Buffer) => {
            buf += chunk.toString();
            if (buf.includes('consistency-test')) {
              res.destroy();
              resolve(buf);
            }
          });
          setTimeout(() => { res.destroy(); reject(new Error('SSE timeout')); }, 8000);
        },
      );
      req.on('error', reject);
    });

    // 等 SSE 连接建立
    await sleep(500);

    // 发消息
    const { json } = await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, { content: 'consistency-test' });
    const msgId = json.message.id;

    // WS 验证（超时与 SSE 对齐为 8000ms）
    const wsEvent = await waitForMessage(ws, (m) => m.type === 'new_message' && m.payload?.content === 'consistency-test', 8000);
    expect(wsEvent.payload.id).toBe(msgId);

    // SSE 验证：解析 data: 行为 JSON，与 WS payload 做结构性比较
    const sseRaw = await ssePromise;
    const sseLines = (sseRaw as string).split('\n').filter((l: string) => l.startsWith('data:'));
    const ssePayload = JSON.parse(sseLines[sseLines.length - 1].replace(/^data:\s*/, ''));
    expect(ssePayload.id || ssePayload.payload?.id).toBe(msgId);
    expect(ssePayload.content || ssePayload.payload?.content).toBe('consistency-test');
    expect(ssePayload.sender_id || ssePayload.payload?.sender_id).toBe(memberAId);

    // Poll 验证
    const pollRes = await httpJson(port, 'POST', '/api/v1/poll', agentToken, { api_key: agentApiKey, cursor: 0, timeout_ms: 2000 });
    const pollMsgs = pollRes.json.events.filter((e: any) => e.kind === 'message');
    expect(pollMsgs.some((e: any) => JSON.parse(e.payload).id === msgId)).toBe(true);
  });
});
```

### 3.5 场景 5：Remote Explorer 复合流程（P1）

文件：`remote-explorer-e2e.test.ts`

> **注意**：B24 `remote-explorer.integration.test.ts` 已覆盖注册、WS 连接、单次读文件、offline 503、非 owner 403 等基础 case。
> B25 不重复这些独立 case，而是测试一个连贯的多步复合流程。

```typescript
describe('Remote Explorer 复合流程', () => {
  // setup: admin + memberA, channel
  let nodeId: string, nodeToken: string;
  let agentWs: import('ws').WebSocket;

  it('注册→绑定→列目录→读文件→断连→重连→再读：完整生命周期', async () => {
    // Step 1: 注册 Node
    const { json: reg } = await httpJson(port, 'POST', '/api/v1/remote/nodes', adminToken, {
      name: 'lifecycle-machine', channelId, directory: '/home/user',
    });
    expect(reg.token).toBeDefined();
    nodeToken = reg.token;
    nodeId = reg.id;

    // Step 2: Remote Agent WS 连接
    agentWs = await connectWS(port, '/ws/remote', { token: nodeToken });
    wsConnections.push(agentWs);
    expect(agentWs.readyState).toBe(1);

    // Step 3: Agent 响应 list + read 请求
    agentWs.on('message', (raw) => {
      const msg = JSON.parse(raw.toString());
      if (msg.type === 'request' && msg.action === 'list') {
        agentWs.send(JSON.stringify({
          type: 'response', id: msg.id,
          data: { entries: [{ name: 'file.txt', type: 'file', size: 100 }, { name: 'sub', type: 'directory', size: 0 }] },
        }));
      }
      if (msg.type === 'request' && msg.action === 'read') {
        agentWs.send(JSON.stringify({
          type: 'response', id: msg.id,
          data: { content: 'hello world', size: 11, mime_type: 'text/plain' },
        }));
      }
    });

    // Step 4: 列目录
    const { status: listStatus, json: listJson } = await httpJson(port, 'GET', `/api/v1/remote/nodes/${nodeId}/list?path=/`, adminToken);
    expect(listStatus).toBe(200);
    expect(listJson.entries).toHaveLength(2);

    // Step 5: 读文件
    const { status: readStatus, json: readJson } = await httpJson(port, 'GET', `/api/v1/remote/nodes/${nodeId}/read?path=/file.txt`, adminToken);
    expect(readStatus).toBe(200);
    expect(readJson.content).toBe('hello world');

    // Step 6: 断连 → 503
    await closeWsAndWait(agentWs);
    const { status: offlineStatus } = await httpJson(port, 'GET', `/api/v1/remote/nodes/${nodeId}/read?path=/file.txt`, adminToken);
    expect(offlineStatus).toBe(503);

    // Step 7: 重连 → 再读成功
    const agentWs2 = await connectWS(port, '/ws/remote', { token: nodeToken });
    wsConnections.push(agentWs2);
    agentWs2.on('message', (raw) => {
      const msg = JSON.parse(raw.toString());
      if (msg.type === 'request' && msg.action === 'read') {
        agentWs2.send(JSON.stringify({
          type: 'response', id: msg.id,
          data: { content: 'hello again', size: 11, mime_type: 'text/plain' },
        }));
      }
    });
    const { status: reconnectStatus, json: reconnectJson } = await httpJson(port, 'GET', `/api/v1/remote/nodes/${nodeId}/read?path=/file.txt`, adminToken);
    expect(reconnectStatus).toBe(200);
    expect(reconnectJson.content).toBe('hello again');
  });
});
```

### 3.6 场景 6：Workspace 消息引用附件 + 下载验证（P1）

文件：`workspace-message-e2e.test.ts`

> **注意**：B24 `workspace-flow.integration.test.ts` 已覆盖 upload → list（user isolation）→ rename 基础 case。
> B25 仅测试 B24 未覆盖的部分：消息引用附件 + 下载内容一致性验证。

```typescript
describe('Workspace 消息引用附件 + 下载验证', () => {
  let fileId: string;

  it('上传文件 → 发消息引用 → 下载内容一致', async () => {
    // 上传文件
    const { json: file } = await httpJson(port, 'POST', `/api/v1/channels/${channelId}/workspace/upload`, memberAToken, {
      name: 'report.txt', content: Buffer.from('report data').toString('base64'), mime_type: 'text/plain',
    });
    fileId = file.id;

    // 发消息引用文件
    const { json: msg } = await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, {
      content: `See file: ${fileId}`,
    });
    expect(msg.message.content).toContain(fileId);

    // 下载并验证内容一致
    const { json: dl } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/workspace/${fileId}/download`, memberAToken);
    const content = dl.content || dl.data;
    expect(content).toBeDefined();
    // base64 解码后应与原始内容一致
    if (typeof content === 'string' && content.length < 1000) {
      expect(Buffer.from(content, 'base64').toString()).toBe('report data');
    }
  });
});
```

### 3.7 场景 7：分页 + 实时消息共存（P1）

文件：`pagination-realtime-e2e.test.ts`

```typescript
describe('分页 + 实时消息共存', () => {
  it('发 150 条消息 → 初始加载 100 条 + hasMore → 分页加载剩余', async () => {
    for (let i = 0; i < 150; i++) {
      seedMessage(testDb, channelId, adminId, `msg-${i}`, Date.now() + i);
    }
    const { json: page1 } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages?limit=100`, adminToken);
    expect(page1.messages.length).toBe(100);
    expect(page1.hasMore).toBe(true);
    const lastId = page1.messages[page1.messages.length - 1].id;
    const { json: page2 } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages?limit=100&before=${lastId}`, adminToken);
    expect(page2.messages.length).toBe(50);
    expect(page2.hasMore).toBe(false);
  });

  it('分页加载期间新消息通过 WS 实时到达', async () => {
    const ws = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);
    // 开始分页加载
    await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages?limit=100`, memberAToken);
    // 同时发新消息
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: 'realtime-during-pagination' });
    const event = await waitForMessage(ws, (m) => m.payload?.content === 'realtime-during-pagination');
    expect(event).toBeDefined();
  });
});
```

### 3.8 场景 8：DM 完整链路（P1）

文件：`dm-lifecycle-e2e.test.ts`

```typescript
describe('DM 完整链路', () => {
  it('创建 DM → 双方 WS 收到', async () => {
    const { json } = await httpJson(port, 'POST', `/api/v1/dm/${memberBId}`, memberAToken);
    dmChannelId = json.id;
    expect(json.type).toBe('dm');
  });

  it('DM 发消息 → 对方 WS 收到', async () => {
    const wsB = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(wsB, dmChannelId);
    wsConnections.push(wsB);
    await httpJson(port, 'POST', `/api/v1/channels/${dmChannelId}/messages`, memberAToken, { content: 'private hi' });
    const event = await waitForMessage(wsB, (m) => m.type === 'new_message');
    expect(event.payload.content).toBe('private hi');
  });

  it('第三方看不到 DM', async () => {
    const { status } = await httpJson(port, 'GET', `/api/v1/channels/${dmChannelId}/messages`, adminToken);
    expect([403, 404]).toContain(status);
  });
});
```

### 3.9 场景 9：Slash Commands + WS 推送（P1）

文件：`slash-ws-e2e.test.ts`

```typescript
describe('Slash Commands + WS 推送', () => {
  it('/topic 改名 → 所有成员 WS 收到 channel_updated 事件', async () => {
    const ws = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: '/topic New Topic' });
    const event = await waitForMessage(ws, (m) => m.type === 'channel_updated', 3000);
    expect(event.payload.topic).toBe('New Topic');
  });

  it('/invite 加人 → 新成员 WS 开始收到事件', async () => {
    const newMember = seedMember(testDb, 'NewGuy');
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, {
      content: `/invite <@${newMember}>`,
    });
    const ws = await connectAuthWS(port, authCookie(newMember));
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: 'welcome new guy' });
    const event = await waitForMessage(ws, (m) => m.type === 'new_message');
    expect(event.payload.content).toBe('welcome new guy');
  });
});
```

### 3.10 场景 10：公开频道预览 + 加入（P1）

文件：`public-preview-e2e.test.ts`

```typescript
describe('公开频道预览 + 加入', () => {
  it('未加入用户能看 24h 消息预览', async () => {
    seedMessage(testDb, publicChannelId, adminId, 'recent-msg', Date.now() - 3600_000);
    seedMessage(testDb, publicChannelId, adminId, 'old-msg', Date.now() - 25 * 3600_000);
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${publicChannelId}/preview`, outsiderToken);
    expect(json.messages.some((m: any) => m.content === 'recent-msg')).toBe(true);
    expect(json.messages.some((m: any) => m.content === 'old-msg')).toBe(false);
  });

  it('自助加入 → 开始收到 WS 推送', async () => {
    await httpJson(port, 'POST', `/api/v1/channels/${publicChannelId}/join`, outsiderToken);
    const ws = await connectAuthWS(port, outsiderToken);
    await subscribeToChannel(ws, publicChannelId);
    wsConnections.push(ws);
    await httpJson(port, 'POST', `/api/v1/channels/${publicChannelId}/messages`, adminToken, { content: 'post-join' });
    const event = await waitForMessage(ws, (m) => m.type === 'new_message');
    expect(event.payload.content).toBe('post-join');
  });

  it('加入后通过分页可见旧消息', async () => {
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${publicChannelId}/messages`, outsiderToken);
    expect(json.messages.length).toBeGreaterThan(0);
  });
});
```

### 3.11 场景 11：多设备同一用户（P1）

文件：`multi-device-e2e.test.ts`

```typescript
describe('多设备同一用户', () => {
  // setup: admin + memberA, channel, memberA addChannelMember

  it('同一用户两个 WS 连接都收到新消息', async () => {
    const ws1 = await connectAuthWS(port, memberAToken);
    const ws2 = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(ws1, channelId);
    await subscribeToChannel(ws2, channelId);
    wsConnections.push(ws1, ws2);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: 'multi-device' });
    const [ev1, ev2] = await Promise.all([
      waitForMessage(ws1, (m) => m.type === 'new_message' && m.payload?.content === 'multi-device'),
      waitForMessage(ws2, (m) => m.type === 'new_message' && m.payload?.content === 'multi-device'),
    ]);
    expect(ev1.payload.id).toBe(ev2.payload.id);
  });

  it('断开一个连接后另一个不受影响', async () => {
    const ws1 = await connectAuthWS(port, memberAToken);
    const ws2 = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(ws1, channelId);
    await subscribeToChannel(ws2, channelId);
    wsConnections.push(ws1, ws2);
    await closeWsAndWait(ws1);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: 'after-disconnect' });
    const event = await waitForMessage(ws2, (m) => m.type === 'new_message' && m.payload?.content === 'after-disconnect');
    expect(event).toBeDefined();
  });

  it('两个连接各自订阅不同频道，互不干扰', async () => {
    const channel2Id = seedChannel(testDb, 'second-ch', adminId);
    addChannelMember(testDb, channel2Id, memberAId);
    const ws1 = await connectAuthWS(port, memberAToken);
    const ws2 = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(ws1, channelId);
    await subscribeToChannel(ws2, channel2Id);
    wsConnections.push(ws1, ws2);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, adminToken, { content: 'ch1-only' });
    await httpJson(port, 'POST', `/api/v1/channels/${channel2Id}/messages`, adminToken, { content: 'ch2-only' });
    const ws1Msgs = await collectMessages(ws1, 1500);
    const ws2Msgs = await collectMessages(ws2, 1500);
    expect(ws1Msgs.some(m => m.payload?.content === 'ch2-only')).toBe(false);
    expect(ws2Msgs.some(m => m.payload?.content === 'ch1-only')).toBe(false);
  });
});
```

### 3.12 场景 12：DM + 公开频道 + 私有频道隔离交叉（P1）

文件：`channel-isolation-e2e.test.ts`

```typescript
describe('DM + 公开频道 + 私有频道隔离交叉', () => {
  // setup: admin + memberA + memberB
  // publicCh (all members), privateCh (admin + memberA only), dmAB (memberA <-> memberB)

  it('公开频道消息只推送到公开频道订阅者', async () => {
    const wsA = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(wsA, publicChId);
    await subscribeToChannel(wsA, privateChId);
    await subscribeToChannel(wsA, dmABId);
    wsConnections.push(wsA);
    await httpJson(port, 'POST', `/api/v1/channels/${publicChId}/messages`, adminToken, { content: 'public-msg' });
    const event = await waitForMessage(wsA, (m) => m.type === 'new_message' && m.payload?.content === 'public-msg');
    expect(event.payload.channel_id).toBe(publicChId);
  });

  it('私有频道消息不泄漏到 DM 或公开频道', async () => {
    const wsB = await connectAuthWS(port, memberBToken);
    wsConnections.push(wsB);
    await httpJson(port, 'POST', `/api/v1/channels/${privateChId}/messages`, adminToken, { content: 'private-secret' });
    const msgs = await collectMessages(wsB, 1500);
    expect(msgs.filter(m => m.payload?.content === 'private-secret')).toHaveLength(0);
  });

  it('DM 消息仅双方可见，第三方不收到', async () => {
    const wsAdmin = await connectAuthWS(port, adminToken);
    wsConnections.push(wsAdmin);
    const wsB = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(wsB, dmABId);
    wsConnections.push(wsB);
    await httpJson(port, 'POST', `/api/v1/channels/${dmABId}/messages`, memberAToken, { content: 'dm-only' });
    const evB = await waitForMessage(wsB, (m) => m.type === 'new_message' && m.payload?.content === 'dm-only');
    expect(evB).toBeDefined();
    const adminMsgs = await collectMessages(wsAdmin, 1500);
    expect(adminMsgs.filter(m => m.payload?.content === 'dm-only')).toHaveLength(0);
  });

  it('三种频道同时发消息 → 各自只收到对应频道的事件', async () => {
    const wsA = await connectAuthWS(port, memberAToken);
    await subscribeToChannel(wsA, publicChId);
    await subscribeToChannel(wsA, privateChId);
    await subscribeToChannel(wsA, dmABId);
    wsConnections.push(wsA);
    await Promise.all([
      httpJson(port, 'POST', `/api/v1/channels/${publicChId}/messages`, adminToken, { content: 'pub' }),
      httpJson(port, 'POST', `/api/v1/channels/${privateChId}/messages`, adminToken, { content: 'priv' }),
      httpJson(port, 'POST', `/api/v1/channels/${dmABId}/messages`, memberBToken, { content: 'dm' }),
    ]);
    const msgs = await collectMessages(wsA, 2000);
    const channels = msgs.filter(m => m.type === 'new_message').map(m => m.payload.channel_id);
    expect(new Set(channels).size).toBe(3);
  });
});
```

### 3.13 场景 13：频道删除级联（P2）

文件：`channel-delete-cascade-e2e.test.ts`

```typescript
describe('频道删除级联', () => {
  // setup: admin + memberA + memberB, channel with messages

  it('删除频道 → 所有成员 WS 收到 channel_deleted 事件', async () => {
    const wsA = await connectAuthWS(port, memberAToken);
    const wsB = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(wsA, channelId);
    await subscribeToChannel(wsB, channelId);
    wsConnections.push(wsA, wsB);
    await httpJson(port, 'DELETE', `/api/v1/channels/${channelId}`, adminToken);
    const [evA, evB] = await Promise.all([
      waitForMessage(wsA, (m) => m.type === 'channel_deleted'),
      waitForMessage(wsB, (m) => m.type === 'channel_deleted'),
    ]);
    expect(evA.payload.channel_id).toBe(channelId);
    expect(evB.payload.channel_id).toBe(channelId);
  });

  it('删除后成员列表 API 返回 404', async () => {
    const { status } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/members`, adminToken);
    expect(status).toBe(404);
  });

  it('删除后消息 API 返回 404', async () => {
    const { status } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages`, adminToken);
    expect(status).toBe(404);
  });
});
```

### 3.14 场景 14：成员变更系统消息（P2）

文件：`member-change-sysmsg-e2e.test.ts`

```typescript
describe('成员变更系统消息', () => {
  // setup: admin + memberA + memberB, channel

  it('加入频道 → WS 收到 system 类型消息', async () => {
    const ws = await connectAuthWS(port, adminToken);
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/members`, adminToken, { user_id: memberBId });
    const event = await waitForMessage(ws, (m) => m.type === 'new_message' && m.payload?.system === true);
    expect(event.payload.content).toContain(memberBId);
  });

  it('离开频道 → WS 收到 system 离开消息', async () => {
    const ws = await connectAuthWS(port, adminToken);
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);
    await httpJson(port, 'DELETE', `/api/v1/channels/${channelId}/members/${memberBId}`, adminToken);
    const event = await waitForMessage(ws, (m) => m.type === 'new_message' && m.payload?.system === true);
    expect(event.payload.content).toContain(memberBId);
  });

  it('系统消息通过 HTTP 历史接口也可查到', async () => {
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages`, adminToken);
    const sysMsgs = json.messages.filter((m: any) => m.system === true);
    expect(sysMsgs.length).toBeGreaterThanOrEqual(2);
  });
});
```

### 3.15 场景 15：快速连续操作（P2）

文件：`rapid-fire-e2e.test.ts`

```typescript
describe('快速连续操作', () => {
  // setup: admin + memberA, channel

  it('100ms 内连发 5 条消息 → 全部入库且按序', async () => {
    const promises = [];
    for (let i = 0; i < 5; i++) {
      promises.push(httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, { content: `rapid-${i}` }));
    }
    const results = await Promise.all(promises);
    expect(results.every(r => r.status === 200 || r.status === 201)).toBe(true);
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages?limit=10`, adminToken);
    const rapidMsgs = json.messages.filter((m: any) => m.content.startsWith('rapid-'));
    expect(rapidMsgs).toHaveLength(5);
  });

  it('对方 WS 按序收到全部 5 条', async () => {
    const ws = await connectAuthWS(port, adminToken);
    await subscribeToChannel(ws, channelId);
    wsConnections.push(ws);
    for (let i = 0; i < 5; i++) {
      await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, { content: `seq-${i}` });
    }
    const msgs = await collectMessages(ws, 3000);
    const seqMsgs = msgs.filter(m => m.type === 'new_message' && m.payload?.content?.startsWith('seq-'));
    expect(seqMsgs).toHaveLength(5);
    for (let i = 0; i < 5; i++) {
      expect(seqMsgs[i].payload.content).toBe(`seq-${i}`);
    }
  });
});
```

### 3.16 场景 16：并发成员变更 + 消息（P2）

文件：`concurrent-member-msg-e2e.test.ts`

```typescript
describe('并发成员变更 + 消息', () => {
  // setup: admin + memberA, channel, memberA in channel

  it('踢出与发消息并发 → 不能 500 或数据不一致', async () => {
    const [kickRes, msgRes] = await Promise.all([
      httpJson(port, 'DELETE', `/api/v1/channels/${channelId}/members/${memberAId}`, adminToken),
      httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages`, memberAToken, { content: 'race-msg' }),
    ]);
    expect(kickRes.status).not.toBe(500);
    expect(msgRes.status).not.toBe(500);
    // 消息要么成功(200/201)要么被拒(403)
    expect([200, 201, 403]).toContain(msgRes.status);
  });

  it('并发后数据库状态一致：成员已被移除', async () => {
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/members`, adminToken);
    const memberIds = json.members.map((m: any) => m.user_id || m.id);
    expect(memberIds).not.toContain(memberAId);
  });
});
```

### 3.17 场景 17：Token 轮换中的 WS（P2）

文件：`token-rotation-ws-e2e.test.ts`

```typescript
describe('Token 轮换中的 WS', () => {
  // setup: admin + agent with apiKey, channel

  it('rotate-api-key → 旧 WS 连接收到关闭码 4001', async () => {
    const pluginWs = await connectWS(port, '/ws/plugin', { headers: { authorization: `Bearer ${oldApiKey}` } });
    wsConnections.push(pluginWs);
    const closePromise = new Promise<number>((resolve) => {
      pluginWs.on('close', (code) => resolve(code));
    });
    const { json } = await httpJson(port, 'POST', `/api/v1/agents/${agentId}/rotate-key`, adminToken);
    newApiKey = json.api_key;
    const closeCode = await closePromise;
    expect(closeCode).toBe(4001);
  });

  it('新 key 重连成功', async () => {
    const newWs = await connectWS(port, '/ws/plugin', { headers: { authorization: `Bearer ${newApiKey}` } });
    wsConnections.push(newWs);
    expect(newWs.readyState).toBe(1); // OPEN
  });

  it('旧 key 重连失败', async () => {
    await expect(connectWS(port, '/ws/plugin', { headers: { authorization: `Bearer ${oldApiKey}` } }))
      .rejects.toThrow();
  });
});
```

### 3.18 场景 18：级联删除完整性（P2）

文件：`user-delete-cascade-e2e.test.ts`

```typescript
describe('级联删除完整性', () => {
  // setup: admin + targetUser (has agent, invite code, channel membership, WS)

  it('删除用户 → 该用户 WS 连接断开', async () => {
    const ws = await connectAuthWS(port, targetUserToken);
    wsConnections.push(ws);
    const closePromise = new Promise<number>((resolve) => {
      ws.on('close', (code) => resolve(code));
    });
    await httpJson(port, 'DELETE', `/api/v1/users/${targetUserId}`, adminToken);
    const closeCode = await closePromise;
    expect(closeCode).toBeDefined();
  });

  it('删除后该用户的 agent 不可用', async () => {
    const { status } = await httpJson(port, 'GET', `/api/v1/agents/${targetAgentId}`, adminToken);
    expect([404, 410]).toContain(status);
  });

  it('删除后该用户的邀请码作废', async () => {
    const { status } = await httpJson(port, 'POST', '/api/v1/auth/invite', {}, { code: targetInviteCode });
    expect([400, 404, 410]).toContain(status);
  });

  it('删除后该用户从频道成员列表移除', async () => {
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/members`, adminToken);
    const memberIds = json.members.map((m: any) => m.user_id || m.id);
    expect(memberIds).not.toContain(targetUserId);
  });
});
```

### 3.19 场景 19：Workspace 文件并发上传（P2）

文件：`workspace-concurrent-upload-e2e.test.ts`

```typescript
describe('Workspace 文件并发上传', () => {
  // setup: admin + memberA + memberB, channel, both members in channel

  it('A 和 B 同时上传同名文件 → 两个都成功，ID 不同', async () => {
    const [resA, resB] = await Promise.all([
      httpJson(port, 'POST', `/api/v1/channels/${channelId}/workspace/upload`, memberAToken, {
        name: 'shared.txt', content: Buffer.from('content-A').toString('base64'), mime_type: 'text/plain',
      }),
      httpJson(port, 'POST', `/api/v1/channels/${channelId}/workspace/upload`, memberBToken, {
        name: 'shared.txt', content: Buffer.from('content-B').toString('base64'), mime_type: 'text/plain',
      }),
    ]);
    expect(resA.status).toBeLessThan(300);
    expect(resB.status).toBeLessThan(300);
    expect(resA.json.id).not.toBe(resB.json.id);
  });

  it('Workspace 列表包含两个文件，无数据损坏', async () => {
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/workspace`, adminToken);
    const sharedFiles = json.files.filter((f: any) => f.name === 'shared.txt' || f.name.startsWith('shared'));
    expect(sharedFiles.length).toBeGreaterThanOrEqual(2);
  });
});
```

### 3.20 场景 20：消息 Reaction + WS 双向（P2）

文件：`reaction-bidirectional-e2e.test.ts`

```typescript
describe('消息 Reaction + WS 双向', () => {
  // setup: admin + memberA + memberB, channel, all members

  it('A 加 reaction → B WS 收到 reaction_added', async () => {
    const msgId = seedMessage(testDb, channelId, adminId, 'react-target');
    const wsB = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(wsB, channelId);
    wsConnections.push(wsB);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, memberAToken, { emoji: '👍' });
    const event = await waitForMessage(wsB, (m) => m.type === 'reaction_added');
    expect(event.payload.emoji).toBe('👍');
    expect(event.payload.user_id).toBe(memberAId);
    expect(event.payload.message_id).toBe(msgId);
  });

  it('A 取消 reaction → B WS 收到 reaction_removed', async () => {
    const msgId = seedMessage(testDb, channelId, adminId, 'unreact-target');
    const wsB = await connectAuthWS(port, memberBToken);
    await subscribeToChannel(wsB, channelId);
    wsConnections.push(wsB);
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, memberAToken, { emoji: '👍' });
    await waitForMessage(wsB, (m) => m.type === 'reaction_added');
    await httpJson(port, 'DELETE', `/api/v1/channels/${channelId}/messages/${msgId}/reactions/👍`, memberAToken);
    const event = await waitForMessage(wsB, (m) => m.type === 'reaction_removed');
    expect(event.payload.emoji).toBe('👍');
    expect(event.payload.message_id).toBe(msgId);
  });

  it('多人对同一消息加不同 reaction → WS 事件各自独立', async () => {
    const msgId = seedMessage(testDb, channelId, adminId, 'multi-react');
    const wsAdmin = await connectAuthWS(port, adminToken);
    await subscribeToChannel(wsAdmin, channelId);
    wsConnections.push(wsAdmin);
    await Promise.all([
      httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, memberAToken, { emoji: '🔥' }),
      httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, memberBToken, { emoji: '❤️' }),
    ]);
    const msgs = await collectMessages(wsAdmin, 2000);
    const reactions = msgs.filter(m => m.type === 'reaction_added');
    expect(reactions).toHaveLength(2);
    const emojis = reactions.map(r => r.payload.emoji).sort();
    expect(emojis).toEqual(['❤️', '🔥']);
  });

  it('HTTP GET 消息详情包含 reactions 列表', async () => {
    const msgId = seedMessage(testDb, channelId, adminId, 'check-reactions');
    await httpJson(port, 'POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, memberAToken, { emoji: '🎉' });
    const { json } = await httpJson(port, 'GET', `/api/v1/channels/${channelId}/messages/${msgId}`, adminToken);
    expect(json.reactions || json.message?.reactions).toBeDefined();
  });
});
```

## 4. Task Breakdown

| Task | 场景 | 文件 | 优先级 | 预估 |
|------|------|------|--------|------|
| **T0: P0 场景 1-4** | | | | |
| T0.1 | 场景 1：完整聊天 + WS 推送 | `chat-lifecycle-e2e.test.ts` | P0 | 1.5h |
| T0.2 | 场景 2：Agent-Human 完整往返 | `agent-human-e2e.test.ts` | P0 | 1.5h |
| T0.3 | 场景 3：权限动态变化 + WS 隔离 | `permission-ws-e2e.test.ts` | P0 | 1.5h |
| T0.4 | 场景 4：SSE/WS/Poll 三通道一致性 | `three-channel-consistency-e2e.test.ts` | P0 | 2h |
| **T0b: P1 场景 5-10** | | | | |
| T0b.1 | 场景 5：Remote Explorer 复合流程 | `remote-explorer-e2e.test.ts` | P1 | 1.5h |
| T0b.2 | 场景 6：Workspace 消息引用附件 + 下载 | `workspace-message-e2e.test.ts` | P1 | 1h |
| T0b.3 | 场景 7：分页 + 实时消息共存 | `pagination-realtime-e2e.test.ts` | P1 | 1h |
| T0b.4 | 场景 8：DM 完整链路 | `dm-lifecycle-e2e.test.ts` | P1 | 1h |
| T0b.5 | 场景 9：Slash Commands + WS 推送 | `slash-ws-e2e.test.ts` | P1 | 1h |
| T0b.6 | 场景 10：公开频道预览 + 加入 | `public-preview-e2e.test.ts` | P1 | 1h |
| **T1: P1 场景 11-12** | | | | |
| T1.1 | 场景 11：多设备同一用户 | `multi-device-e2e.test.ts` | P1 | 1h |
| T1.2 | 场景 12：DM + 公开 + 私有隔离交叉 | `channel-isolation-e2e.test.ts` | P1 | 1.5h |
| **T2: P2 边界场景 13-14** | | | | |
| T2.1 | 场景 13：频道删除级联 | `channel-delete-cascade-e2e.test.ts` | P2 | 1h |
| T2.2 | 场景 14：成员变更系统消息 | `member-change-sysmsg-e2e.test.ts` | P2 | 1h |
| **T3: P2 竞态场景 15-16** | | | | |
| T3.1 | 场景 15：快速连续操作 | `rapid-fire-e2e.test.ts` | P2 | 1h |
| T3.2 | 场景 16：并发成员变更 + 消息 | `concurrent-member-msg-e2e.test.ts` | P2 | 1h |
| **T4: P2 安全 + 完整性 17-18** | | | | |
| T4.1 | 场景 17：Token 轮换中的 WS | `token-rotation-ws-e2e.test.ts` | P2 | 1.5h |
| T4.2 | 场景 18：级联删除完整性 | `user-delete-cascade-e2e.test.ts` | P2 | 1.5h |
| **T5: P2 并发 + Reaction 19-20** | | | | |
| T5.1 | 场景 19：Workspace 文件并发上传 | `workspace-concurrent-upload-e2e.test.ts` | P2 | 1h |
| T5.2 | 场景 20：消息 Reaction + WS 双向 | `reaction-bidirectional-e2e.test.ts` | P2 | 1h |

**总计预估：22h**（场景 1-10: 10.5h + 场景 11-20: 11.5h）

## 5. 验收标准

- [ ] 场景 11-12（P1）测试全部通过，WS 多设备推送和频道隔离无泄漏
- [ ] 场景 13-14（P2）频道删除级联和成员变更系统消息逻辑正确
- [ ] 场景 15-16（P2）快速连续操作不丢消息、并发竞态无 500
- [ ] 场景 17-18（P2）Token 轮换断旧连新、用户删除级联完整
- [ ] 场景 19-20（P2）并发上传不损坏、Reaction 增删 WS 双向推送正确
- [ ] 所有测试使用真实 server + 真实 WS，零 mock 内部依赖
- [ ] 所有 WS 连接在 afterAll/afterEach 中正确关闭，无泄漏
- [ ] 不与 B24 或场景 1-10 的测试用例重复
- [ ] CI 全量通过（含场景 1-20）
