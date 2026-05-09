# Admin — 后台管理面

> Phase 2 R3 锁: ADM-0.1 (#197) + ADM-0.2 (#201) + ADM-0.3 (#223) 串成 admin 完全独立子系统. user-rail 与 admin-rail **双轨彻底分裂**, 不再有"role=admin 的普通用户"这种重叠态.

Borgee 的 admin 面是一个**独立子系统**: 不同 SPA 入口 (`admin.html`), 不同 cookie (`borgee_admin_session`), 不同鉴权中间件 (`admin.RequireAdmin`), 不同凭证表 (`admins`). 本文讲清 server + client 双侧实现.

## 0. 单一 admin 入口 (ADM-0.2 后)

| 前缀 | 鉴权 | 谁用 |
|------|------|------|
| `/admin-api/v1/*` | `admin.RequireAdmin` (`borgee_admin_session` cookie / Bearer) | admin SPA (`admin.html`) |
| `/admin-api/auth/{login,logout,me}` | 同上 (login 例外) | admin SPA 登录路径 |

**ADM-0.2 砍掉的旧入口 (不要再以为它们存在)**:

- ❌ `/api/v1/admin/*` god-mode 旧挂载 (`AdminHandler.RegisterAppRoutes` 已移除, `internal/server/auth_isolation_test.go` 反向断言 → 404)
- ❌ `internal/api/admin_auth.go` (旧 JWT + `borgee_admin_token` cookie 路径) — 文件已删
- ❌ `auth.RequirePermission` 里 `users.role == "admin"` 短路 — ADM-0.2 删除
- ❌ `users.role='admin'` 行 — ADM-0.3 (v=10) 4 步 backfill 后 **count=0** (顺序锁: admins INSERT → sessions DELETE → user_permissions DELETE → users DELETE; 单事务)

## 1. Server 鉴权 (`internal/admin/`)

### 凭证表 (ADM-0.1, v=4)

```sql
CREATE TABLE admins (
  id            TEXT PRIMARY KEY,
  login         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,  -- bcrypt cost ≥ 10
  created_at    INTEGER NOT NULL
);
```

**红线 (review checklist §ADM-0.1)**: `admins` 表**不准**多 `org_id / role / is_admin / email` 字段. admin 不在任何 org, 不通过 promote 产生.

### Bootstrap

`cmd/collab/main.go` 启动调 `internal/admin.Bootstrap(db)`, 读 env:

| env | 缺失行为 |
|-----|---------|
| `BORGEE_ADMIN_LOGIN` | fail-loud panic |
| `BORGEE_ADMIN_PASSWORD_HASH` | fail-loud panic; 非 bcrypt 串或 cost < 10 也 panic |

`INSERT OR IGNORE` 落第一个 admin. 已存在 (login UNIQUE) → 跳过.

> 旧 env `ADMIN_USER` / `ADMIN_PASSWORD` **已废弃**. `config.go` 仍读取以兼容过渡期日志参考, bootstrap 路径完全不看.

### Login + Session (ADM-0.2, v=5)

`POST /admin-api/auth/login` body `{login, password}`:

- bcrypt 验证 + `subtle.ConstantTimeCompare` (单测 `auth_test.go::TestLogin_1E_ConstantTimeCompare`)
- 通过后 `crypto/rand` 生成 32 字节 hex (64 字符) token, 落 `admin_sessions(token PK, admin_id, created_at, expires_at)` 表 (v=5)
- 写 cookie `borgee_admin_session`: `HttpOnly; SameSite=Lax; MaxAge=604800; Path=/`, prod 非 localhost 加 `Secure`
- raw token 同时 JSON body 返回 (Bearer 兼容)

**红线**: cookie 值**必须**是 token (不是 admin id / login / email / 任何可枚举值). `internal/admin/auth.go::ResolveSession` 是 cookie → admin 的唯一通路.

### 中间件 `admin.RequireAdmin` (`middleware.go`)

1. 读 cookie `borgee_admin_session`;
2. 没有再看 `Authorization: Bearer <token>`;
3. 查 `admin_sessions` 表 (token + 未过期), 找到对应 admin → 注入 context. 否则 401.

### 隔离单测 (反向断言)

- `internal/admin/middleware_test.go` — `borgee_token` (user cookie) 喂 `/admin-api/v1/orgs` → 401
- `internal/server/auth_isolation_test.go` — `/api/v1/admin/users` (旧 god-mode) → 404
- `internal/admin/handlers_field_whitelist_test.go` — `/admin-api/v1/{stats,users,invites,channels}` 反射扫 response, 出现 `body|content|text|artifact` 等业务正文字段 → red

### 包级 import 隔离

`internal/admin/` 包**禁止 import** `internal/auth/`. grep enforce + 单测兜底.

## 2. Server 路由

| Method | Path | 行为 |
|--------|------|------|
| POST | `/admin-api/auth/login` | bcrypt + 签 session token |
| POST | `/admin-api/auth/logout` | 删 session token + Set-Cookie 过期 |
| GET | `/admin-api/auth/me` | 当前 session 信息 |
| POST | `/admin-api/v1/auth/login` (legacy alias) | ADM-0.2 重挂同 handler, 给 admin SPA 0-改 |

业务 admin 路由 (stats / users / invites / channels) 全部 `/admin-api/v1/*` 前缀, 全经 `RequireAdmin`:

| Method | Path | 备注 |
|--------|------|------|
| GET | `/admin-api/v1/stats` | user/channel/online + `by_org[]` (CM-1.3); 字段白名单守门 |
| GET | `/admin-api/v1/users` | 全量, role 现在只有 `member / agent` (ADM-0.3 后) |
| POST/PATCH/DELETE | `/admin-api/v1/users/...` | role 硬锁 `member`, 软删, agent 级联禁用 |
| `*` | `/admin-api/v1/{invites,channels,...}` | response sanitizer 只回元数据 |

> ADM-0.2 砍掉 god-mode 后, admin **看不到**消息 body / artifact / content. 字段白名单单测保护.

## 3. Client SPA (`packages/client/src/admin/`)

构建: `admin.html` 是 Vite 第二入口 (`vite.config.ts` Rollup `input.admin`), 与用户 SPA 共构建, 不共 React 树. `server.go` 对 `/admin` / `/admin/*` 请求 fallback 到 `dist/admin.html`.

`AdminAuthProvider` (`auth.ts`) — session 只在 React state, **不写 localStorage**. 真凭证是 HttpOnly `borgee_admin_session` cookie. `BASE = '/admin-api/v1'`, 所有请求 `credentials: 'include'`.

### Pages

| 文件 | 路由 | 端点 |
|------|------|------|
| `LoginPage.tsx` | `/admin` | `POST /admin-api/auth/login` (or v1 alias) |
| `DashboardPage.tsx` | `/admin/dashboard` | `GET /admin-api/v1/stats` (含 `by_org[]`) |
| `UsersPage.tsx` | `/admin/users` | `GET/POST/PATCH/DELETE /admin-api/v1/users[/:id]` |
| `UserDetailPage.tsx` | `/admin/users/:id` | 只读, `GET /admin-api/v1/users` + `/:id/agents` |
| `ChannelsPage.tsx` | `/admin/channels` | `GET /channels`, `DELETE /channels/{id}/force`; `#general` / DM 守卫 |
| `InvitesPage.tsx` | `/admin/invites` | `GET/POST/DELETE /invites` |
| `SettingsPage.tsx` | `/admin/settings` | session 信息 + logout |

**SPA 缺失/有意为之**:
- 没有"创建 admin"按钮 (admin 只能由 env bootstrap 决定)
- 没有 admin-only WebSocket 通道 (admin SPA 不订阅 `/ws`)

## 4. 与 PRD 的对应

PRD F1: admin = 全权运维 + 不在 org 内. 代码层:

- **唯一 admin 路径**: `admins` 表 + `borgee_admin_session` cookie + `/admin-api/*` 路由
- **唯一 user 路径**: `users` 表 + `borgee_token` JWT + `/api/v1/*` 路由 (role ∈ `{member, agent}`)
- 两条**永不交叉**, 单测 + middleware + migration 三层 enforce

## 5. 风险与注意

- `BORGEE_ADMIN_PASSWORD_HASH` 是 bcrypt hash (不是明文); 仍建议外面套 KMS / Sealed Secret, 定期轮换 login password.
- session token 64 hex chars, 失窃等价 admin 全权 — 紧急 `DELETE FROM admin_sessions WHERE admin_id=...` 即时吊销.
- `ForceDeleteChannel` 不可撤回; admin 误操作无 undo. `#general` / DM 类型频道 server 端守卫.
- ADM-0.3 后 `users.role='admin'` count=0 是 G2.0 不变量 — 任何 migration 想恢复 admin role 在 users 表都会破坏 regression suite §3.A–§3.G.

## 6. ADM-2 audit + impersonate (Phase 4)

> Spec: `docs/implementation/modules/adm-2-spec.md` § 1-2; content lock `docs/qa/adm-2-content-lock.md`; stance `docs/qa/adm-2-stance-checklist.md`. Acceptance: `docs/qa/acceptance-templates/adm-2.md`.

### 数据契约 (ADM-2.1 v=22, ADM-2.2 v=23)

- **`admin_actions`** 表 (v=22): `id PK / actor_id FK admins / target_user_id FK users / action enum CHECK / metadata JSON / created_at`. CHECK 5 字面 byte-identical: `delete_channel | suspend_user | change_role | reset_password | start_impersonation`. 双索引 `idx_admin_actions_target_user_id_created_at` + `idx_admin_actions_actor_id_created_at`.
- **`impersonation_grants`** 表 (v=23): `id PK / user_id FK users / granted_at / expires_at / revoked_at NULL`. 蓝图 §3 字面 "由 user 创建, admin 仅消费" — actor_id 不入此表. Index `idx_impersonation_grants_user_id_expires`.

### Endpoints

**User-rail** (`/api/v1/me/*`, 走 `borgee_token` cookie):
- `GET /api/v1/me/admin-actions` — 设计 ④ 只见自己 (server `WHERE target_user_id = current`, ?target_user_id 参数被忽略防 inject); user-rail 不返 `actor_id` raw.
- `GET /api/v1/me/impersonation-grant` — 业主端 BannerImpersonate 查询.
- `POST /api/v1/me/impersonation-grant` — 业主授权 24h (server 固定 expires_at = granted_at + 24h); 已有 active grant → 409 `impersonate.grant_already_active`.
- `DELETE /api/v1/me/impersonation-grant` — 业主主动撤销 (204).

**Admin-rail** (`/admin-api/v1/audit-log`, 走 `borgee_admin_session`):
- `GET /admin-api/v1/audit-log` — 设计 ③ 互可见 (无 WHERE 默认; 可选 `?actor_id=` / `?action=` / `?target_user_id=` 三 filter); user cookie → 401 (REG-ADM0-002 共享底线).

### 5 admin write-action audit hook

`internal/api/admin.go` 的 3 handler 已 wrap:
- `DELETE /admin-api/v1/channels/{id}/force` → action=`delete_channel` (target=channel.created_by; metadata={channel_id, channel_name})
- `PATCH /admin-api/v1/users/{id}` `disabled=true` → `suspend_user`
- `PATCH /admin-api/v1/users/{id}` `password` → `reset_password`
- `PATCH /admin-api/v1/users/{id}` `role` 改 → `change_role` (metadata={old_role, new_role})
- `start_impersonation` 留 admin SPA 端 future patch wire

audit + DM emit 走 `store.EmitAdminActionAudit` (composite); DM emit failure 不 rollback audit (蓝图 §2 优先).

### Forward-only 设计 ⑤

Schema 不挂 `updated_at` 列, server 不开 UPDATE/DELETE 路径. grep 检查 `UPDATE admin_actions\|DELETE FROM admin_actions` 在 `internal/` 除 migration 应 count==0.

### 反约束 (stance §2 ADM2-NEG-001..010)

- DM body 字面不含 `{admin_id}` / `{actor_id}` / `${adminId}` template placeholder
- DM body 不渲染 raw UUID actor_id (走 `actorLogin` = `admins.Login`)
- DM body `{ts}` 走 `time.Format("2006-01-02 15:04")` 不是 epoch ms
- `admin_actions.metadata` 不挂 body/content/text/artifact 字段 (god-mode 仅元数据)

### ADM-2-FOLLOWUP (#626 PR feat/adm-2-followup)

- REG-ADM2-010 wire: `handleCreateMyImpersonateGrant` 在 `s.GrantImpersonation` 成功后 fire `InsertAdminAction(actor=user, target=user, action="start_impersonation", metadata={grant_id, expires_at})` audit hook → REG-ADM2-003 4/5 → 5/5 收口.
- REG-ADM2-010 helper: `api.RequireImpersonationGrant(w, r, s, targetUserID)` 返 `(true, *admin.Admin)` 或 (false, _) 已写 4 字面错码: `impersonate.no_admin` (401) / `impersonate.no_target` (400) / `impersonate.no_grant` (403). 5 admin write handler 集成留 v1 后续; helper 已落 + 4 unit branch 全覆.
- REG-ADM2-011: 新 admin SPA audit-log 页 `[data-page="admin-audit-log"]` + `[data-adm2-audit-list="true"]` + `[data-adm2-red-banner="active"]` + 中文 title "审计日志" + 中文 empty "暂无审计记录" + 红 banner 字面 byte-identical "当前以业主身份操作 — 该会话受 24h 时限".
- 测试 seam: `admin.WithAdminContext(ctx, *Admin)` 导出 (test-only 注入 adminCtxKey, production 仍走 RequireAdmin → ResolveSession 唯一路径).

### ADMIN-SPA-SHAPE-FIX (#633 PR feat/admin-spa-shape-fix) — 6 drift 真修

**D1 login**: client `adminLogin(login, password)` body `{login, password}` byte-identical 跟 server `loginRequest{Login,Password}` (auth.go). LoginPage 表单 state username → login, label "Login".

**D2 AdminSession**: client interface 重写 `{id: string, login: string}` byte-identical 跟 server handleMe writeJSON (auth.go:281,314). 反假字段: 0 role / 0 username / 0 admin_id / 0 expires_at. AdminApp.tsx + SettingsPage.tsx 跟随用 `session?.login` 替 `session?.username`.

**D3 AdminChannel.member_count 死字段删**: server `Channel` gorm json (`store/models.go::Channel`) 不返 member_count 字段. 客户端 interface + ChannelsPage 表格列同删.

**D4 AL-8 archived 三态 (走 A)**: server `store.AdminAction` 加 `ArchivedAt *int64 \`gorm:"column:archived_at" json:"-"\``; `sanitizeAdminAction` 加 nil-safe surface (null/缺 = active 不写, non-null = `archived_at: int64 ms`). AdminAuditLogPage row 加 `data-archived-state="active|archived"` + `admin-audit-row-{active,archived}` className. AL-8 §0 设计③ 真兑现.

**D5 InviteCode.note**: 收紧 `string` non-null (server `store.InviteCode.Note string \`json:"note"\`` 默认 "").

**D6 admin-rail handleGrantPermission gate**: `internal/api/admin.go::handleGrantPermission` 加 `auth.IsValidCapability(body.Permission)` 守门, invalid → 400 `invalid_capability` (CAPABILITY-DOT #628 backfill 守存量, 此 gate 守入口 SSOT 第 5 处链 — user-rail 4 处 + admin-rail 1 处 = 5).

**反约束**: server diff ≤13 行 production (D4 sanitizer +5 + D6 gate +5 + struct field +3); 0 endpoint URL / 0 schema migration / 0 routes.go 改; admin-rail SSOT (CookieName `borgee_admin_session` / loginRequest / handleMe writeJSON) 字面 byte-identical 不动. ADM-0 §1.3 admin/user 路径分叉红线守.

### ADMIN-PASSWORD-PLAIN-ENV (PR feat/admin-password-plain-env) — B 方案明文 env

ADM-0.1 bootstrap env 加二选一支持: 推荐 prod 用 hash, dev/testing 用 plain.

| Env | 用途 | 安全 note |
|---|---|---|
| `BORGEE_ADMIN_LOGIN` | admin 登录名 (legacy, 必设) | 不变 |
| `BORGEE_ADMIN_PASSWORD_HASH` | bcrypt hash, cost ≥ 10 (legacy 推荐 prod) | 即使 env 泄露, 攻击者拿哈希仍需暴破 |
| `BORGEE_ADMIN_PASSWORD` | 明文密码 (新, 推荐 dev/testing) | env 泄露 = 明文泄露; 启动时 server bcrypt.GenerateFromPassword(MinBcryptCost) 内存哈希后写表, env 不再读 |

**二选一**: `HASH` 跟 `PASSWORD` 同时设 → bootstrap panic 提示 mutually exclusive. 都不设 → bootstrap panic 提示至少设一个.

**legacy backward-compat**: 仅设 `HASH` (旧路径) → 行为 byte-identical 不变 (server 直存 hash 字面到 admins.password_hash, login verify 走 bcrypt.CompareHashAndPassword).

**新 plain 路径**: 仅设 `PASSWORD` → server 启动时 `bcrypt.GenerateFromPassword([]byte(plain), MinBcryptCost)` 哈希 → 写 admins.password_hash. 简化 deploy (不再要先 htpasswd 算 hash 再填 env).

**反约束**:
- 0 endpoint URL / 0 schema / 0 cookie / 0 admin login/logout/me 行为改 (字面 byte-identical)
- bcrypt cost ≥ MinBcryptCost (10) 守 (review checklist 红线)
- env 中明文 plain 永不写盘 (只内存哈希后写 admins 表 hash 字段)

### ADMIN-SPA-ARCHIVED-UI-FOLLOWUP (PR feat/admin-spa-archived-ui-followup) — #633 D4-A client filter UI 闭环

#633 加了 server `sanitizeAdminAction` archived_at surface + row `data-archived-state` attr + `admin-audit-row-{active,archived}` className 三态, 但 client filter UI 当时未跟改 (REG-ADM-05 e2e 凿实). 此 milestone 闭环 — admin SPA AdminAuditLogPage 真加 `<select data-filter="archived">` 三态 toggle.

**client diff** (~20 行 production):

- `packages/client/src/admin/api.ts` — `AuditLogFilters` 加 `archived?: 'active' | 'archived' | 'all'` union; `fetchAdminAuditLog` 加 `if (filters.archived) qs.set('archived', filters.archived)` URL param 透传 server `?archived=` enum
- `packages/client/src/admin/pages/AdminAuditLogPage.tsx` — Target User ID label 之后 / Filter button 之前加 `<label>View<select data-filter="archived">` 3 option byte-identical (Active/Archived/All), `value={filters.archived ?? 'active'}` 默认 active 跟 server byte-identical

**反约束**:
- 0 server / 0 schema / 0 endpoint URL / 0 cookie 改 (`git diff origin/main -- packages/server-go/` = 0 行)
- #633 D4-A row className/data-archived-state 不破 (此 PR grep 检查 守, 0 改)
- 3 option value byte-identical 跟 server `admin_endpoints.go::handleAdminAuditLog ?archived=` enum 字面单源 (drift = 改两处 client + server)
- `admin-audit-row-archived` className 仅 AdminAuditLogPage 内, 0 漂入 AdminApp 别 page (反 cross-page contamination)

**vitest 5 case** (`admin-spa-archived-ui-followup.test.ts`, file-source content lock createRequire pattern): REG-ASAUI-001 select+3 option / 002 row 三态不破 / 003 AuditLogFilters union / 004 qs.set URL param / 005 反 cross-page contamination. client vitest 107/107 PASS.

**REG-ADM-05 翻牌** ✅ done 同 PR (e2e-scenarios.md §3 总数 + §3 v3 变更日志 + §4.2 闭环 + §5 退出条件 4 处).
