# ADMIN-SPA-SHAPE-FIX spec brief — 6 client/server shape + admin gate drift 真修 (≤80 行)

> 飞马 · 2026-05-01 · v0.2 (post-#629/#626 wave 合后启动) · zhanma-c/e 三轮 audit + zhanma-e impl notes 真值修订
> **关联**: ADM-0.1/0.2 server SSOT · ADM-2 #484 admin god-mode · AL-8 archived 三态 · CAPABILITY-DOT #628 14 const SSOT · COOKIE-NAME-CLEANUP 平行
> **命名**: ADMIN-SPA-SHAPE-FIX = 6 drift (D1-D6) 真修 byte-identical

> ⚠️ 🔴 **6 真 drift, 1 P1 user-facing + 4 silent shape + 1 admin-rail gate hardening**:
> - **D1 login shape** (P1): `api.ts:82-87` send `{username,password}` ≠ server `loginRequest{Login,Password}` → 401 灾难
> - **D2 AdminSession** (zhanma-e 真值修订): server handleMe+handleLogin 真返 `{id, login}` 2 字段 (auth.go:281,314 实读). client `{role, username}` → 改 `{id: string, login: string}` byte-identical 跟 server (移除 role/username, **反假加 admin_id/expires_at** — 之前 audit 概念漂)
> - **D3 AdminChannel.member_count 死字段**: client `member_count?: number` server 不返
> - **D4 AdminAction.archived_at 漏** (走 A 用户决): server store.AdminAction Go struct 实测**无 ArchivedAt 字段** (DB 列存在但 Go 端 0 surfaced) — server 加 ≤5 行 sanitizer surface (反客 client 加假字段); 不改 endpoint URL/schema/business
> - **D5 InviteCode.note 类型不严**: client `note?: string | null` server 返非 null
> - **D6 admin-rail capability gate** (prod hardening, zhanma-c 抓): `admin.go::handleGrantPermission` 0 调 `auth.IsValidCapability` → admin cURL 塞任意字面蔓延 (#628 backfill 守存量入口未守); user-rail 4 处全验 (me_grants:123 / capability_grant:139 / users:117 / ap_2_capabilities:67), admin-rail 是第 5 处链 SSOT 守

## 0. 关键约束 (3 条立场)

1. **server SSOT loginRequest+handleMe shape byte-identical 不动 + D4-A/D6 server diff ≤13 行 (D4 sanitizer +5 + D6 gate +3 + struct field +5)** (ADM-0.1/0.2 立场承袭): D4 加 `store.AdminAction.ArchivedAt *int64` + sanitizeAdminAction nil-safe surface ~5 行; D6 加 IsValidCapability gate ~3 行 + 1 unit; **0 endpoint URL / 0 schema / 0 routes / 0 业务逻辑改**. 反约束: `git diff origin/main -- packages/server-go/` ≤13 行 production; 0 migration v 号.

2. **client 改最小补丁 + 5 interface byte-identical**: D1 adminLogin body+sig / D2 AdminSession `{id, login}` byte-identical / D3 member_count 死字段真删 / D4 client UI 加三态 row class (server sanitizer surface archived_at) / D5 InviteCode.note `string` non-null. 反约束: `"username"` in client/admin/ 0 hit; **`admin_id|expires_at` in api.ts 0 hit (D2 真值修订)**; `id|login` ≥2 hit.

3. **vitest 6 drift 守门 + Go unit gate 真测 (D6) + ADM-0 §1.3 红线**: `api-shape.test.ts` 6+ vitest (D1-D5 + endpoint URL); `admin_grant_permission_gate_test.go` D6 4 case (snake reject / dot accept / typo reject / admin god-mode 不 bypass).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **ASF.1 D1+D2 (auth-shape, client only ≤15 行)** | `api.ts::adminLogin` body `{login,password}` + sig `login:string` (D1, ≤4 callsite); `AdminSession` 重写 `{id:string, login:string}` byte-identical 跟 server handleMe+handleLogin (D2) — 移除 role/username, **反假加 admin_id/expires_at**; auth.ts/LoginPage.tsx 跟随 |
| **ASF.2 D4-A server sanitizer + D6 admin gate (server ≤13 行)** | D4: `store/admin_actions.go::AdminAction` 加 `ArchivedAt *int64 \`gorm:"column:archived_at" json:"-"\``; sanitizeAdminAction 加 `if row.ArchivedAt != nil { out["archived_at"] = *row.ArchivedAt }` ~5 行; D6: `internal/api/admin.go::handleGrantPermission` 加 `if !auth.IsValidCapability(body.Permission) { 400 "invalid_capability" }` ~3 行 + `admin_grant_permission_gate_test.go` 4 case |
| **ASF.3 D3+D4 client + D5 + 6 vitest + closure** | D3 `AdminChannel.member_count` 死字段真删; D4 client `AdminAction` 加 `archived_at?: number\|null` + AdminAuditLogPage.tsx 三态 row class byte-identical 跟 ADM-2-FOLLOWUP #626 不破; D5 `InviteCode.note: string` non-null; `api-shape.test.ts` 6+ case; REG-ASF-001..007 + acceptance + 4 件套 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) D1 client send {login} + 反 {username}
grep -nE 'JSON\.stringify\(\s*\{\s*login\s*[,:]' packages/client/src/admin/api.ts  # ≥1
grep -rnE '"username"' packages/client/src/admin/  # 0 hit

# 2) D2 AdminSession 真值 byte-identical (zhanma-e 修订: 反假加 admin_id/expires_at)
grep -nE '"id":\s*[a-z]\.|"login":\s*[a-z]\.' packages/server-go/internal/admin/auth.go  # ≥2 (handleMe+handleLogin)
grep -nE 'admin_id|expires_at' packages/client/src/admin/api.ts  # 0 hit (真值无)
grep -nE '\bid:\s*string;\s*login:\s*string' packages/client/src/admin/api.ts  # ≥1 (D2 byte-identical)

# 3) D3 死字段真删 + D4 client+server archived_at 真补
grep -nE 'member_count' packages/client/src/admin/  # 0 hit
grep -nE 'archived_at' packages/client/src/admin/api.ts packages/client/src/admin/pages/AdminAuditLogPage.tsx  # ≥2
grep -nE 'ArchivedAt' packages/server-go/internal/store/admin_actions.go  # ≥1 (D4-A)
grep -nE 'archived_at' packages/server-go/internal/api/admin*.go  # ≥1 (sanitizeAdminAction surface)

# 4) D5 non-null + D6 admin gate IsValidCapability
grep -nE 'note:\s*string\s*;' packages/client/src/admin/api.ts  # ≥1
grep -nE 'auth\.IsValidCapability' packages/server-go/internal/api/admin.go  # ≥1 (D6 admin-rail 第 5 处)
grep -nE 'invalid_capability' packages/server-go/internal/api/admin.go  # ≥1 (字面错码)

# 5) server 改 ≤13 行 production + 0 endpoint URL / 0 schema 改
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0
git diff origin/main -- packages/server-go/internal/{api/server.go,server/server.go} | grep -cE 'HandleFunc|Mount|Routes'  # 0

# 6) post-#629 haystack + 6+ vitest + Playwright admin
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
pnpm vitest run && pnpm exec playwright test -g 'admin'  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ server loginRequest 字段名改 / 新 admin endpoint / cookie 名改 (平行 COOKIE-NAME-CLEANUP) / admin SPA 全 contract audit (留 v2+) / LoginPage UI 视觉文案改 / admin god-mode user-rail 漂入 (ADM-0 §1.3 红线) / AdminChannel server 加 member_count query (反 N+1) / user-rail capability gate 改 (D6 仅 admin-rail)

## 4. 跨 milestone byte-identical 锁

ADM-0.1/0.2 server SSOT (loginRequest + handleMe `{id,login}` byte-identical) + ADM-2 #484 / ADM-2-FOLLOWUP #626 AdminAuditLogPage 跟随 + AL-8 archived (D4-A server sanitizer surface) + CAPABILITY-DOT #628 14 const SSOT (D6 admin-rail 第 5 处链, 跟 user-rail 4 处 byte-identical) + COOKIE-NAME-CLEANUP 平行 + ADM-0 §1.3 红线
