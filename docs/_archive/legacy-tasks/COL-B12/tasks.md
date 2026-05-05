Tasks breakdown with details:

| Task | 测试目标文件 | 端点/模块 | 预估测试数 | 验证方式 | 依赖 |
|------|------------|----------|-----------|---------|------|
| **T1** 测试基础设施 | `vitest.config.ts`, CI yml | — | 0 (infra) | `pnpm test --coverage` 输出报告 | 无 |
| **T2** 认证测试 | `auth.ts`, `routes/` 认证端点 | `POST /auth/register`, `POST /auth/login` | ~10 | Fastify inject, in-memory SQLite | T1 |
| **T3** 频道 CRUD | `routes/channels.ts`, `queries.ts` | `POST/GET/PUT/DELETE /channels` | ~12 | Fastify inject, 权限拒绝断言 | T1 |
| **T4** 消息 CRUD | `routes/messages.ts`, `queries.ts` | `POST/GET/PUT/DELETE /messages` | ~12 | Fastify inject, content masking 检查 | T1 |
| **T5** Reactions | `routes/reactions.ts` | `POST/DELETE /reactions` | ~6 | Fastify inject, 幂等+20 emoji 限制 | T1 |
| **T6** Slash Commands | `routes/channels.ts` | `PUT /channels/:id` (topic) | ~3 | Fastify inject | T1, T3 |
| **T7** 公开频道预览 | `routes/channels.ts` | preview API, self-join | ~5 | Fastify inject, 24h 窗口检查 | T1, T3 |

**关键说明：**
- **总计 ~48 tests**（加上已有 17 个 = ~65）
- **T1 是所有后续任务的前置**：`buildTestApp()` 工厂函数 + coverage 配置
- **T3 是 T6/T7 的前置**：频道测试建立的 helper 可复用
- 所有集成测试使用 **Fastify `app.inject()`** + **独立 in-memory SQLite**，不启动真实服务
- Stretch goal: T8 (WS 测试 `ws.ts`) 不在核心范围内

要我开始实现吗？建议从 T1（测试基础设施）开始。
