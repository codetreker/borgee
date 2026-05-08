# ADM-0 PR Review Checklist (飞马预备)

> 用途: ADM-0.1 / ADM-0.2 / ADM-0.3 三段 PR review **前置死单**, 防止 review 时临时拍脑袋。
> 锚点: [`admin-model.md`](admin-model.md) §ADM-0 + blueprint [`admin-model.md`](../../blueprint/current/admin-model.md) §1.2 / §1.3 / §2 / §3
> 来源决议: PR #188 (蓝图 R3) + PR #189 (implementation R3 重排)
> 维护: 飞马; 战马 PR 来后逐项打勾, 任一红线触发 → request changes 不商量

---

## 通用 (三段全适用)

### 通用 1. PR 模板裸 metadata 强制

PR body **顶部 4 行必须裸出现**, 不准包 ```code fence```, 不准包 H2 (`## Blueprint`), 必须裸文本一行一项:

```
Blueprint: <蓝图引用路径 + §X.Y>
Touches: <internal/admin | internal/auth | store | client | docs>
Current 同步: <docs/current/... 路径 | N/A — 仅文档>
Stage: v0
```

**红线**: 任何一行包在 ``` 里 / 写成 H2 / 行序错 → request changes 不读 diff。

### 通用 2. PR LOC 上限

每段 ≤ 800 LOC (admin-model.md 锁定值)。**红线**: > 800 行 → request changes, 让战马拆。
- 测试文件 (含 `_test.go` / `*.test.tsx`) **算入** LOC (烈马口径, R3 #189 留没明写, 这里锁死)
- 自动生成文件 (sqlc, openapi) **不算入**, 但 PR 里要标注

### 通用 3. Blueprint 反查

每段 PR description 必须挂 blueprint 行号锚点 (≥ 1 条), 没挂 → request changes。

---

## ADM-0.1 — admins 独立表 + env bootstrap + 独立 cookie name (双轨并存)

> 此阶段约束: **users.role='admin' 仍可登录** (双轨), admin-api 可同时被新 admin cookie / 旧 user cookie+role='admin' 走通。

### 1. 数据契约必查项

| 字段 / 资源 | schema_migrations 期望位 | Go struct 期望位 | 必查值 |
|---|---|---|---|
| `admins.id` | `store/migrations/NNNN_admins.sql` 第一行 | `store.Admin.ID` | UUID 主键, NOT NULL |
| `admins.login` | 同上 | `store.Admin.Login` | TEXT NOT NULL UNIQUE |
| `admins.password_hash` | 同上 | `store.Admin.PasswordHash` | TEXT NOT NULL (bcrypt cost ≥ 10) |
| `admins.created_at` | 同上 | `store.Admin.CreatedAt` | TIMESTAMP NOT NULL DEFAULT now() |
| **不准多** | — | — | **没有** `org_id` / `role` / `is_admin` / `email` 字段 (蓝图 §1.2: admin 不在任何 org, 无 promote) |
| schema_migrations v 号 | 必须紧跟 main 当前最大 v + 1 | — | 单调递增, 不准跳号 |
| migration down() | 必须存在且能 drop 表 | — | rollback 不能留孤儿数据 |
| env 读取 | `cmd/server/main.go` startup | — | `BORGEE_ADMIN_LOGIN` + `BORGEE_ADMIN_PASSWORD_HASH` 字面 (不准改名) |
| cookie name | `internal/admin/auth.go` const | — | `borgee_admin_session` 字面 (蓝图 §1.2 锁定, 改名 = 红线) |
| auth path 隔离 | `internal/admin/auth.go` 不 import `internal/auth.go` | — | grep `internal/auth` 命中 0 |

### 2. 行为不变量必查反向断言

| 断言 ID | 描述 | 单测预期路径 |
|---|---|---|
| 1.A | env 未设 → server 启动 fail-loud (panic with clear message) | `cmd/server/main_test.go` 或 `internal/admin/auth_bootstrap_test.go` |
| 1.B | env 设了相同 login 重启 server → admins 表不重复插 (idempotent bootstrap) | `internal/admin/auth_bootstrap_test.go` |
| 1.C | `POST /admin-api/auth/login` 用 env login → 返 200 + Set-Cookie `borgee_admin_session` | `internal/admin/auth_test.go` |
| 1.D | `POST /admin-api/auth/login` 用普通 user login → 返 401 (auth path 隔离) | 同上 |
| 1.E | bcrypt verify 必须用 `subtle.ConstantTimeCompare` 不准 `==` (烈马口径) | 同上 |
| 1.F | 双轨并存验证: `users.role='admin'` 旧账号调 `/admin-api/v1/*` 仍 200 (本阶段不砍, 留 ADM-0.2) | `internal/admin/middleware_test.go` |

### 3. 拒收红线

- ❌ cookie name 不是 `borgee_admin_session` 字面
- ❌ env 变量名不是 `BORGEE_ADMIN_LOGIN` / `BORGEE_ADMIN_PASSWORD_HASH` 字面
- ❌ `admins` 表多任何 R3 蓝图未列字段 (org_id / role / email / is_admin)
- ❌ `internal/admin/auth.go` import 了 `internal/auth.go` 任何符号 (auth path 必须分裂)
- ❌ migration 没有 down() 或 down() 不能干净回滚
- ❌ password_hash 不是 bcrypt (明文 / sha256 / md5)
- ❌ PR > 800 LOC (含测试)

---

## ADM-0.2 — cookie 拆 + RequirePermission 去 admin 短路 + god-mode 元数据-only

> 此阶段约束: **users.role='admin' 调 user-api 一律 401**, 但 users 表里行还在 (留 ADM-0.3 收尾)。

### 1. 数据契约必查项

| 项 | 期望位 | 必查值 |
|---|---|---|
| god-mode response struct 白名单 | `internal/admin/handlers.go` 每个 list endpoint | 字段穷举: orgs(id/name/created_at/member_count), users(id/email/role/created_at), channels(id/name/kind/owner_id/created_at), counts(scalar int), status(scalar enum) |
| 白名单**反向**: 禁字段 grep | 同上 | grep `\bbody\b` / `\bcontent\b` / `\btext\b` / `\bartifact\b` 命中 0 |
| RequirePermission 中间件 | `internal/middleware/auth.go` (或同等位) | 移除 "if user.Role == 'admin' return next" 分支, grep `role.*admin.*return` 命中 0 |
| admin SPA fetch path | `client/src/admin/api.ts` (或同等位) | 所有 `/admin-api/v1/*` 带 `credentials: 'include'` + 不再发送 user session cookie |

### 2. 行为不变量必查反向断言 (烈马一票否决, G2.0 gate 输入)

| 断言 ID | 描述 | 单测预期路径 |
|---|---|---|
| **2.A.1** | admin cookie 调 `/api/v1/messages` → 401 | `internal/server/auth_isolation_test.go` |
| **2.A.2** | admin cookie 调 `/api/v1/channels` → 401 | 同上 |
| **2.A.3** | admin cookie 调 `/api/v1/agents` → 401 | 同上 |
| **2.B.1** | user cookie 调 `/admin-api/v1/users` → 401 | 同上 |
| **2.B.2** | user cookie 调 `/admin-api/v1/orgs` → 401 | 同上 |
| **2.B.3** | user cookie 调 `/admin-api/v1/channels` → 401 | 同上 |
| **2.C** | god-mode endpoint 返回 JSON 经 reflect 扫描, 字段名集合 ∩ {body, content, text, artifact} = ∅ (fail-closed) | `internal/admin/handlers_field_whitelist_test.go` |
| **2.D** | `users.role='admin'` 旧账号调 `/admin-api/v1/*` 现在 → 401 (因为 cookie 拆 + 短路砍) | `internal/admin/middleware_test.go` |

### 3. 拒收红线

- ❌ god-mode endpoint response struct 出现 `body` / `content` / `text` / `artifact` 字段名 (任一)
- ❌ RequirePermission 中间件保留 "role='admin' 直通" 分支
- ❌ 单测 2.A / 2.B / 2.C 任一 missing 或 skip
- ❌ admin SPA 改 cookie path 但 client API 层未同步 (会导致前端整体 401, 集成测试必查)
- ❌ PR > 800 LOC

---

## ADM-0.3 — users.role enum 收成二态 + backfill

> 此阶段约束: **users WHERE role='admin' 行数恒为 0** (post-migration assertion).

### 1. 数据契约必查项

| 项 | 期望位 | 必查值 |
|---|---|---|
| schema_migrations v=N+2 | `store/migrations/NNNN_users_role_enum.sql` | `ALTER TABLE users ADD CONSTRAINT users_role_chk CHECK (role IN ('member','agent'))` 字面 |
| Go enum | `store/types.go` (或同等) | `type UserRole string`; 仅 `UserRoleMember` / `UserRoleAgent` 两个 const, 删除 `UserRoleAdmin` |
| testutil fixture | `internal/testutil/server.go` line ~58/~73 | 删除所有 `role: "admin"` fixture, grep 命中 0 |
| backfill SQL | 同 migration 文件 | 顺序: (1) `INSERT INTO admins (login, password_hash) SELECT ... FROM users WHERE role='admin' ON CONFLICT (login) DO NOTHING` (2) `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE role='admin')` (3) `DELETE FROM users WHERE role='admin'` |
| **idempotent 保证** | backfill SQL 必须用 `ON CONFLICT (login) DO NOTHING` 或等价 | 重跑不重复插, 红线见下 |

### 2. 行为不变量必查反向断言

| 断言 ID | 描述 | 单测预期路径 |
|---|---|---|
| 3.A | post-migration: `SELECT COUNT(*) FROM users WHERE role='admin'` = 0 | `store/migrations_test.go` 或 `internal/admin/migration_test.go` |
| 3.B | backfill 旧 admin 行 → admins 表多对应 login (login + bcrypt hash 复用) | 同上 |
| 3.C | backfill revoke session: 旧 admin user_id 在 sessions 表行数 = 0 | 同上 |
| 3.D | **idempotent**: migration up 跑两次 (用 `ApplyOnce` 之外的强制重跑路径) → admins 表行数与单次跑等同 | 同上 |
| 3.E | down() 测试 (烈马 R3 forward-only 仍要求 down 验证语法): drop CHECK 约束 + 不留孤儿 | 同上 |
| 3.F | testutil `SeedAdmin(t, db)` helper 改造完毕, 不再走 users 表 (改走 admins 表) | 各依赖 testutil 的测试文件 grep |
| 3.G | (反向) 任何代码路径仍写 `users.role = 'admin'` → 编译失败 (Go enum 删除该 const 自动达成) | `go vet` / build |

### 3. 拒收红线

- ❌ backfill SQL 不带 `ON CONFLICT DO NOTHING` 或等价 (重跑会重复插 admins)
- ❌ migration 顺序错: 先 DELETE users 再 INSERT admins (会丢 login/hash 信息, 灾难)
- ❌ Go enum 仍保留 `UserRoleAdmin` const (即便注释掉也不行, 必须删干净)
- ❌ testutil fixture 仍有 `role: "admin"` 残留 (grep 命中 ≥ 1)
- ❌ 旧 admin user 的 sessions 没 revoke (会导致旧 cookie 仍能登 user-api, 与 ADM-0.2 隔离矛盾)
- ❌ PR > 800 LOC

---

## 跨段一致性检查 (合 main 之后, 飞马 G2.0 gate 验证)

ADM-0.3 merge 后, 飞马跑一次 G2.0 完整验证 (烈马一票否决):

- [ ] 集成测试: 注册新 admin (env bootstrap) → 登 admin-api → 列 orgs → 退出 → 用同 cookie 调 user-api 一律 401
- [ ] 集成测试: 注册新 user → 登 user-api → 调 admin-api 一律 401
- [ ] 数据库扫描: `SELECT role, COUNT(*) FROM users GROUP BY role` 输出仅 `member` / `agent` 两行
- [ ] 数据库扫描: `admins` 表至少 1 行 (env bootstrap)
- [ ] god-mode 字段 lint: 跑 `internal/admin/handlers_field_whitelist_test.go` 全绿
- [ ] LOC 总计: ADM-0.1 + 0.2 + 0.3 ≤ 2400 LOC, 实际记录到 G2.audit 行
- [ ] blueprint-sha.txt 在 evidence 目录, sha 等于 #188 merge commit

---

## Review 流程 SOP

1. PR 一开 → 飞马 5 分钟内贴 "review checklist 启动" comment, 引用本文档段落
2. 逐项打勾, 任一红线 → request changes (引用本文档段落 + 行号)
3. 全绿 → comment LGTM (gh CLI 不允许 self-approve own org PR, comment 即批准)
4. 战马 self-merge → 飞马更新 PROGRESS.md ADM-0.x 行打 ✅ + audit row 登记

---

## 维护历史

- **2026-04-28** 飞马 v1: 三段 checklist 落地, deadline 2026-04-29 EOD 前就位 (战马 INFRA-2 + ADM-0.1 大概率 4-29 PR)
