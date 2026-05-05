# COL-B12: 测试覆盖度提升 — 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 目标

- 测试覆盖率 **≥ 80%**（语句覆盖）
- CI 加覆盖率检查，低于 80% 则 fail
- 覆盖所有核心后端模块

## 2. 现状

- 1 个测试文件：`packages/server/src/__tests__/core.test.ts`（218 行，17 个测试）
- 只覆盖权限检查逻辑
- 无 API 路由测试、无 WS 测试、无 query 层测试

## 3. 测试策略

### 3.1 测试框架

已有 Vitest。加 coverage：`vitest --coverage`（`@vitest/coverage-v8`）。

### 3.2 覆盖范围

| 模块 | 文件 | 测试类型 | 优先级 |
|------|------|---------|-------|
| 认证 | `auth.ts` | 单测（注册、登录、JWT） | P0 |
| 权限 | `queries.ts` 权限相关 | 单测（已有部分） | P0 |
| 消息 CRUD | `routes/messages.ts` | 集成测试（Fastify inject） | P0 |
| 频道 CRUD | `routes/channels.ts` | 集成测试 | P0 |
| Reactions | `routes/reactions.ts` | 集成测试 | P1 |
| 用户管理 | `routes/users.ts` | 集成测试 | P1 |
| WS 事件 | `ws.ts` | 集成测试（WS client mock） | P2 |
| DB migration | `db.ts` | 单测（schema 正确性） | P1 |

### 3.3 集成测试模式

用 Fastify 的 `app.inject()` 做 HTTP 集成测试，不启动真实服务：

```typescript
import { buildApp } from '../index.js';

const app = await buildApp(); // 返回 Fastify 实例
const res = await app.inject({
  method: 'POST',
  url: '/api/v1/auth/register',
  payload: { ... }
});
expect(res.statusCode).toBe(201);
```

每个测试用独立的 in-memory SQLite DB。

### 3.4 CI 覆盖率检查

修改 `.github/workflows/ci.yml`：
```yaml
- run: cd packages/server && npx vitest run --coverage
- run: |
    COVERAGE=$(cat packages/server/coverage/coverage-summary.json | jq '.total.statements.pct')
    if (( $(echo "$COVERAGE < 80" | bc -l) )); then
      echo "Coverage $COVERAGE% < 80%"
      exit 1
    fi
```

## 4. Task Breakdown

### T1: 测试基础设施

**内容**：
1. 加 `@vitest/coverage-v8` 依赖
2. 配置 `vitest.config.ts` 加 coverage
3. 创建 `buildTestApp()` 工厂函数（in-memory DB + Fastify inject）
4. CI 加覆盖率检查

**验收标准**：
- [ ] `pnpm test --coverage` 输出覆盖率报告
- [ ] CI 检查覆盖率 < 80% 则 fail

### T2: 认证测试

**内容**：注册、登录、JWT 验证、邀请码校验

**验收标准**：
- [ ] 注册成功/失败各场景
- [ ] 登录成功/失败
- [ ] 邀请码消费 + 并发安全

### T3: 频道 CRUD 测试

**内容**：创建/删除/更新频道、成员管理、权限检查

**验收标准**：
- [ ] CRUD 全路径
- [ ] 权限拒绝场景
- [ ] 软删除 + 幂等

### T4: 消息 CRUD + 编辑/删除测试

**内容**：发送/查询/编辑/删除消息

**验收标准**：
- [ ] 消息 CRUD
- [ ] 编辑权限（只能编辑自己的）
- [ ] 删除（自己 + admin）
- [ ] 已删消息 content masking

### T5: Reactions 测试

**内容**：添加/移除 reaction、幂等、限制

**验收标准**：
- [ ] 添加/移除 reaction
- [ ] 同 user+message+emoji 幂等
- [ ] 20 种 emoji 限制

### T6: Slash Commands 后端测试

**内容**：/topic API、频道更新

**验收标准**：
- [ ] PUT /channels/:id 更新 topic

### T7: 公开频道预览测试

**内容**：预览 API、自加入、非成员限制

**验收标准**：
- [ ] 24h 预览
- [ ] 自加入公开频道
- [ ] 私有频道 404

## 5. 目标

完成 T1-T7 后覆盖率应达 80%+。WS 测试（T8）作为 stretch goal。
