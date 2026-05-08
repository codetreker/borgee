# AP-2 spec brief — capability 透明 UI 无角色名 (≤80 行) [v1 推断 scope]

> 飞马 · 2026-05-01 · 用户拍板待 PR review (推断 scope: capability 透明 UI 反 role bleed) · zhanma 主战 + 飞马 review
> **关联**: AP-1 #493 ✅ user_permissions + 14 capability const · AP-3 #521 ✅ cross-org · AP-4-enum #591 ✅ enum SSOT · AP-5 #555 messages ACL · ADM-0 §1.3 admin god-mode 红线
> **命名**: AP-2 = auth-permissions 第二件 (UI 层透明)

> ⚠️ 推断 scope (PROGRESS [ ] **AP-2** UI bundle 无角色名一句话, 用户无明确细则) — 本 spec v1 按 (a) capability 透明 UI 写, PR review 用户拍板再调.
> ⚠️ Client + server response shape milestone — **0 schema 改 / 0 endpoint URL 改 / 0 user-facing API 行为改** (仅 response payload 字段映射 + UI label).

## 0. 关键约束 (3 条立场)

1. **AP-1 14 capability const + AP-4-enum #591 reflect-lint SSOT byte-identical 不破** (跨 AP stack 锁链承袭): UI 显示**只走 capability token 字面** (如 `messages.write` / `channels.create` / `agent.invite`), 反向不映射"管理员/编辑者/查看者"等 RBAC role 名 (反 role bleed). 反约束: 反向 grep client/src/ + server-go response payload 字面 `管理员|编辑者|查看者|admin\.role|editor\.role|viewer\.role` 0 hit (除 admin god-mode 路径独立 ACL 系统).

2. **client UI bundle + server response shape SSOT + i18n 字面 SSOT**:
   - **client 改**: `permission 视图组件` 走 capability token 字面渲染 — `packages/client/src/components/PermissionsView.tsx` 改 (~60 行) + `packages/client/src/i18n/capabilities.ts` 新 (~50 行 14 const 各 i18n label SSOT, 反 inline 字面散落)
   - **server 响应 shape**: `/api/v1/me/grants` + `/api/v1/users/{id}/permissions` 返回 `{capabilities: ["messages.write", ...]}` 单源, **不返回** `{role: "editor"}` 字段
   - **i18n SSOT**: 14 const 各 i18n key (e.g. `messages.write` → "发送消息" / "Send messages"), `capabilities.ts` 单源 helper `capabilityLabel(token)` 走 t() 翻译
   - 反约束: client + server `"role"` JSON key 在 permission/grant response 0 hit + i18n key 14 hit per 语言

