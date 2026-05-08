# Acceptance Template — AP-0-bis: message.read 默认 + backfill 迁移

> 蓝图: `docs/blueprint/current/auth-permissions.md` §3 (R3 已固化 message.read)
> Implementation: `docs/architecture/auth-permissions.md` AP-0-bis
> R3 决议: 4 人 review 立场冲突 #1 (2026-04-28)
> 依赖: ADM-0.2 已 merge (cookie 拆 + RequirePermission 短路砍 落地后才能挂 message.read gate); 与 INFRA-2 / CM-onboarding 可并行; CM-3 backfill pattern 沿用此处 helper

## 验收清单

### 数据契约

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.3 新注册 agent → user_permissions 多 2 行 (`message.send` + `message.read`) | unit | 战马 / 烈马 | _(待填)_ |
| migration v=N up 后, 现网所有 `role=agent` 都有 `message.read` 行 (idempotent) | unit | 战马 / 烈马 | _(待填)_ |
| migration down 干净回滚, 不残留 `message.read` | unit | 战马 / 烈马 | _(待填)_ |

### 行为不变量

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `GET /channels/:id/messages` 加 `RequirePermission("message.read")` | unit | 战马 | _(待填)_ |
| 无 `message.read` 的 agent 调上述 endpoint → 403 | unit | 战马 / 烈马 | _(待填)_ |
| 有 `message.read` 的 agent 调上述 endpoint → 200 + 消息列表 | unit | 战马 | _(待填)_ |

### testutil helper (跨 milestone 复用, CM-3 也用)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 新增 `testutil.SeedLegacyAgent(t, db)` helper, 插旧 schema agent (无 message.read) | unit | 飞马 / 战马 | _(待填)_ |
| Helper 文档化在 `internal/testutil/README.md` (or godoc) | 人眼 (PR review) | 飞马 | _(待填)_ |
| Backfill 测试用 SeedLegacyAgent 验"前 → up → 后" | unit | 战马 / 烈马 | _(待填)_ |

### 蓝图行为对照 (闸 2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| auth-permissions §3 Messaging capability list 含 `message.read` | CI grep | 飞马 | _(待填)_ |
| auth-permissions §1.3 agent 默认 `[message.send, message.read]` | 人眼 (PR review) | 飞马 | _(待填)_ |

### 闸 5 (覆盖率)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 单测覆盖 ≥ 80% (含分支文件 store/queries.go + middleware) | unit + go cover | 战马 / 烈马 | _(待填)_ |

### 退出条件

- 上表 11 项全绿
- AP-0 (#177) 既有测试**回归全绿** (新加 message.read 不破坏旧 default grant 测试)
- 飞马引用 review 同意
