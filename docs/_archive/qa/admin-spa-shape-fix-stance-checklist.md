# ADMIN-SPA-SHAPE-FIX stance checklist v0.draft (≤80 行)

> 野马/飞马联拟 · 2026-05-01 · v0.draft (post-#629/#626 wave 合后转正)
> **关联**: `admin-spa-shape-fix-spec.md.draft` §0..§4 byte-identical 锚 / ADM-0.1/0.2 server SSOT / ADM-2 #484 / ADM-2-FOLLOWUP #626 / AL-8 archived 三态 / CAPABILITY-DOT #628 14 const SSOT
> **立场**: 6 drift 真修 (D1 login P1 user-facing + D2 AdminSession + D3 member_count 死字段 + D4 archived_at 漏 + D5 note 类型 + D6 admin-rail capability gate 漏 prod hardening)

## 1. 立场 (8 项)

- [ ] **§1.1 server SSOT loginRequest+handleMe shape byte-identical 不动 + D4-A/D6 server 仅加 ≤13 行 sanitizer/gate 入口** (跟 spec §0 立场 ① + §4 锁 1): server `loginRequest{Login}` / `handleMe writeJSON {id, login}` 字面不动 (ADM-0.1/0.2 立场承袭); D4-A 加 `store.AdminAction.ArchivedAt *int64` + sanitizeAdminAction nil-safe surface ~5 行; D6 加 IsValidCapability gate ~3 行. 反约束: `git diff origin/main -- packages/server-go/` ≤13 行 production; 0 endpoint URL/schema/routes 改
- [ ] **§1.2 D1 client adminLogin shape 真接 server byte-identical** (跟 spec §1 ASF.1): body `{login, password}`; sig `login: string`; ≤4 callsite 跟随
- [ ] **§1.3 D2 AdminSession 真值修订 byte-identical 跟 server handleMe+handleLogin** (zhanma-e impl notes 真值 auth.go:281,314): client `{id: string, login: string}` byte-identical (2 字段); 反假加 admin_id/expires_at (之前 audit 概念漂); 反 silent drift `{role, username}` 漂回
- [ ] **§1.4 D3 AdminChannel.member_count 死字段真删** (跟 spec §1 ASF.3 + DEFERRED-UNWIND #629 死代码真删模式): server 不返字段 → client 真删 (反 server 加 N+1 query)
- [ ] **§1.5 D4-A server sanitizer surface archived_at + client UI 三态 row class** (走 A 用户决, 跟 spec §1 ASF.2/3): server +5 行 surface (反客 client 加假字段); client UI 三态 (active/archived/soft-deleted) byte-identical 跟 ADM-2-FOLLOWUP #626 不破
- [ ] **§1.6 D6 admin-rail handleGrantPermission IsValidCapability 第 5 处链 byte-identical 守 SSOT** (跟 spec §1 ASF.2 + CAPABILITY-DOT #628 真接): admin.go 加 `if !auth.IsValidCapability { 400 "invalid_capability" }` ~3 行; user-rail 4 处 (me_grants:123 / capability_grant:139 / users:117 / ap_2_capabilities:67) + admin-rail 第 5 处链 byte-identical; admin god-mode 不 bypass (ADM-0 §1.3)
- [ ] **§1.7 D5 InviteCode.note 类型严格 non-null** (跟 spec §1 ASF.3): `note: string` byte-identical 跟 server 返非 null
- [ ] **§1.8 vitest 6 drift 守门 + Go unit gate 真测 (D6) + 0 cookie 名改** (跟 spec §0 立场 ③): `api-shape.test.ts` 6+ case (D1-D5 + endpoint URL); `admin_grant_permission_gate_test.go` D6 4 case (snake reject / dot accept / typo reject / admin god-mode 不 bypass)

## 2. 黑名单 grep (反约束 7 锚)

```bash
# 1) D1 反 client send {username} POST body (post-fix 0 hit)
grep -rnE '"username"\s*[,:]\s*' packages/client/src/admin/ --include='*.ts' --include='*.tsx' | wc -l  # 0 hit

# 2) D2 反 silent drift {role, username} + 反假加 admin_id/expires_at (zhanma-e 真值修订)
grep -rnE 'session\?\.username|adminSession\.username|\.username\s*\)' packages/client/src/admin/  # 0 hit
grep -nE 'admin_id|expires_at' packages/client/src/admin/api.ts  # 0 hit (真值无, 反假加)
grep -nE '"id":\s*[a-z]\.|"login":\s*[a-z]\.' packages/server-go/internal/admin/auth.go  # ≥2 hit (handleMe + handleLogin)

# 3) D3 反 member_count 死字段 (post-cleanup 0 hit production)
grep -nE 'member_count' packages/client/src/admin/  # 0 hit (除测试反向锚)

# 4) D4 archived_at 真补 (server +5 行 sanitizer surface + client UI 三态)
grep -nE 'archived_at' packages/client/src/admin/api.ts packages/client/src/admin/pages/AdminAuditLogPage.tsx  # ≥2 hit (client)
grep -nE 'ArchivedAt' packages/server-go/internal/store/admin_actions.go  # ≥1 hit (D4-A struct)
grep -nE 'archived_at' packages/server-go/internal/api/admin*.go  # ≥1 hit (sanitizeAdminAction surface)

# 5) D5 反 InviteCode.note nullable (post-fix non-null)
grep -nE 'note\?:\s*string\s*\|\s*null' packages/client/src/admin/api.ts  # 0 hit
grep -nE 'note:\s*string\s*;' packages/client/src/admin/api.ts  # ≥1 hit (non-null)

# 6) D6 admin-rail capability gate 真挂 IsValidCapability 入口 (CAPABILITY-DOT #628 真接)
grep -nE 'auth\.IsValidCapability|IsValidCapability\(' packages/server-go/internal/api/admin.go  # ≥1 hit
grep -nE 'invalid_capability' packages/server-go/internal/api/admin.go  # ≥1 hit (字面错码)

# 7) 反 server SSOT 改 + server 改 ≤13 行 production + vitest 6+ case
git diff origin/main -- packages/server-go/internal/admin/auth.go | grep -cE '^[+-].*Login\s+string|loginRequest'  # 0 hit (loginRequest 不动)
grep -cE 'expect.*login|expect.*\.id|expect.*member_count|expect.*archived_at|expect.*\.note' packages/client/src/admin/__tests__/api-shape.test.ts  # ≥5 hit
```

## 3. 不在范围 (跟 spec §3 留账 byte-identical)

- ❌ server loginRequest+handleMe shape 字段名/字段加改 (server SSOT 不动 ADM-0.1/0.2 立场承袭)
- ❌ 新 admin endpoint / cookie 名改 (平行 COOKIE-NAME-CLEANUP) / admin SPA 全 contract audit (留 v2+)
- ❌ schema migration v 号 / endpoint URL / routes 改 (D4-A 仅 store struct + sanitizer, D6 仅 1 gate 行)
- ❌ LoginPage UI 视觉/文案改 / admin god-mode user-rail 漂入 (ADM-0 §1.3 红线)
- ❌ AdminChannel server 加 member_count query (反 N+1, 真删字段更稳)
- ❌ user-rail capability gate 改 (D6 仅 admin-rail handleGrantPermission, user-rail abac 既有 4 处不动)

## 4. 验收挂钩 (跟 spec §1 ASF.3 closure byte-identical)

- [ ] **REG-ASF-001** D1 client adminLogin body shape `{login, password}` byte-identical (vitest mock fetch)
- [ ] **REG-ASF-002** D2 AdminSession `{id, login}` byte-identical 跟 server handleMe+handleLogin (zhanma-e 真值修订, 反假加 admin_id/expires_at)
- [ ] **REG-ASF-003** D3 AdminChannel.member_count 死字段真删 + D4 server +5 行 sanitizer surface archived_at + client UI 三态 row class
- [ ] **REG-ASF-004** D5 InviteCode.note `string` non-null + vitest D1-D5 各 1+ case (≥5 hit)
- [ ] **REG-ASF-005** D6 admin-rail handleGrantPermission IsValidCapability gate 第 5 处链 byte-identical 守 + invalid_capability 字面错码 + Go unit 4 case
- [ ] **REG-ASF-006** server 改 ≤13 行 production (D4-A ~5 + D6 ~3) + 0 cookie 名改 + 0 endpoint URL 改 + 0 schema 改
- [ ] **REG-ASF-007** post-#629/#626 wave 合后 haystack 三轨过 + admin SPA login Playwright e2e 真过 (浏览器路径)

## 5. v0 → v1 transition + 跨 milestone 锁链

v0.draft → wave 合后转正去 .draft. ADM-0.1/0.2 server SSOT + ADM-2 #484 + ADM-2-FOLLOWUP #626 AdminAuditLogPage byte-identical + AL-8 archived 三态 + CAPABILITY-DOT #628 14 const SSOT (D6) + COOKIE-NAME-CLEANUP 平行 + DEFERRED-UNWIND #629 死代码真删模式 + ADM-0 §1.3 红线 + spec §0..§4 byte-identical

| 2026-05-01 | 飞马/野马 | v0.2 stance — 6 drift D1-D6. zhanma-e 真值修订 D2 `{id, login}` (反假加 admin_id/expires_at) + D4 走 A (server +5 行 sanitizer surface) + D6 admin-rail IsValidCapability 第 5 处链 SSOT (zhanma-c 抓). 8 立场 + 7 黑名单 grep + 7 REG. |
