# COL-B24: 集成测试覆盖 — 技术设计 v4

日期：2026-04-23 | 状态：Draft

## 1. 技术决策

### 1.1 框架选择
保留 Vitest + Fastify inject。理由：已有 200+ 测试基础，inject 不起 HTTP server 零端口冲突。
WS 测试用 `ws` 包 + 真实 server（`server.listen({ port: 0 })`），OS 分配随机端口。

### 1.2 测试数据隔离
每个测试文件独立 in-memory SQLite DB（`new Database(':memory:')`）。不用 beforeEach 清数据。

### 1.3 多用户模拟

封装 `TestContext` helper：

```typescript
class TestContext {
  app: FastifyInstance;
  db: Database;
  admin: { id: string; token: string };
  memberA: { id: string; token: string };
  memberB: { id: string; token: string };
  agent: { id: string; apiKey: string; ownerId: string };
  channel: { id: string };

  static async create(opts?: { routes?: (app: FastifyInstance) => void }): Promise<TestContext> {
    const ctx = new TestContext();
    ctx.db = createTestDb();
    ctx.app = Fastify({ logger: false });
    // 注入 db mock
    ctx.app.decorate('testDb', ctx.db);
    if (opts?.routes) opts.routes(ctx.app);
    await ctx.app.ready();
    ctx.admin = { id: seedAdmin(ctx.db), token: '' };
    ctx.admin.token = authCookie(ctx.admin.id);
    ctx.memberA = { id: seedMember(ctx.db, 'MemberA'), token: '' };
    ctx.memberA.token = authCookie(ctx.memberA.id);
    ctx.memberB = { id: seedMember(ctx.db, 'MemberB'), token: '' };
    ctx.memberB.token = authCookie(ctx.memberB.id);
    ctx.agent = { id: seedAgent(ctx.db, ctx.admin.id), apiKey: '', ownerId: ctx.admin.id };
    ctx.agent.apiKey = ctx.db.prepare('SELECT api_key FROM users WHERE id = ?').get(ctx.agent.id).api_key;
    ctx.channel = { id: seedChannel(ctx.db, ctx.admin.id) };
    addChannelMember(ctx.db, ctx.channel.id, ctx.memberA.id);
    addChannelMember(ctx.db, ctx.channel.id, ctx.memberB.id);
    return ctx;
  }

  async inject(method: string, url: string, token: string, body?: any) {
    return this.app.inject({
      method: method as any, url,
      payload: body,
      headers: { cookie: token },
    });
  }

  async close() { await this.app.close(); this.db.close(); }
}
```

### 1.4 WS 测试 Helper

```typescript
async function connectWS(port: number, path: string, query?: Record<string, string>): Promise<WebSocket> {
  const qs = query ? '?' + new URLSearchParams(query).toString() : '';
  const ws = new WebSocket(`ws://localhost:${port}${path}${qs}`);
  await new Promise((resolve, reject) => {
    ws.on('open', resolve);
    ws.on('error', reject);
  });
  return ws;
}

function waitForMessage(ws: WebSocket, filter?: (msg: any) => boolean): Promise<any> {
  return new Promise((resolve) => {
    ws.on('message', (raw) => {
      const msg = JSON.parse(raw.toString());
      if (!filter || filter(msg)) resolve(msg);
    });
  });
}

