# Admin SPA — UserDetailPage (admin-spa-ui-coverage 第一波)

> 2026-05-02 · admin-spa-ui-coverage milestone (战马C). 一个 milestone 一个 PR. 0 server / 0 endpoint / 0 schema 改, 仅 client ≤300 行 接 既有 server endpoint UI.

## 0. 设计沿用

- **ADM-0 §1.3 admin god-mode 路径独立** — UserDetailPage 仅访问 `/admin-api/*` 走 admin api 模块, 不串 user-rail (`/api/v1/`) + 不 import user-rail `lib/api`
- **CAPABILITY-DOT #628 14 const SSOT byte-identical** — UserDetailPage 走 `lib/capabilities::CAPABILITY_TOKENS` 单源, 不准 hardcode 字面散落
- **ADMIN-SPA-SHAPE-FIX #633 D6 server gate + 本 milestone client UI dropdown** — server `auth.IsValidCapability` 守入口, client dropdown 限定 14 dot-notation, 双侧守门 SSOT

## 1. 文件 + 范围

| 文件 | 改动 |
|---|---|
| `packages/client/src/admin/api.ts` | 加 `UserPermissionDetail` interface (4 字段 byte-identical 跟 server `sanitize` admin.go:393-403) + `UserPermissionsResponse` interface + `fetchUserPermissions` / `grantUserPermission` / `revokeUserPermission` 3 helper byte-identical 跟 server admin.go:39-41; `patchUser` body 扩 5 字段 (`display_name?` / `password?` / `disabled?` + `role?` + `require_mention?`) byte-identical 跟 server `handleUpdateUser` admin.go:205-211 |
| `packages/client/src/admin/pages/UserDetailPage.tsx` | 重写 4 段 UI (账号信息既有 + 账号操作新 + 能力授权新 + 当前授权新 + Agents 既有不动); 15 DOM 锚 `data-asuc-*` byte-identical (跟 ADM-2-FOLLOWUP `data-adm2-*` + ADMIN-SPA-SHAPE-FIX `data-asf-*` 模式沿用) |

## 2. UI 段 (4 段)

1. **账号操作** (`data-asuc-account-actions`): 重置密码 input + 改角色 select (member/agent) + 启停账号 toggle button — 走 `patchUser({password|role|disabled})`
2. **能力授权** (`data-asuc-grant-form`): capability dropdown (14 const SSOT, 不 hardcode) + scope input (默认 `*`) + 授予 button — 走 `grantUserPermission(id, permission, scope)`
3. **当前授权** (`data-asuc-permissions-list`): table row 列 (能力 label / token / scope / 授予时间 / 撤销 button); 空态 `暂无授权` — 走 `fetchUserPermissions(id).details` + `revokeUserPermission(id, permission, scope)`
4. **Agents** (既有不动)

## 3. 中文 UI 文案 (14 字面 byte-identical, content-lock §1)

账号操作 / 能力授权 / 当前授权 / 重置密码 / 改角色 / 账号状态 / 启用账号 / 停用账号 / 授予 / 撤销 / 已授予 / 已撤销 / 暂无授权 / 未知能力

## 4. server endpoint (既有, 不动)

| Method | Path | Handler | 用途 |
|---|---|---|---|
| GET | `/admin-api/v1/users/{id}/permissions` | `handleListUserPermissions` (admin.go:39) | 列 user 当前授权 |
| POST | `/admin-api/v1/users/{id}/permissions` | `handleGrantPermission` (admin.go:40) | 授予 capability (post-#633 D6 IsValidCapability gate) |
| DELETE | `/admin-api/v1/users/{id}/permissions` | `handleRevokePermission` (admin.go:41) | 撤销 capability |
| PATCH | `/admin-api/v1/users/{id}` | `handleUpdateUser` (admin.go:205-211) | 改 display_name/password/role/require_mention/disabled (5 字段 body) |

## 5. grep 检查项 (REG-ASUC-001..007)

```bash
# REG-ASUC-006 — 反 hardcode 14 const 字面 (CAPABILITY-DOT SSOT)
grep -E "'channel\.read'|'artifact\.commit'|'user\.mention'" packages/client/src/admin/pages/UserDetailPage.tsx  # 0 hit

# REG-ASUC-007 — admin god-mode 路径独立 (ADM-0 §1.3 红线)
grep -E "fetch\(['\"]\/api\/v1" packages/client/src/admin/pages/UserDetailPage.tsx  # 0 hit
grep -E "from ['\"]\.\.\/\.\.\/lib\/api['\"]" packages/client/src/admin/pages/UserDetailPage.tsx  # 0 hit
```

## 6. 不在范围

- 第二波 admin endpoint UI (B 类候选 4 项: GET /runtimes / GET /heartbeat-lag / GET /channels/archived / GET /channels/{id}/description/history) — 留 `admin-spa-ui-coverage-wave-2` milestone
- edit-history admin SPA UI — 留 v1 GA backlog
- bundle-grant UI — `BundleSelector` AP-2 #620 已在 user-rail, admin-rail 不重复, 留 v2+