3. **0 schema / 0 endpoint URL / 0 routes.go / admin god-mode UI 永久独立路径** (跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / RT-3 / DL-2/3 / HB-2 v0(D) 系列承袭): PR diff 仅 (a) client `PermissionsView.tsx` 改 (b) `i18n/capabilities.ts` 新 (c) server 响应 shape 字段映射改 (~30 行 helper) (d) `me_grants.go` / `users_permissions.go` 走 helper. 反约束: 0 endpoint URL / 0 schema / 0 migration v 号 + admin god-mode UI 永久独立 (`/admin-api/users/*` 不走本 helper, ADM-0 §1.3 红线).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **AP2.1 server response shape** | `internal/api/me_grants.go` + `users_permissions.go` 改走 ResponseShape helper (~30 行) returns capabilities 数组不返 role 名; 反向断言 unit test ≥2 (response 不含 role JSON key) |
| **AP2.2 client UI bundle + i18n** | `packages/client/src/i18n/capabilities.ts` 新 (14 const i18n key SSOT, capabilityLabel(token) helper); `PermissionsView.tsx` 改 ~60 行 走 capabilityLabel + vitest ~6 case (14 const 渲染 + 反 role 名 grep + i18n 切换 + admin path 独立) |
| **AP2.3 closure** | REG-AP2-001..006 (8 反向 grep + 14 capability const 不破 + AP-4-enum reflect-lint 不破 + role 名 0 hit + i18n 14 key + admin god-mode 独立 + haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock §1+§2 (i18n 14 key 字面锁 + 反 role 名双语锁) + 4 件套 |

## 2. 反向 grep 锚 (8 反约束)

```bash
# 1) AP-1 14 capability const byte-identical 不破
grep -rcE 'CapabilityMessagesWrite|CapabilityChannelsCreate|CapabilityAgentInvite' packages/server-go/internal/auth/  # ≥3 hit (字面不动)

# 2) AP-4-enum reflect-lint SSOT 不破
git diff origin/main -- packages/server-go/internal/auth/capabilities_lint_test.go | grep -cE '^-|^\+'  # 0 hit

# 3) role 名 0 hit in permission/grant response (反 role bleed)
grep -rE '"role":\s*"(admin|editor|viewer|owner)"' packages/server-go/internal/api/me_grants.go packages/server-go/internal/api/users_permissions.go  # 0 hit
grep -rE '管理员|编辑者|查看者' packages/client/src/components/PermissionsView.tsx packages/client/src/i18n/capabilities.ts  # 0 hit

# 4) i18n 14 key SSOT 单源
grep -cE 'messages\.write|channels\.create|agent\.invite' packages/client/src/i18n/capabilities.ts  # ≥14 hit per 语言

# 5) admin god-mode UI 独立路径 (ADM-0 §1.3 红线)
grep -rE 'capabilityLabel\(' packages/client/src/components/admin/  # 0 hit (admin UI 走独立 ACL helper)

# 6) 0 schema / 0 endpoint URL 改
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '^\+.*HandleFunc|^\+.*Handle\('  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0 hit

# 7) capabilityLabel helper 单源 (反 inline 字面散落)
grep -rE 'function capabilityLabel|export.*capabilityLabel' packages/client/src/  # ==1 hit (SSOT)

# 8) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./... && pnpm vitest run --testTimeout=10000  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **admin role hierarchy UI** — admin 路径独立 ACL 系统 (ADM-0 §1.3 红线)
- ❌ **capability 排序/分组 UI** — 留 v3+ (本 v1 走字母序简单渲染)
- ❌ **per-user capability 编辑 UI** — 走 admin /admin-api/* 独立路径
- ❌ **AP-1 expires_at sweeper UI 显示** — 留 AP-1.bis 续作 (前 stale spec 真值)
- ❌ **i18n 切换 UI** — 假定既有 i18n provider, 本 spec 仅加 14 key

## 4. 跨 milestone byte-identical 锁

- AP-1 #493 14 capability const + AP-4-enum #591 reflect-lint SSOT 不动
- AP-3 #521 cross-org / AP-5 #555 messages ACL helper 不破
- ADM-0 §1.3 admin god-mode 独立 (capability 透明 UI 仅 user-rail)
- NAMING-1 #614 命名规范 (me_grants.go / users_permissions.go)
- 0-行为-改 wrapper 决策树**变体**: 跟 RT-3 / DL-2/3 / HB-2 v0(D) 同源

## 5. 派活 + 双签 + 飞马自审

派 **zhanma-d** (RT-3 #616 client 主战熟手). 飞马 review.

✅ **APPROVED with 2 必修条件**:
🟡 必修-1: scope 推断 (capability 透明 UI), PR body 必明示"等用户拍板再调"
🟡 必修-2: 14 const + AP-4-enum reflect-lint byte-identical 不破

担忧 (1 项): role 名双语 SSOT (英 admin/editor/viewer/owner + 中 管理员/编辑者/查看者) 跟 RT-3 #616 typing-indicator 双语反向 grep 同模式承袭, 战马走双语 grep CI 守.

**ROI 拍**: AP-2 ⭐⭐ — capability 透明真兑现 (反 role bleed). PR review 时用户可调 scope.