async function waitForClose(ws: WebSocket): Promise<number> {
  return new Promise((resolve) => ws.on('close', (code) => resolve(code)));
}
```

## 2. 场景测试用例

### 2.1 场景 1：认证流程

测试文件：`auth-flow.test.ts`

```typescript
describe('认证流程', () => {
  let ctx: TestContext;
  beforeAll(async () => { ctx = await TestContext.create({ routes: (app) => { registerAuthRoutes(app); }}); });
  afterAll(() => ctx.close());

  it('注册 → 有效邀请码 → 201 + 用户信息 + JWT cookie', async () => {
    const code = seedInviteCode(ctx.db, ctx.admin.id, 'INVITE001');
    seedChannel(ctx.db, ctx.admin.id, 'general');
    const res = await ctx.app.inject({
      method: 'POST', url: '/api/v1/auth/register',
      payload: { invite_code: 'INVITE001', email: 'new@test.com', password: 'pass123', display_name: 'New' },
    });
    expect(res.statusCode).toBe(201);
    expect(res.json().user.display_name).toBe('New');
    expect(res.headers['set-cookie']).toContain('collab_token=');
  });

  it('注册 → 无效邀请码 → 400', async () => {
    const res = await ctx.app.inject({
      method: 'POST', url: '/api/v1/auth/register',
      payload: { invite_code: 'BADCODE', email: 'x@test.com', password: 'pass', display_name: 'X' },
    });
    expect(res.statusCode).toBe(400);
  });

  it('注册 → 已使用的邀请码 → 400', async () => {
    const code = seedInviteCode(ctx.db, ctx.admin.id, 'USED001');
    ctx.db.prepare('UPDATE invite_codes SET used_by = ?, used_at = ? WHERE code = ?')
      .run(ctx.memberA.id, Date.now(), 'USED001');
    const res = await ctx.app.inject({
      method: 'POST', url: '/api/v1/auth/register',
      payload: { invite_code: 'USED001', email: 'y@test.com', password: 'pass', display_name: 'Y' },
    });
    expect(res.statusCode).toBe(400);
  });

  it('登录 → 正确密码 → 200 + JWT cookie', async () => {
    // 先注册
    const code = seedInviteCode(ctx.db, ctx.admin.id, 'LOGIN001');
    await ctx.app.inject({
      method: 'POST', url: '/api/v1/auth/register',
      payload: { invite_code: 'LOGIN001', email: 'login@test.com', password: 'mypass', display_name: 'LoginUser' },
    });
    const res = await ctx.app.inject({
      method: 'POST', url: '/api/v1/auth/login',
      payload: { email: 'login@test.com', password: 'mypass' },
    });
    expect(res.statusCode).toBe(200);
    expect(res.headers['set-cookie']).toContain('collab_token=');
  });

  it('登录 → 错误密码 → 401', async () => {
    const res = await ctx.app.inject({
      method: 'POST', url: '/api/v1/auth/login',
      payload: { email: 'login@test.com', password: 'wrong' },
    });
    expect(res.statusCode).toBe(401);
  });

  it('API Key 认证 → Agent 用 api_key 访问 → 200', async () => {
    const res = await ctx.app.inject({
      method: 'GET', url: '/api/v1/channels',
      headers: { 'x-api-key': ctx.agent.apiKey },
    });
    expect(res.statusCode).toBe(200);
  });

  it('过期/无效 token → 401', async () => {
    const res = await ctx.app.inject({
      method: 'GET', url: '/api/v1/channels',
      headers: { cookie: 'collab_token=invalid.jwt.token' },
    });
    expect(res.statusCode).toBe(401);
  });
});
```

### 2.2 场景 2：频道生命周期

测试文件：`channel-lifecycle.test.ts`

```typescript
describe('频道生命周期', () => {
  let ctx: TestContext;
  beforeAll(async () => { ctx = await TestContext.create({ routes: (app) => { registerChannelRoutes(app); registerMessageRoutes(app); }}); });
  afterAll(() => ctx.close());

  it('admin 创建公开频道 → 201', async () => {
    const res = await ctx.inject('POST', '/api/v1/channels', ctx.admin.token, { name: 'pub-ch', visibility: 'public' });
    expect(res.statusCode).toBe(201);
    expect(res.json().visibility).toBe('public');
  });

  it('admin 创建私有频道 → 201', async () => {
    const res = await ctx.inject('POST', '/api/v1/channels', ctx.admin.token, { name: 'priv-ch', visibility: 'private' });
    expect(res.statusCode).toBe(201);
    expect(res.json().visibility).toBe('private');
  });

  it('member 加入公开频道 → 200', async () => {
    const chId = seedChannel(ctx.db, ctx.admin.id, 'join-test');
    const res = await ctx.inject('POST', `/api/v1/channels/${chId}/join`, ctx.memberA.token);
    expect(res.statusCode).toBe(200);
  });

  it('频道内发消息 → 201 + 消息内容', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/messages`, ctx.memberA.token, { content: 'hello world' });
    expect(res.statusCode).toBe(201);
    expect(res.json().content).toBe('hello world');
  });

  it('软删除频道 → admin 200, member 403', async () => {
    const chId = seedChannel(ctx.db, ctx.admin.id, 'del-test');
    const res1 = await ctx.inject('DELETE', `/api/v1/channels/${chId}`, ctx.memberA.token);
    expect(res1.statusCode).toBe(403);
    const res2 = await ctx.inject('DELETE', `/api/v1/channels/${chId}`, ctx.admin.token);
    expect(res2.statusCode).toBe(200);
  });

  it('公开频道预览 → 未加入用户看到最近 24h 消息', async () => {
    const chId = seedChannel(ctx.db, ctx.admin.id, 'preview-ch');
    const now = Date.now();
    seedMessage(ctx.db, chId, ctx.admin.id, 'recent', now - 3600_000);
    seedMessage(ctx.db, chId, ctx.admin.id, 'old', now - 25 * 3600_000);
    const res = await ctx.inject('GET', `/api/v1/channels/${chId}/preview`, ctx.memberB.token);
    expect(res.statusCode).toBe(200);
    const msgs = res.json().messages;
    expect(msgs.some((m: any) => m.content === 'recent')).toBe(true);
    expect(msgs.some((m: any) => m.content === 'old')).toBe(false);
  });

  it('多频道隔离 → A 频道消息不出现在 B 频道', async () => {
    const chA = seedChannel(ctx.db, ctx.admin.id, 'iso-a');
    const chB = seedChannel(ctx.db, ctx.admin.id, 'iso-b');
    seedMessage(ctx.db, chA, ctx.admin.id, 'msg-in-A');
    const res = await ctx.inject('GET', `/api/v1/channels/${chB}/messages`, ctx.admin.token);
    const msgs = res.json().messages || [];
    expect(msgs.find((m: any) => m.content === 'msg-in-A')).toBeUndefined();
  });

  it('DM 创建 → 只有两人可见', async () => {
    const res = await ctx.inject('POST', '/api/v1/dm', ctx.memberA.token, { userId: ctx.memberB.id });
    expect(res.statusCode).toBe(201);
    const dmId = res.json().id;
    // memberB 能看到
    const res2 = await ctx.inject('GET', `/api/v1/channels/${dmId}`, ctx.memberB.token);
    expect(res2.statusCode).toBe(200);
    // admin（非参与者）看不到 DM 内容（或 403）
  });

  it('踢出成员 → admin 移除成员后该成员无法访问频道', async () => {
    const chId = seedChannel(ctx.db, ctx.admin.id, 'kick-test');
    addChannelMember(ctx.db, chId, ctx.memberA.id);
    // admin 踢出 memberA
    const res1 = await ctx.inject('DELETE', `/api/v1/channels/${chId}/members/${ctx.memberA.id}`, ctx.admin.token);
    expect(res1.statusCode).toBe(200);
    // memberA 无法访问频道
    const res2 = await ctx.inject('GET', `/api/v1/channels/${chId}/messages`, ctx.memberA.token);
    expect(res2.statusCode).toBe(403);
    // memberA 不在成员列表中
    const member = ctx.db.prepare('SELECT * FROM channel_members WHERE channel_id = ? AND user_id = ?').get(chId, ctx.memberA.id);
    expect(member).toBeUndefined();
  });
});
```

### 2.3 场景 3：权限体系

测试文件：`permissions.test.ts`

```typescript
describe('权限体系', () => {
  let ctx: TestContext;
  beforeAll(async () => { ctx = await TestContext.create({ routes: (app) => { registerChannelRoutes(app); registerMessageRoutes(app); registerAdminRoutes(app); }}); });
  afterAll(() => ctx.close());

  it('admin 创建频道 → 200', async () => {
    const res = await ctx.inject('POST', '/api/v1/channels', ctx.admin.token, { name: 'perm-test' });
    expect(res.statusCode).toBe(201);
  });

  it('member 创建频道 → 403', async () => {
    const res = await ctx.inject('POST', '/api/v1/channels', ctx.memberA.token, { name: 'no-perm' });
    expect(res.statusCode).toBe(403);
  });

  it('member 删除自己的消息 → 200', async () => {
    const msgId = seedMessage(ctx.db, ctx.channel.id, ctx.memberA.id, 'my msg');
    const res = await ctx.inject('DELETE', `/api/v1/channels/${ctx.channel.id}/messages/${msgId}`, ctx.memberA.token);
    expect(res.statusCode).toBe(200);
  });

  it('member 删除别人的消息 → 403', async () => {
    const msgId = seedMessage(ctx.db, ctx.channel.id, ctx.memberB.id, 'not mine');
    const res = await ctx.inject('DELETE', `/api/v1/channels/${ctx.channel.id}/messages/${msgId}`, ctx.memberA.token);
    expect(res.statusCode).toBe(403);
  });

  it('admin 删除任何人的消息 → 200', async () => {
    const msgId = seedMessage(ctx.db, ctx.channel.id, ctx.memberA.id, 'admin can delete');
    const res = await ctx.inject('DELETE', `/api/v1/channels/${ctx.channel.id}/messages/${msgId}`, ctx.admin.token);
    expect(res.statusCode).toBe(200);
  });

  it('Agent owner 管理 agent → 200, 非 owner → 403', async () => {
    // admin 是 agent 的 owner
    const res1 = await ctx.inject('PATCH', `/api/v1/agents/${ctx.agent.id}`, ctx.admin.token, { display_name: 'Updated' });
    expect(res1.statusCode).toBe(200);
    // memberA 不是 owner
    const res2 = await ctx.inject('PATCH', `/api/v1/agents/${ctx.agent.id}`, ctx.memberA.token, { display_name: 'Hijack' });
    expect(res2.statusCode).toBe(403);
  });

  it('member 不能删除频道 → 403', async () => {
    const res = await ctx.inject('DELETE', `/api/v1/channels/${ctx.channel.id}`, ctx.memberA.token);
    expect(res.statusCode).toBe(403);
  });

  it('跨用户消息可见性 → 私有频道非成员看不到消息', async () => {
    const privCh = seedChannel(ctx.db, ctx.admin.id, 'priv-vis-test', 'private');
    addChannelMember(ctx.db, privCh, ctx.memberA.id);
    seedMessage(ctx.db, privCh, ctx.memberA.id, 'secret msg');
    // memberB 非成员，无法读取
    const res = await ctx.inject('GET', `/api/v1/channels/${privCh}/messages`, ctx.memberB.token);
    expect(res.statusCode).toBe(403);
  });
});
```

### 2.4 场景 4：消息系统

测试文件：`message-system.test.ts`

```typescript
describe('消息系统', () => {
  let ctx: TestContext;
  let channelId: string;
  beforeAll(async () => {
    ctx = await TestContext.create({ routes: (app) => { registerMessageRoutes(app); registerReactionRoutes(app); }});
    channelId = ctx.channel.id;
  });
  afterAll(() => ctx.close());

  it('发送消息 → 201 + sender_id + content', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${channelId}/messages`, ctx.memberA.token, { content: 'hello' });
    expect(res.statusCode).toBe(201);
    expect(res.json().sender_id).toBe(ctx.memberA.id);
    expect(res.json().content).toBe('hello');
  });

  it('编辑自己的消息 → content 更新 + editedAt 设置', async () => {
    const msgId = seedMessage(ctx.db, channelId, ctx.memberA.id, 'original');
    const res = await ctx.inject('PATCH', `/api/v1/channels/${channelId}/messages/${msgId}`, ctx.memberA.token, { content: 'edited' });
    expect(res.statusCode).toBe(200);
    expect(res.json().content).toBe('edited');
    expect(res.json().edited_at).toBeDefined();
  });

  it('编辑别人的消息 → 403', async () => {
    const msgId = seedMessage(ctx.db, channelId, ctx.memberA.id, 'not yours');
    const res = await ctx.inject('PATCH', `/api/v1/channels/${channelId}/messages/${msgId}`, ctx.memberB.token, { content: 'hijack' });
    expect(res.statusCode).toBe(403);
  });

  it('删除消息 → 软删除（deleted_at 设置）', async () => {
    const msgId = seedMessage(ctx.db, channelId, ctx.memberA.id, 'to delete');
    await ctx.inject('DELETE', `/api/v1/channels/${channelId}/messages/${msgId}`, ctx.memberA.token);
    const row = ctx.db.prepare('SELECT deleted_at FROM messages WHERE id = ?').get(msgId);
    expect(row.deleted_at).toBeDefined();
  });

  it('@mention → mentions 表写入', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${channelId}/messages`, ctx.memberA.token, {
      content: `hello <@${ctx.memberB.id}>`, mentions: [ctx.memberB.id],
    });
    expect(res.statusCode).toBe(201);
    const mention = ctx.db.prepare('SELECT * FROM mentions WHERE user_id = ? AND message_id = ?').get(ctx.memberB.id, res.json().id);
    expect(mention).toBeDefined();
  });

  it('Reaction 增删 → 添加 200, 重复 409, 删除 200', async () => {
    const msgId = seedMessage(ctx.db, channelId, ctx.memberA.id, 'react me');
    const r1 = await ctx.inject('POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, ctx.memberA.token, { emoji: '👍' });
    expect(r1.statusCode).toBe(201);
    const r2 = await ctx.inject('POST', `/api/v1/channels/${channelId}/messages/${msgId}/reactions`, ctx.memberA.token, { emoji: '👍' });
    expect(r2.statusCode).toBe(409);
    const r3 = await ctx.inject('DELETE', `/api/v1/channels/${channelId}/messages/${msgId}/reactions/👍`, ctx.memberA.token);
    expect(r3.statusCode).toBe(200);
  });

  it('分页 → limit + before cursor + hasMore', async () => {
    // 插入 15 条消息
    const ids: string[] = [];
    for (let i = 0; i < 15; i++) {
      ids.push(seedMessage(ctx.db, channelId, ctx.memberA.id, `msg-${i}`, Date.now() + i));
    }
    const r1 = await ctx.inject('GET', `/api/v1/channels/${channelId}/messages?limit=10`, ctx.memberA.token);
    expect(r1.json().messages.length).toBe(10);
    expect(r1.json().hasMore).toBe(true);
    const lastId = r1.json().messages[9].id;
    const r2 = await ctx.inject('GET', `/api/v1/channels/${channelId}/messages?limit=10&before=${lastId}`, ctx.memberA.token);
    expect(r2.json().messages.length).toBe(5);
    expect(r2.json().hasMore).toBe(false);
  });

  it('系统消息 → type=system + sender_id=null', async () => {
    const sysId = seedMessage(ctx.db, channelId, null, 'User joined', Date.now(), 'system');
    const res = await ctx.inject('GET', `/api/v1/channels/${channelId}/messages?limit=50`, ctx.admin.token);
    const sysMsg = res.json().messages.find((m: any) => m.id === sysId);
    expect(sysMsg).toBeDefined();
    expect(sysMsg.type).toBe('system');
    expect(sysMsg.sender_id).toBeNull();
  });

  it('附件自动存入 → 发送带附件消息后可通过消息 ID 获取附件', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${channelId}/messages`, ctx.memberA.token, {
      content: 'see attachment',
      attachments: [{ name: 'doc.pdf', content: Buffer.from('pdf data').toString('base64'), mime_type: 'application/pdf' }],
    });
    expect(res.statusCode).toBe(201);
    const msgId = res.json().id;
    const attachments = ctx.db.prepare('SELECT * FROM attachments WHERE message_id = ?').all(msgId);
    expect(attachments.length).toBe(1);
    expect(attachments[0].name).toBe('doc.pdf');
  });
});
```

### 2.5 场景 5：requireMention 过滤

测试文件：`require-mention.test.ts`

> 需要真实 server（SSE/WS/Poll 都需要 HTTP 连接）。

```typescript
describe('requireMention 过滤', () => {
  let server: FastifyInstance;
  let port: number;
  let adminToken: string;
  let agentKey: string;
  let agentId: string;
  let channelId: string;

  beforeAll(async () => {
    server = await buildFullApp(); // 注册所有路由
    await server.listen({ port: 0 });
    port = (server.server.address() as any).port;
    // setup admin, agent (requireMention=true), channel
  });
  afterAll(async () => { await server.close(); });

  describe.each([
    ['SSE', '/api/v1/channels/{ch}/stream'],
    ['Poll', '/api/v1/channels/{ch}/poll'],
  ])('%s 路径', (label, pathTemplate) => {
    it('未被 @ 的消息 → 不推送给 requireMention=true 的 agent', async () => {
      const path = pathTemplate.replace('{ch}', channelId);
      const res = await fetch(`http://localhost:${port}${path}`, {
        headers: { 'x-api-key': agentKey },
        signal: AbortSignal.timeout(3000),
      });
      // 同时发一条不含 @agent 的消息
      await fetch(`http://localhost:${port}/api/v1/channels/${channelId}/messages`, {
        method: 'POST',
        headers: { cookie: adminToken, 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: 'no mention here' }),
      });
      // 等 2s，不应收到该消息
      const events = await collectEvents(res, 2000);
      expect(events.filter(e => e.data?.content === 'no mention here')).toHaveLength(0);
    });

    it('被 @ 的消息 → 推送给 requireMention=true 的 agent', async () => {
      const path = pathTemplate.replace('{ch}', channelId);
      const res = await fetch(`http://localhost:${port}${path}`, {
        headers: { 'x-api-key': agentKey },
        signal: AbortSignal.timeout(5000),
      });
      await fetch(`http://localhost:${port}/api/v1/channels/${channelId}/messages`, {
        method: 'POST',
        headers: { cookie: adminToken, 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: `hello <@${agentId}>`, mentions: [agentId] }),
      });
      const events = await collectEvents(res, 3000);
      expect(events.some(e => e.data?.content?.includes(agentId))).toBe(true);
    });
  });

  it('WS 路径 → requireMention=true + 未被 @ → 不推送', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    const received: any[] = [];
    ws.on('message', (raw) => received.push(JSON.parse(raw.toString())));
    await fetch(`http://localhost:${port}/api/v1/channels/${channelId}/messages`, {
      method: 'POST',
      headers: { cookie: adminToken, 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: 'ws no mention' }),
    });
    await sleep(2000);
    expect(received.filter(m => m.payload?.content === 'ws no mention')).toHaveLength(0);
    ws.close();
  });

  it('DM 频道 → 不受 requireMention 限制', async () => {
    // 创建 DM
    const dmRes = await fetch(`http://localhost:${port}/api/v1/dm`, {
      method: 'POST',
      headers: { cookie: adminToken, 'Content-Type': 'application/json' },
      body: JSON.stringify({ userId: agentId }),
    });
    const dm = await dmRes.json();
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    const received: any[] = [];
    ws.on('message', (raw) => received.push(JSON.parse(raw.toString())));
    await fetch(`http://localhost:${port}/api/v1/channels/${dm.id}/messages`, {
      method: 'POST',
      headers: { cookie: adminToken, 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: 'dm without mention' }),
    });
    await sleep(2000);
    expect(received.some(m => m.payload?.content === 'dm without mention')).toBe(true);
    ws.close();
  });
});
```

### 2.6 场景 6：Slash Commands

测试文件：`slash-commands.test.ts`

```typescript
describe('Slash Commands', () => {
  let ctx: TestContext;
  beforeAll(async () => { ctx = await TestContext.create({ routes: (app) => { registerSlashCommands(app); }}); });
  afterAll(() => ctx.close());

  it('/help → 返回命令列表', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/messages`, ctx.memberA.token, { content: '/help' });
    expect(res.statusCode).toBe(201);
    expect(res.json().content).toContain('/help');
  });

  it('/invite @user → 将用户加入频道', async () => {
    const newCh = seedChannel(ctx.db, ctx.admin.id, 'invite-test');
    const res = await ctx.inject('POST', `/api/v1/channels/${newCh}/messages`, ctx.admin.token, { content: `/invite <@${ctx.memberA.id}>` });
    expect(res.statusCode).toBe(201);
    const member = ctx.db.prepare('SELECT * FROM channel_members WHERE channel_id = ? AND user_id = ?').get(newCh, ctx.memberA.id);
    expect(member).toBeDefined();
  });

  it('/leave → 退出频道', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/messages`, ctx.memberA.token, { content: '/leave' });
    expect(res.statusCode).toBe(201);
    const member = ctx.db.prepare('SELECT * FROM channel_members WHERE channel_id = ? AND user_id = ?').get(ctx.channel.id, ctx.memberA.id);
    expect(member).toBeUndefined();
  });

  it('/topic new topic → 修改频道 topic', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/messages`, ctx.admin.token, { content: '/topic New Topic Here' });
    expect(res.statusCode).toBe(201);
    const ch = ctx.db.prepare('SELECT topic FROM channels WHERE id = ?').get(ctx.channel.id);
    expect(ch.topic).toBe('New Topic Here');
  });

  it('/dm @user → 创建 DM', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/messages`, ctx.memberA.token, { content: `/dm <@${ctx.memberB.id}>` });
    expect(res.statusCode).toBe(201);
  });

  it('无效命令 → 错误提示', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/messages`, ctx.memberA.token, { content: '/nonexistent' });
    expect(res.json().content).toContain('Unknown command');
  });
});
```

### 2.7 场景 7：Workspace

测试文件：`workspace-flow.test.ts`

```typescript
describe('Workspace', () => {
  let ctx: TestContext;
  beforeAll(async () => { ctx = await TestContext.create({ routes: (app) => { registerWorkspaceRoutes(app); registerUploadRoutes(app); }}); });
  afterAll(() => ctx.close());

  it('上传文件 → 201 + 文件元数据', async () => {
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/upload`, ctx.memberA.token, {
      name: 'test.txt', content: Buffer.from('hello').toString('base64'), mime_type: 'text/plain',
    });
    expect(res.statusCode).toBe(201);
    expect(res.json().name).toBe('test.txt');
  });

  it('列出文件 → 只看到自己的文件', async () => {
    // memberA 上传了文件
    const r1 = await ctx.inject('GET', `/api/v1/channels/${ctx.channel.id}/workspace`, ctx.memberA.token);
    expect(r1.json().files.length).toBeGreaterThan(0);
    // memberB 看不到 memberA 的文件
    const r2 = await ctx.inject('GET', `/api/v1/channels/${ctx.channel.id}/workspace`, ctx.memberB.token);
    expect(r2.json().files.length).toBe(0);
  });

  it('重命名文件 → 200 + 新名称', async () => {
    const fileId = ctx.db.prepare('SELECT id FROM workspace_files WHERE user_id = ? LIMIT 1').get(ctx.memberA.id)?.id;
    const res = await ctx.inject('PATCH', `/api/v1/channels/${ctx.channel.id}/workspace/${fileId}`, ctx.memberA.token, { name: 'renamed.txt' });
    expect(res.statusCode).toBe(200);
    expect(res.json().name).toBe('renamed.txt');
  });

  it('同名文件冲突 → 自动加后缀', async () => {
    await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/upload`, ctx.memberA.token, {
      name: 'dup.txt', content: Buffer.from('a').toString('base64'), mime_type: 'text/plain',
    });
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/upload`, ctx.memberA.token, {
      name: 'dup.txt', content: Buffer.from('b').toString('base64'), mime_type: 'text/plain',
    });
    expect(res.statusCode).toBe(201);
    expect(res.json().name).not.toBe('dup.txt'); // 应有后缀
  });

  it('文件夹 CRUD → 创建 + 嵌套 + 删除', async () => {
    const r1 = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/folder`, ctx.memberA.token, { name: 'docs' });
    expect(r1.statusCode).toBe(201);
    const folderId = r1.json().id;
    // 在文件夹内创建子文件夹
    const r2 = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/folder`, ctx.memberA.token, { name: 'sub', parent_id: folderId });
    expect(r2.statusCode).toBe(201);
    // 删除文件夹
    const r3 = await ctx.inject('DELETE', `/api/v1/channels/${ctx.channel.id}/workspace/${folderId}`, ctx.memberA.token);
    expect(r3.statusCode).toBe(200);
  });

  it('10MB 大小限制 → 超限 413', async () => {
    const bigContent = Buffer.alloc(11 * 1024 * 1024).toString('base64');
    const res = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/upload`, ctx.memberA.token, {
      name: 'huge.bin', content: bigContent, mime_type: 'application/octet-stream',
    });
    expect(res.statusCode).toBe(413);
  });

  it('删除文件 → 200 + 从列表消失', async () => {
    const fileId = ctx.db.prepare('SELECT id FROM workspace_files WHERE user_id = ? AND is_directory = 0 LIMIT 1').get(ctx.memberA.id)?.id;
    if (fileId) {
      const res = await ctx.inject('DELETE', `/api/v1/channels/${ctx.channel.id}/workspace/${fileId}`, ctx.memberA.token);
      expect(res.statusCode).toBe(200);
    }
  });

  it('下载文件 → 200 + 正确内容', async () => {
    const upRes = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/upload`, ctx.memberA.token, {
      name: 'download-me.txt', content: Buffer.from('download content').toString('base64'), mime_type: 'text/plain',
    });
    const fId = upRes.json().id;
    const res = await ctx.inject('GET', `/api/v1/channels/${ctx.channel.id}/workspace/${fId}/download`, ctx.memberA.token);
    expect(res.statusCode).toBe(200);
    expect(res.body).toContain('download content');
  });

  it('移动文件到文件夹 → 200 + parent_id 更新', async () => {
    // 创建文件夹
    const folderRes = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/folder`, ctx.memberA.token, { name: 'target-folder' });
    const folderId = folderRes.json().id;
    // 上传文件
    const upRes = await ctx.inject('POST', `/api/v1/channels/${ctx.channel.id}/workspace/upload`, ctx.memberA.token, {
      name: 'move-me.txt', content: Buffer.from('move').toString('base64'), mime_type: 'text/plain',
    });
    const fileId = upRes.json().id;
    // 移动文件
    const res = await ctx.inject('PATCH', `/api/v1/channels/${ctx.channel.id}/workspace/${fileId}`, ctx.memberA.token, { parent_id: folderId });
    expect(res.statusCode).toBe(200);
    expect(res.json().parent_id).toBe(folderId);
  });
});
```

### 2.8 场景 8：Plugin 通信

测试文件：`plugin-comm.test.ts`

> 需要真实 server（WS endpoint）。

```typescript
describe('Plugin 通信', () => {
  let server: FastifyInstance;
  let port: number;
  let agentKey: string;

  beforeAll(async () => {
    server = await buildFullApp();
    await server.listen({ port: 0 });
    port = (server.server.address() as any).port;
    // setup admin + agent with api_key
  });
  afterAll(async () => { await server.close(); });

  it('WS 连接 → 有效 API Key → 连接成功', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    expect(ws.readyState).toBe(WebSocket.OPEN);
    ws.close();
  });

  it('WS 连接 → 无效 API Key → close 4001', async () => {
    const ws = new WebSocket(`ws://localhost:${port}/ws/plugin?apiKey=invalid`);
    const code = await waitForClose(ws);
    expect(code).toBe(4001);
  });

  it('SSE 连接 → 有效 API Key → 事件流建立', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/channels/${channelId}/stream`, {
      headers: { 'x-api-key': agentKey },
    });
    expect(res.status).toBe(200);
    expect(res.headers.get('content-type')).toContain('text/event-stream');
  });

  it('WS apiCall → 发消息 → server 收到并广播', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    const requestId = 'req-001';
    ws.send(JSON.stringify({
      type: 'apiCall',
      id: requestId,
      method: 'POST',
      path: `/api/v1/channels/${channelId}/messages`,
      body: { content: 'from plugin' },
    }));
    const response = await waitForMessage(ws, (m) => m.id === requestId);
    expect(response.status).toBe(201);
    expect(response.body.content).toBe('from plugin');
    ws.close();
  });

  it('WS apiCall → 添加 reaction → 200', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    ws.send(JSON.stringify({
      type: 'apiCall', id: 'req-react',
      method: 'POST',
      path: `/api/v1/channels/${channelId}/messages/${msgId}/reactions`,
      body: { emoji: '🔥' },
    }));
    const res = await waitForMessage(ws, (m) => m.id === 'req-react');
    expect(res.status).toBe(201);
    ws.close();
  });

  it('消息事件 → WS 客户端收到推送', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    // 用 HTTP 发消息触发事件
    await fetch(`http://localhost:${port}/api/v1/channels/${channelId}/messages`, {
      method: 'POST',
      headers: { 'x-api-key': agentKey, 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: 'trigger event' }),
    });
    const event = await waitForMessage(ws, (m) => m.type === 'event' && m.kind === 'message');
    expect(event.payload.content).toBe('trigger event');
    ws.close();
  });

  it('WS apiCall → 编辑消息 → 200 + 内容更新', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    // 先发一条消息
    ws.send(JSON.stringify({
      type: 'apiCall', id: 'req-create-edit',
      method: 'POST',
      path: `/api/v1/channels/${channelId}/messages`,
      body: { content: 'to be edited' },
    }));
    const created = await waitForMessage(ws, (m) => m.id === 'req-create-edit');
    const msgId = created.body.id;
    // 编辑消息
    ws.send(JSON.stringify({
      type: 'apiCall', id: 'req-edit',
      method: 'PATCH',
      path: `/api/v1/channels/${channelId}/messages/${msgId}`,
      body: { content: 'edited via ws' },
    }));
    const editRes = await waitForMessage(ws, (m) => m.id === 'req-edit');
    expect(editRes.status).toBe(200);
    expect(editRes.body.content).toBe('edited via ws');
    ws.close();
  });

  it('WS apiCall → 删除消息 → 200', async () => {
    const ws = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    ws.send(JSON.stringify({
      type: 'apiCall', id: 'req-create-del',
      method: 'POST',
      path: `/api/v1/channels/${channelId}/messages`,
      body: { content: 'to be deleted' },
    }));
    const created = await waitForMessage(ws, (m) => m.id === 'req-create-del');
    const msgId = created.body.id;
    ws.send(JSON.stringify({
      type: 'apiCall', id: 'req-del',
      method: 'DELETE',
      path: `/api/v1/channels/${channelId}/messages/${msgId}`,
    }));
    const delRes = await waitForMessage(ws, (m) => m.id === 'req-del');
    expect(delRes.status).toBe(200);
    ws.close();
  });

  it('断连重连 → WS 断开后重新连接仍能收到事件', async () => {
    const ws1 = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    ws1.close();
    await sleep(500);
    // 重新连接
    const ws2 = await connectWS(port, '/ws/plugin', { apiKey: agentKey });
    expect(ws2.readyState).toBe(WebSocket.OPEN);
    // 发消息触发事件
    await fetch(`http://localhost:${port}/api/v1/channels/${channelId}/messages`, {
      method: 'POST',
      headers: { 'x-api-key': agentKey, 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: 'after reconnect' }),
    });
    const event = await waitForMessage(ws2, (m) => m.type === 'event' && m.kind === 'message');
    expect(event.payload.content).toBe('after reconnect');
    ws2.close();
  });
});
```

### 2.9 场景 9：Remote Explorer

测试文件：`remote-explorer.test.ts`

> 需要真实 server（WS）。

```typescript
describe('Remote Explorer', () => {
  let server: FastifyInstance;
  let port: number;
  let ctx: TestContext;

  beforeAll(async () => {
    server = await buildFullApp();
    await server.listen({ port: 0 });
    port = (server.server.address() as any).port;
    ctx = await TestContext.create();
  });
  afterAll(async () => { await server.close(); ctx.close(); });

  it('注册 Node → 201 + token', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes`, {
      method: 'POST',
      headers: { cookie: ctx.admin.token, 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'my-machine', channelId: ctx.channel.id, directory: '/home/user' }),
    });
    expect(res.status).toBe(201);
    const data = await res.json();
    expect(data.token).toBeDefined();
  });

  it('WS 连接 → 有效 token → 连接成功', async () => {
    // 获取 node token
    const ws = await connectWS(port, '/ws/remote', { token: nodeToken });
    expect(ws.readyState).toBe(WebSocket.OPEN);
    ws.close();
  });

  it('文件代理读取 → Agent WS 转发 → 返回文件内容', async () => {
    // Mock agent 连接 WS 并响应 file_read 请求
    const agentWs = await connectWS(port, '/ws/remote', { token: nodeToken });
    agentWs.on('message', (raw) => {
      const msg = JSON.parse(raw.toString());
      if (msg.type === 'request' && msg.action === 'read') {
        agentWs.send(JSON.stringify({
          type: 'response', id: msg.id,
          data: { content: 'file content here', size: 17, mime_type: 'text/plain' },
        }));
      }
    });
    // HTTP 读取文件
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes/${nodeId}/read?path=/test.txt`, {
      headers: { cookie: ctx.admin.token },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.content).toBe('file content here');
    agentWs.close();
  });

  it('Node 离线 → 读取返回 503', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes/${offlineNodeId}/read?path=/x`, {
      headers: { cookie: ctx.admin.token },
    });
    expect(res.status).toBe(503);
  });

  it('非 owner 访问 → 403', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes/${nodeId}/read?path=/test.txt`, {
      headers: { cookie: ctx.memberA.token },
    });
    expect(res.status).toBe(403);
  });

  it('多用户隔离 → 用户 A 的 Node 用户 B 看不到', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes`, {
      headers: { cookie: ctx.memberB.token },
    });
    const nodes = await res.json();
    expect(nodes.find((n: any) => n.owner_id === ctx.admin.id)).toBeUndefined();
  });

  it('列出所有 Node → owner 看到自己的 Node 列表', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes`, {
      headers: { cookie: ctx.admin.token },
    });
    expect(res.status).toBe(200);
    const nodes = await res.json();
    expect(Array.isArray(nodes)).toBe(true);
    expect(nodes.length).toBeGreaterThan(0);
    expect(nodes.every((n: any) => n.owner_id === ctx.admin.id)).toBe(true);
  });

  it('Node stat → 返回文件元信息', async () => {
    // Mock agent 连接 WS 并响应 stat 请求
    const agentWs = await connectWS(port, '/ws/remote', { token: nodeToken });
    agentWs.on('message', (raw) => {
      const msg = JSON.parse(raw.toString());
      if (msg.type === 'request' && msg.action === 'stat') {
        agentWs.send(JSON.stringify({
          type: 'response', id: msg.id,
          data: { name: 'test.txt', size: 1024, isDirectory: false, mtime: Date.now() },
        }));
      }
    });
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes/${nodeId}/stat?path=/test.txt`, {
      headers: { cookie: ctx.admin.token },
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.name).toBe('test.txt');
    expect(data.size).toBe(1024);
    expect(data.isDirectory).toBe(false);
    agentWs.close();
  });
});
```

### 2.10 场景 10：Plugin 测试（Mock OpenClaw）

测试文件：`plugin-openclaw-mock.test.ts`

```typescript
// ── Mock Harness ──
class OpenClawMockHarness {
  private ctrl = new AbortController();
  public statuses: any[] = [];
  public inboundMessages: any[] = [];

  createAccount(overrides?: Partial<any>): any {
    return {
      accountId: 'test-account',
      enabled: true,
      baseUrl: 'http://localhost:PORT',
      apiKey: 'col_test_key',
      botUserId: '',
      botDisplayName: 'TestBot',
      requireMention: false,
      pollTimeoutMs: 30000,
      transport: 'auto',
      config: { allowFrom: ['*'] },
      configured: true,
      ...overrides,
    };
  }

  createContext(account: any) {
    return {
      abortSignal: this.ctrl.signal,
      account,
      cfg: {},
      setStatus: (s: any) => this.statuses.push(s),
    };
  }

  shutdown() { this.ctrl.abort(); }
}

describe('Plugin + Collab 集成', () => {
  let server: FastifyInstance;
  let port: number;
  let harness: OpenClawMockHarness;

  beforeAll(async () => {
    server = await buildFullApp();
    await server.listen({ port: 0 });
    port = (server.server.address() as any).port;
    harness = new OpenClawMockHarness();
  });
  afterAll(async () => { harness.shutdown(); await server.close(); });

  it('Plugin 启动 → 连接 Collab → 收到事件', async () => {
    const account = harness.createAccount({ baseUrl: `http://localhost:${port}` });
    const ctx = harness.createContext(account);
    // 启动 plugin gateway
    const gatewayPromise = startCollabGateway('test-ch', 'Test', ctx as any);
    await sleep(2000); // 等连接建立
    // 发消息触发事件
    // ... 验证 inbound 收到
    harness.shutdown();
  });

  it('Plugin outbound → sendMessage → Collab server 收到', async () => {
    // 通过 plugin outbound 发消息
    // 验证 Collab server 数据库中有消息
  });

  it('requireMention 过滤 → mock harness 中验证', async () => {
    const account = harness.createAccount({ requireMention: true });
    // 发不含 @bot 的消息 → plugin 不转发
    // 发含 @bot 的消息 → plugin 转发
  });
});

