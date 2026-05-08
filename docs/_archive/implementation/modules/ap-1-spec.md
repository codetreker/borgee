# AP-1 access perms 严格 403 — spec brief (飞马 v0)

> 飞马 · 2026-04-29 · ≤80 行 · Phase 4 第 8 个 milestone (entry checklist #456 §1 row 7)
> 关联: 蓝图 `auth-permissions.md` §1 (ABAC + UI bundle 混合) + §1.2 v1 三 scope (`*` / `channel:<id>` / `artifact:<id>`) / `data-layer.md` `user_permissions(user_id, permission, scope)` SSOT / `g3-audit.md` A5 (CHN-1 REG-CHN1-007 ⏸️→🟢 trigger flip) / GitHub 私有 repo 同模式
> Owner: zhanma-c (主战) + 飞马 (spec 协作)

---

## 1. 范围 (3 立场)

### 立场 ① — 严格 403 (非 member 也 403, 不再 404 隐藏存在性)
- 当前 (CHN-1 #286): 非 member `GET /api/v1/channels/:id` → **404** (隐藏存在性)
- AP-1 后: 非 member `GET /api/v1/channels/:id` → **403** (暴露存在但拒访问), 跟 GitHub repo 私有路径同模式
- **REG-CHN1-007 ⏸️ deferred → 🟢 active flip**: 改断 `status === 403` 而非 `404`

### 立场 ② — ABAC capability check 单源
- `user_permissions(user_id, permission, scope)` 是 SSOT (蓝图 §1.1)
- 所有 endpoint authz 走 `internal/auth/abac.go::HasCapability(ctx, permission, scope)` 单 helper, 不字面 hardcode permission name (反 grep 锁)
- agent capability 走同 ABAC (agent 是 user_id 一种)

### 立场 ③ — capability 字面白名单 (≤30, 蓝图 §1)
v1 capability list (示意, 烈马 acceptance 锁字面):
- `read_channel` / `write_channel` / `delete_channel`
- `read_artifact` / `write_artifact` / `commit_artifact` / `iterate_artifact` / `rollback_artifact`
- `mention_user` / `read_dm` / `send_dm`
- `manage_members` / `invite_user` / `change_role`
- `admin_*` (admin god-mode 走 `/admin-api/*` 不入此白名单, ADM-0 §1.3 红线)

---

## 2. 反约束 (5 grep 锁, count==0)

```bash
# 1) hardcode permission 字面 (走 helper 单源)
git grep -nE 'HasCapability\("[a-z_]+"' packages/server-go/internal/api/  # 0 hit (走 const)

# 2) 非 helper authz check (反 ad-hoc)
git grep -nE 'if.*role.*==.*"admin"|user\.role\s*==' packages/server-go/internal/api/  # 0 hit

# 3) bundle 字面入 server (bundle 是 client UI 糖)
git grep -rnE '"bundle":|bundle_id' packages/server-go/internal/  # 0 hit

# 4) capability scope 漂出 v1 三层
git grep -nE 'workspace:|org:' packages/server-go/internal/auth/  # 0 hit (v1 不实施)

# 5) admin god-mode 走 ABAC (反, admin 不入业务 ABAC)
git grep -nE 'HasCapability.*admin_' packages/server-go/internal/api/  # 0 hit (admin 走 /admin-api 单独 mw)
```

---

## 3. 文件清单 (≤8 文件)

| 文件 | 范围 |
|---|---|
| `internal/auth/abac.go` | `HasCapability(ctx, permission, scope) bool` 单 helper SSOT |
| `internal/auth/capabilities.go` | capability const 字面白名单 (≤30, 跟 acceptance byte-identical) |
| `internal/auth/abac_test.go` | 5 立场单测 (≥10 case) + 反约束 grep 单测 |
| `internal/api/channels.go` | 改 `GET /channels/:id` 404→403 (REG-CHN1-007 flip) |
| `internal/api/artifacts.go` / `messages.go` / `mentions.go` | 走 `HasCapability` 单 helper |
| `migrations/00XX_ap_1_user_permissions.go` | v=24 (next 紧 v=23 ADM-2.2 后) — 如 schema 改; 不改可省 |
| `docs/qa/acceptance-templates/ap-1.md` (烈马) | ≤30 capability 字面 + REG-CHN1-007 flip 锚 + 5 反向断言 |
| `docs/qa/ap-1-stance-checklist.md` (野马) | 3 立场 + 5 反约束 grep 单测锚 |

---

## 4. 验收挂钩

- REG-CHN1-007 ⏸️→🟢 (CHN-1 严格 403 e2e: 非 member token GET /channels/:id → 403)
- REG-AP1-001..00X (5 反约束 grep + capability check 各 endpoint 覆盖 + admin 不入业务 ABAC + bundle 不入 server + scope 不漂 v1)
- G4.4 退出 gate 依赖 (PHASE-4-ENTRY-CHECKLIST §3 G4.4 严格 403 + admin 不入业务路径全 e2e)

---

## 5. 不在范围 (留账)

- workspace / org scope (v1 不做)
- `expires_at` 列 (schema 保留, UI / runtime 不做)
- bundle UI 渲染 (client 端 follow-up, 不在 AP-1 server scope)
- admin god-mode capability check (走 /admin-api 单独 mw, ADM-0 §1.3 已落)

---

## 6. 跨 milestone byte-identical 锁

- 跟 ADM-2 admin god-mode reject 链 7 同精神 (admin 不入 ABAC 业务路径, 走 /admin-api/* mw)
- 跟 CV-1 rollback owner-only 同模式 (capability gate + DOM-level gate 双层 defense-in-depth)
- 跟 CHN-1 #286 既有 404 路径承袭 (改一处 status code, 反向断言改 e2e)

---
