# Acceptance Template — ADM-0: admin 拆表 (3 段串行 PR)

> 蓝图: `docs/blueprint/current/admin-model.md` §1.2 + §1.3 + §2 + §3
> Implementation: `docs/architecture/admin-model.md` ADM-0
> R3 决议: 4 人 review 立场冲突 #2 (B29 路线, 2026-04-28)
> 烈马一票否决 gate: G2.0 (cookie 串扰反向断言)

## 拆 PR 顺序 (串行, 不并发)

- **ADM-0.1**: 新增 `admins` 独立表 + `/admin-api/auth/login` env bootstrap + 独立 cookie name
- **ADM-0.2**: cookie 拆分 + `RequirePermission` 去 admin 短路 + god-mode endpoint 元数据-only
- **ADM-0.3**: `users.role` enum 收成二态 + backfill 移行到 admins 表 + revoke session

## 验收清单

### 数据契约 (ADM-0.1 + ADM-0.3 落)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `admins` 表 schema 字段固化 (id / login / password_hash / created_at / created_by / last_login_at) | unit | 飞马 / 烈马 | _(填 PR # + test 路径)_ |
| `users.role` enum DB CHECK 限定 `('member','agent')` | unit | 战马 / 烈马 | _(待填)_ |
| `users WHERE role='admin'` post-migration 行数 == 0 | unit | 战马 / 烈马 | _(待填)_ |
| god-mode response struct 白名单 (列出每 endpoint 返回字段, 无 `body` / `content` / `text`) | unit + CI grep | 飞马 / 烈马 | _(待填)_ |

### 行为不变量 (G2.0 烈马一票否决)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1.a admin cookie 调 `/api/v1/messages` → 401 | unit | 烈马 | _(待填)_ |
| 4.1.a admin cookie 调 `/api/v1/channels` → 401 | unit | 烈马 | _(待填)_ |
| 4.1.a admin cookie 调 `/api/v1/agents` → 401 | unit | 烈马 | _(待填)_ |
| 4.1.b user cookie 调 `/admin-api/v1/users` → 401 | unit | 烈马 | _(待填)_ |
| 4.1.b user cookie 调 `/admin-api/v1/orgs` → 401 | unit | 烈马 | _(待填)_ |
| 4.1.b user cookie 调 `/admin-api/v1/channels` → 401 | unit | 烈马 | _(待填)_ |
| 4.1.c god-mode endpoint JSON 反射扫描无 `body`/`content`/`text` 字段名 | unit (fail-closed reflect scan) | 烈马 | _(待填)_ |
| 4.1.d post-migration `users WHERE role='admin'` == 0 | unit | 战马 / 烈马 | _(待填)_ |
| 4.1.e (烈马 #189 review 加补) 旧 admin 拿 ADM-0.3 之前的 user cookie → 401 (session revoke 反向断言) | E2E | 烈马 | _(待填, INFRA-2 后)_ |

### 蓝图行为对照 (闸 2)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| §1.2 B env bootstrap 无 promote: 没有任何 `/admin-api/.../promote` endpoint | CI grep | 飞马 | _(待填)_ |
| §1.3 god-mode 仅元数据: response struct 白名单 doc 引用蓝图行 | 人眼 | 飞马 (review) | _(待填)_ |
| §3 `admins` schema 与 doc 一致 | unit | 飞马 / 烈马 | _(待填)_ |

### testutil fixture 改造 (跨段 PR 必须)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `testutil/server.go:58/73` 删除 `role=admin` user fixture, 改造为 `SeedAdmin(t, db)` 创 admins 行 | unit | 飞马 / 战马 | _(待填)_ |
| 已 merge admin SPA 测试 (≥ 8 个) 全部跟改 fixture, CI 全绿 | unit | 烈马 (回归 audit) | _(待填)_ |

### 退出条件

- 上表 14 项**全绿** (一票否决式: 任何 4.1.x 红 → 不签字)
- 飞马引用 review 同意
- G2.0 gate 在 PROGRESS.md 标 ✅