// ── Plugin 单元测试 ──
describe('Plugin 单元测试', () => {
  describe('outbound', () => {
    it('sendCollabText → 调用正确的 API endpoint', async () => {
      // mock fetch，验证 URL 和 body
    });
    it('WS 可用时 → 走 apiCall', async () => {
      // mock WS client，验证走 WS 而非 HTTP
    });
  });

  describe('ws-client', () => {
    it('连接成功 → 状态为 connected', async () => {
      const client = new CollabWsClient({ url: 'ws://localhost:PORT/ws/plugin', apiKey: 'test' });
      await client.connect();
      expect(client.state).toBe('connected');
      client.close();
    });
    it('断连 → 指数退避重连', async () => {
      const client = new CollabWsClient({ url: 'ws://localhost:PORT/ws/plugin', apiKey: 'test', reconnect: true });
      const delays: number[] = [];
      client.on('reconnecting', (delay: number) => delays.push(delay));
      await client.connect();
      // 模拟 3 次断连
      for (let i = 0; i < 3; i++) {
        client.simulateDisconnect();
        await sleep(100);
      }
      // 验证指数退避：每次 delay 递增
      expect(delays.length).toBeGreaterThanOrEqual(2);
      expect(delays[1]).toBeGreaterThan(delays[0]);
      client.close();
    });
    it('apiCall → request/response 匹配', async () => {
      const client = new CollabWsClient({ url: 'ws://localhost:PORT/ws/plugin', apiKey: 'test' });
      await client.connect();
      // mock server 响应
      const res = await client.apiCall('GET', '/api/v1/channels');
      expect(res.status).toBeDefined();
      expect(res.body).toBeDefined();
      client.close();
    });
    it('apiCall 超时 → reject', async () => {
      const client = new CollabWsClient({ url: 'ws://localhost:PORT/ws/plugin', apiKey: 'test', apiCallTimeoutMs: 100 });
      await client.connect();
      // server 不响应 → 超时
      await expect(client.apiCall('GET', '/api/v1/slow')).rejects.toThrow(/timeout/i);
      client.close();
    });
  });

  describe('sse-client', () => {
    it('解析 SSE 事件 → 正确提取 data', async () => {
      const raw = 'id: 1\nevent: message\ndata: {"content":"hello"}\n\n';
      const events = parseSseStream(raw);
      expect(events).toHaveLength(1);
      expect(events[0].id).toBe('1');
      expect(events[0].event).toBe('message');
      expect(events[0].data).toEqual({ content: 'hello' });
    });
    it('cursor 管理 → 重连时带 lastEventId', async () => {
      const client = new CollabSseClient({ url: 'http://localhost:PORT/api/v1/channels/ch1/stream', apiKey: 'test' });
      // 模拟收到 id=42 的事件
      client.processEvent({ id: '42', event: 'message', data: {} });
      expect(client.lastEventId).toBe('42');
      // 重连时应携带 Last-Event-ID
      const headers = client.buildReconnectHeaders();
      expect(headers['Last-Event-ID']).toBe('42');
    });
  });

  describe('file-access', () => {
    it('白名单内路径 → 允许读取', async () => {
      const checker = new FileAccessChecker({ allowPaths: ['/home/user/docs'] });
      expect(checker.isAllowed('/home/user/docs/readme.md')).toBe(true);
      expect(checker.isAllowed('/home/user/docs/sub/file.txt')).toBe(true);
    });
    it('白名单外路径 → 403 拒绝', async () => {
      const checker = new FileAccessChecker({ allowPaths: ['/home/user/docs'] });
      expect(checker.isAllowed('/etc/passwd')).toBe(false);
      expect(checker.isAllowed('/home/user/secret')).toBe(false);
      expect(checker.isAllowed('/home/user/docs/../secret')).toBe(false);
    });
  });

  describe('accounts', () => {
    it('配置解析 → 正确的默认值', async () => {
      const account = resolveCollabAccount({});
      expect(account.pollTimeoutMs).toBe(30000);
      expect(account.transport).toBe('auto');
      expect(account.botDisplayName).toBeDefined();
    });
    it('requireMention 默认 true', async () => {
      const account = resolveCollabAccount({});
      expect(account.requireMention).toBe(true);
    });
    it('transport 枚举校验', async () => {
      expect(() => resolveCollabAccount({ transport: 'ws' })).not.toThrow();
      expect(() => resolveCollabAccount({ transport: 'sse' })).not.toThrow();
      expect(() => resolveCollabAccount({ transport: 'poll' })).not.toThrow();
      expect(() => resolveCollabAccount({ transport: 'auto' })).not.toThrow();
      expect(() => resolveCollabAccount({ transport: 'invalid' as any })).toThrow();
    });
  });
});
```

### 2.11 场景 11：并发安全

测试文件：`concurrency.test.ts`

```typescript
describe('并发安全', () => {
  let ctx: TestContext;
  beforeAll(async () => { ctx = await TestContext.create(); });
  afterAll(() => ctx.close());

  it('邀请码并发消费 → 只有一个成功', async () => {
    const code = seedInviteCode(ctx.db, ctx.admin.id, 'RACE001');
    seedChannel(ctx.db, ctx.admin.id, 'general');
    const promises = Array.from({ length: 5 }, (_, i) =>
      ctx.app.inject({
        method: 'POST', url: '/api/v1/auth/register',
        payload: { invite_code: 'RACE001', email: `race${i}@test.com`, password: 'pass', display_name: `Race${i}` },
      })
    );
    const results = await Promise.all(promises);
    const successes = results.filter(r => r.statusCode === 201);
    expect(successes.length).toBe(1);
  });

  it('同一消息并发编辑 → 不丢数据', async () => {
    const msgId = seedMessage(ctx.db, ctx.channel.id, ctx.memberA.id, 'original');
    const promises = Array.from({ length: 3 }, (_, i) =>
      ctx.inject('PATCH', `/api/v1/channels/${ctx.channel.id}/messages/${msgId}`, ctx.memberA.token, { content: `edit-${i}` })
    );
    const results = await Promise.all(promises);
    const successes = results.filter(r => r.statusCode === 200);
    expect(successes.length).toBeGreaterThanOrEqual(1);
    // 最终内容应是最后一个成功的编辑
    const row = ctx.db.prepare('SELECT content FROM messages WHERE id = ?').get(msgId);
    expect(row.content).toMatch(/^edit-\d$/);
  });
});
```

### 2.12 场景 12：Plugin 部署验证

测试文件：`plugin-build.test.ts`

```typescript
describe('Plugin 部署验证', () => {
  it('build 产出 dist/ 文件完整', async () => {
    const distPath = path.resolve(__dirname, '../../dist');
    expect(fs.existsSync(distPath)).toBe(true);
    expect(fs.existsSync(path.join(distPath, 'index.js'))).toBe(true);
    expect(fs.existsSync(path.join(distPath, 'index.d.ts'))).toBe(true);
  });

  it('package.json 不含 devDependencies 和 scripts', () => {
    const pkg = JSON.parse(fs.readFileSync(path.resolve(__dirname, '../../package.json'), 'utf-8'));
    // production build 不应有 devDeps（或确认不会被打包）
    expect(pkg.main || pkg.exports).toBeDefined();
    // extensions 入口指向编译后的文件
    if (pkg.extensions?.entry) {
      expect(pkg.extensions.entry).toMatch(/\.js$/);
    }
  });
});
```

### 2.13 场景 13：数据库 Migration

测试文件：`migration.test.ts`

```typescript
describe('数据库 Migration', () => {
  it('新 DB → 全部表创建成功', () => {
    const db = createTestDb();
    const tables = db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all().map((r: any) => r.name);
    expect(tables).toContain('users');
    expect(tables).toContain('channels');
    expect(tables).toContain('messages');
    expect(tables).toContain('channel_members');
    expect(tables).toContain('mentions');
    expect(tables).toContain('events');
    expect(tables).toContain('workspace_files');
    db.close();
  });

  it('Migration 幂等 → 重复执行不报错', () => {
    const db = createTestDb();
    expect(() => {
      // 再次执行 schema 创建（createTestDb 内部的 SQL）
      // 因为用了 IF NOT EXISTS，不应报错
      createTestDb(); // 新 DB，但验证 SQL 本身幂等
    }).not.toThrow();
    db.close();
  });

  it('新增列不破坏现有数据', () => {
    const db = createTestDb();
    const adminId = seedAdmin(db, 'MigAdmin');
    const chId = seedChannel(db, adminId, 'mig-ch');
    seedMessage(db, chId, adminId, 'before migration');
    // 模拟新增列
    db.exec('ALTER TABLE messages ADD COLUMN metadata TEXT DEFAULT NULL');
    // 旧数据仍可读
    const msg = db.prepare('SELECT * FROM messages WHERE content = ?').get('before migration');
    expect(msg).toBeDefined();
    expect(msg.metadata).toBeNull();
    db.close();
  });
});
```

### 2.14 场景 14：消息文件链接链路（B22）

测试文件：`file-link.test.ts`

> 需要真实 server（WS 代理读取）。

```typescript
describe('消息文件链接链路', () => {
  let server: FastifyInstance;
  let port: number;
  let ctx: TestContext;

  beforeAll(async () => {
    server = await buildFullApp();
    await server.listen({ port: 0 });
    port = (server.server.address() as any).port;
    ctx = await TestContext.create();
  });
  afterAll(async () => { await server.close(); ctx.close(); });

  it('Agent 发含文件路径的消息 → 存储成功', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/channels/${ctx.channel.id}/messages`, {
      method: 'POST',
      headers: { 'x-api-key': ctx.agent.apiKey, 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: 'See file: /home/user/report.txt' }),
    });
    expect(res.status).toBe(201);
  });

  it('Owner 读取文件 → Plugin WS 转发 → 返回内容', async () => {
    // mock agent WS 响应 file_read
    // owner 请求读取 → 200 + 文件内容
  });

  it('非 owner 读取 → 403', async () => {
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes/${nodeId}/read?path=/report.txt`, {
      headers: { cookie: ctx.memberB.token },
    });
    expect(res.status).toBe(403);
  });

  it('Agent 离线 → 读取返回 503 + 离线提示', async () => {
    // 不连接 agent WS
    const res = await fetch(`http://localhost:${port}/api/v1/remote/nodes/${nodeId}/read?path=/report.txt`, {
      headers: { cookie: ctx.admin.token },
    });
    expect(res.status).toBe(503);
  });

  it('白名单外路径 → 403', async () => {
    // agent WS 连接但拒绝白名单外路径
    // 请求 /etc/passwd → 403
  });
});
```

## 3. OpenClaw Mock Harness 实现

详见场景 10。核心组件：

| 组件 | 职责 | 实现方式 |
|------|------|----------|
| `OpenClawMockHarness` | 模拟 OpenClaw runtime | 类，管理 AbortController + 收集 inbound |
| `createAccount()` | 生成 ResolvedCollabAccount | 带默认值的 factory |
| `createContext()` | 生成 ChannelGatewayContext | abortSignal + setStatus mock |
| `shutdown()` | 停止 plugin | abort signal |

**Plugin import 解决**：tsconfig paths alias 映射 `openclaw/plugin-sdk/*` 到实际路径。

## 4. Task Breakdown

| Task | 内容 | 文件 | 预估 |
|------|------|------|------|
| T1 | TestContext + WS helper + CI 配置 | setup-integration.ts | 小 |
| T2 | 场景 1+3: 认证 + 权限 | auth-flow.test.ts, permissions.test.ts | 中 |
| T3 | 场景 2+4: 频道 + 消息 | channel-lifecycle.test.ts, message-system.test.ts | 中 |
| T4 | 场景 5+6: requireMention + Slash | require-mention.test.ts, slash-commands.test.ts | 中 |
| T5 | 场景 7: Workspace | workspace-flow.test.ts | 中 |
| T6 | 场景 8+14: Plugin WS + 文件链接 | plugin-comm.test.ts, file-link.test.ts | 大 |
| T7 | 场景 9: Remote Explorer | remote-explorer.test.ts | 大 |
| T8 | 场景 10: OpenClaw mock harness + Plugin 集成 | plugin-openclaw-mock.test.ts | 大 |
| T9 | 场景 11+12+13 + 覆盖率调优 | concurrency.test.ts, plugin-build.test.ts, migration.test.ts | 中 |

## 5. 验收标准

- [ ] 14 个场景全部有对应测试文件
- [ ] 每个场景的子项全部有具体 test case（describe/it + setup/action/assertion）
- [ ] TestContext helper 封装多用户场景
- [ ] OpenClaw mock harness 能跑 startCollabGateway
- [ ] WS 测试用真实 server + 随机端口
- [ ] 覆盖率 ≥ 85%
- [ ] CI 通过
- [ ] 现有 API 测试保留不删除
