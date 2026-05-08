# Acceptance Template — admin-spa-ui-coverage 第一波 (≤50 行)

> Spec: `admin-spa-ui-coverage-spec.md` (战马C v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收
>
> **范围**: client-only UI 接 existing server endpoint — users 详情页 capability grant UI (D6 真兑现) + PATCH /users/{id} 5 字段 body (role/disabled/password 等). 0 server / 0 endpoint / 0 schema 改, ≤300 行 client.

## 验收清单

### §1 行为不变量 (CAPABILITY-DOT SSOT + admin god-mode 路径独立)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 api.ts 加 fetchUserPermissions / grantUserPermission / revokeUserPermission + UserPermissionDetail interface (字段 byte-identical 跟 server admin.go:393-403 sanitize) | 数据契约 | `admin-spa-ui-coverage.test.tsx::REG-ASUC-001..002` PASS |
| 1.2 patchUser 扩 body 字段 (role + require_mention) byte-identical 跟 server handleUpdateUser admin.go:205-211 | 数据契约 | `_REG-ASUC-003` PASS |
| 1.3 UserDetailPage 9 DOM 锚 `data-asuc-*` byte-identical (跟 ADMIN-SPA-SHAPE-FIX `data-asf-*` 模式承袭) | DOM grep | `_REG-ASUC-004` PASS (15 anchors verified) |
| 1.4 中文 UI 文案 14 字面 byte-identical (账号操作 / 能力授权 / 当前授权 / 重置密码 / 改角色 / 启用/停用账号 / 授予 / 撤销 / 已授予 / 已撤销 / 暂无授权 / 未知能力 + 2 derivative msg) | content-lock | `_REG-ASUC-005` PASS |
| 1.5 CAPABILITY-DOT #628 14 const SSOT byte-identical — UserDetailPage 从 `lib/capabilities` import, 反 hardcode 字面 | grep | `_REG-ASUC-006` PASS (3 hardcode 0 hit) |

### §2 数据契约 + 反向 grep 锚 (server endpoint 不动)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 `git diff origin/main -- packages/server-go/` 0 行 (本 milestone client-only) | git diff | 0 行 ✅ |
| 2.2 admin god-mode 路径独立 — UserDetailPage 仅 /admin-api/* 走, 不串 user-rail (`/api/v1/` + `from '../../lib/api'` 0 hit) | grep | `_REG-ASUC-007` PASS |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有 client vitest 全绿不破 + 1 新 file 7 case PASS (107 file 704 case 全绿) | full vitest | vitest run PASS |
| 3.2 立场承袭 ADMIN-SPA-SHAPE-FIX #633 D6 server gate + 本 milestone client UI 走 CAPABILITY-DOT #628 14 const SSOT 双侧守门 | inspect | spec §0 立场承袭 byte-identical |

## REG-ASUC-* (initial ⚪ → 🟢 post-impl)

- REG-ASUC-001 🟢 api.ts 3 helper (fetchUserPermissions / grantUserPermission / revokeUserPermission) + endpoint path byte-identical
- REG-ASUC-002 🟢 UserPermissionDetail interface 4 字段 byte-identical 跟 server sanitize
- REG-ASUC-003 🟢 patchUser body 扩 5 字段 byte-identical 跟 server handleUpdateUser
- REG-ASUC-004 🟢 UserDetailPage 9+ DOM 锚 data-asuc-* SSOT (15 实测)
- REG-ASUC-005 🟢 中文 UI 文案 14 字面 byte-identical (content-lock §1)
- REG-ASUC-006 🟢 CAPABILITY-DOT #628 14 const SSOT byte-identical (反 hardcode 0 hit)
- REG-ASUC-007 🟢 admin god-mode 路径独立 (ADM-0 §1.3 红线 反 user-rail leak 0 hit)

## 退出条件

- §1 (5) + §2 (2) + §3 (2) 全绿 — 一票否决
- vitest 7 case PASS + 既有 107 file 704 case 全绿
- 0 server / 0 endpoint / 0 schema 改
- 登记 REG-ASUC-001..007

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-02 | 战马C | v0+v1 实施 — admin-spa-ui-coverage 第一波 4 件套 byte-identical + UserDetailPage 250 行 client UI + api.ts 60 行 helper + 7 vitest PASS. REG-ASUC-001..007 ⚪→🟢 全翻. 立场承袭 ADMIN-SPA-SHAPE-FIX #633 D6 + CAPABILITY-DOT #628 + ADM-0 §1.3. |
