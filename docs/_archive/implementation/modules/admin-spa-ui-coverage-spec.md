# admin-spa-ui-coverage spec brief — 第一波 users UI + 3 PATCH endpoint (≤80 行)

> 战马C · 2026-05-02 · post-#633 admin-spa-shape-fix wave; admin SPA UI 缺补第一波
> 关联: ADM-0 §1.3 红线 / CAPABILITY-DOT #628 14 const SSOT / admin-spa-shape-fix #633 D6 IsValidCapability gate

> ⚠️ Client-only milestone — **0 server / 0 endpoint / 0 schema / 0 routes 改** + ≤300 行 client 接 UI to existing server endpoint. liema e2e 真凿实 admin SPA 6 endpoint UI 缺, 第一波兑现 users 详情页 capability grant + 3 PATCH endpoint UI.

## 0. 关键约束 (3 条立场)

1. **0 server 改 + 复用 existing server endpoint** — server 已挂 GET/POST/DELETE `/admin-api/v1/users/{id}/permissions` (admin.go:39-41) + PATCH `/users/{id}` body 5 字段 (admin.go:205-211: display_name/password/role/require_mention/disabled). 反约束: `git diff origin/main -- packages/server-go/` 0 行.
2. **CAPABILITY-DOT #628 14 const SSOT byte-identical** — UserDetailPage 走 `lib/capabilities::CAPABILITY_TOKENS` 单源, 反 hardcode 字面 (反向 grep `'channel.read'|'artifact.commit'|'user.mention'` 在 UserDetailPage.tsx 0 hit). server-side post-#633 D6 IsValidCapability gate 守入口, 客户端只能选 14 dot-notation.
3. **admin god-mode 路径独立 (ADM-0 §1.3 红线)** — UserDetailPage 仅访问 `/admin-api/*` 走 admin api 模块, 不串 user-rail (`/api/v1/`) + 不 import user-rail `lib/api`. 反向 grep `fetch.*/api/v1/` + `from '../../lib/api'` 在 UserDetailPage.tsx 0 hit.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **ASUC.1 api.ts 扩 helper (≤60 行)** | `UserPermissionDetail` interface + `UserPermissionsResponse` interface + `fetchUserPermissions` / `grantUserPermission` / `revokeUserPermission` 3 method byte-identical 跟 server endpoint shape; `patchUser` body 字段从 3 → 5 (加 role + require_mention 跟 server handleUpdateUser admin.go:205-211 byte-identical) |
| **ASUC.2 UserDetailPage UI (≤200 行)** | 4 段 UI: 账号操作 (重置密码 / 改角色 / 启停账号) + 能力授权 (14 capability dropdown + scope input + 授予 button) + 当前授权 list (capability label + scope + 撤销 button) + 既有 Agents 段不动. 9 DOM 锚 `data-asuc-*` byte-identical (跟 ADM-2-FOLLOWUP `data-adm2-*` + ADMIN-SPA-SHAPE-FIX 模式承袭) |
| **ASUC.3 vitest + 4 件套 + closure** | `admin-spa-ui-coverage.test.tsx` 7 case (api shape + UserPermissionDetail interface + patchUser 字段 + DOM 锚 + 中文文案 + CAPABILITY-DOT SSOT + admin god-mode 路径独立); REG-ASUC-001..006 ⚪→🟢 + acceptance + 4 件套 + content-lock |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) 0 server / 0 endpoint / 0 schema 改
git diff origin/main -- packages/server-go/  # 0 行

# 2) CAPABILITY-DOT 14 const SSOT byte-identical (反 hardcode)
grep -E "'channel\.read'|'artifact\.commit'|'user\.mention'" packages/client/src/admin/pages/UserDetailPage.tsx  # 0 hit

# 3) admin god-mode 路径独立 (ADM-0 §1.3 红线)
grep -E "fetch\(['\"]\/api\/v1" packages/client/src/admin/pages/UserDetailPage.tsx  # 0 hit
grep -E "from ['\"]\.\.\/\.\.\/lib\/api['\"]" packages/client/src/admin/pages/UserDetailPage.tsx  # 0 hit

# 4) 9 DOM 锚 byte-identical (反向 grep 守门)
grep -cE 'data-asuc-' packages/client/src/admin/pages/UserDetailPage.tsx  # ≥9 hit

# 5) capabilities lib 单源 (反 inline 字面散落)
grep -nE "import.*CAPABILITY_TOKENS.*capabilityLabel.*isKnownCapability.*from.*lib\/capabilities" packages/client/src/admin/pages/UserDetailPage.tsx  # ≥1

# 6) post-#633 wave 既有 test
pnpm vitest run  # ALL PASS (107 file 704 case)
```

## 3. 不在范围 (留账)

- ❌ **第二波 admin endpoint UI** (B 类候选 4 项: GET /runtimes / GET /heartbeat-lag / GET /channels/archived / GET /channels/{id}/description/history) — 留 admin-spa-ui-coverage-wave-2 milestone
- ❌ **edit-history admin SPA UI** (messages/comment edit-history) — 太多 surface, 留 v1 GA backlog
- ❌ **server endpoint 加 / shape 改** (本 milestone 仅 client 接 UI; 真 prod hardening 留 admin-spa-shape-fix wave)
- ❌ **bundle-grant UI** (BundleSelector AP-2 #620 已在 user-rail, admin-rail 不重复; 留 v2+)

## 4. 跨 milestone byte-identical 锁

- ADMIN-SPA-SHAPE-FIX #633 D6 IsValidCapability gate 守 server 入口 + 本 milestone client UI 走 SSOT (CAPABILITY-DOT #628 14 const)
- ADM-2-FOLLOWUP #626 AdminAuditLogPage `data-adm2-*` DOM 锚模式承袭
- ADM-0 §1.3 admin/user 路径分叉红线 (admin SPA 仅 /admin-api/* 走)
- AP-2 #620 user-rail PermissionsView + BundleSelector 不破 (admin-rail 不串)
