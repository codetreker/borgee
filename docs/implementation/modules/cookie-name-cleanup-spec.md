# cookie-name-cleanup spec brief — auth.CookieName SSOT (≤80 行)

> 战马C · 2026-05-01 · post-#633 admin-spa-shape-fix wave; cookie 字面单源收口 milestone
> 关联: ADM-0.1 cookie 拆 / NAMING-1 / DEFERRED-UNWIND audit真删 / admin-spa-shape-fix #633

> ⚠️ Refactor milestone — **0 cookie 字面值改 (反 user session 失效)** + **0 schema / 0 endpoint / 0 routes** + 仅 production callsite 引用 SSOT + 3 真 drift cleanup (audit-反转: silent-skip + 死代码).

## 0. 关键约束 (3 条立场)

1. **`auth.CookieName = "borgee_token"` SSOT 不动字面** — internal/auth/middleware.go 加 const, mirror admin-rail `internal/admin/auth.go::CookieName="borgee_admin_session"` 模式. 反约束: cookie 字面值 byte-identical 跟现 user session, 反向 grep `"borgee_token"` in production .go (除本 const + ws/client.go 已替) ==0 hit (post-cleanup).

2. **6 production callsite 真改引用 + 30+ test 跟随** (audit-真删立场承袭 NAMING-1 / DEFERRED-UNWIND): production 6 site (`auth/middleware.go` ×2 + `api/auth.go` ×2 + `api/poll.go` ×2 + `ws/client.go` ×1 = 7 真) 全引用 SSOT; testutil/server.go (3) + auth/auth_coverage_test.go + api/auth_test.go (3) + api/error_branches_test.go (8) + api/internal_coverage_test.go (1, +1 真删) 高 leverage 路径走 const, 16 leaf test 留字面 (单测 byte-identical wire bytes, 改 churn>SSOT 边际).

3. **3 真 drift cleanup (audit-反转 + COL-B27 历史标记)**:
   - **D1** `e2e/tests/ap-2-bundle.spec.ts:163` fallback OR 删 — 仅查 `borgee_admin_session`, miss 则 fail-loud throw (反 silent-skip 漂 — §3.2 admin god-mode UI 反向断言 case 真跑)
   - **D2** `internal/api/internal_coverage_test.go:137` 死 cookie line `borgee_admin_token` 真删 (jsonReq helper user-rail only, admin endpoint 不会走 borgee_admin_token, 0 行为改)
   - **D3** `docs/_archive/legacy-tasks/COL-B27/design.md` 加历史标记 (v0.1 草稿写 `borgee_admin_token`, ADM-0.1 #479 改 SSOT 为 `borgee_admin_session`, 设计文 §3+§5+§9+§13 字面是历史草稿残留 — 加 header 标记不动正文)

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **CNC.1 server SSOT + production callsite** | `internal/auth/middleware.go` 加 `CookieName = "borgee_token"` const + 2 callsite refactor; `internal/api/auth.go` 2 callsite (`auth.CookieName`); `internal/api/poll.go` 2 callsite; `internal/ws/client.go` 1 callsite — 全 7 production hit 走 SSOT |
| **CNC.2 高 leverage test fixture 跟随** | testutil/server.go (3 hit) + auth/auth_coverage_test.go (1) + api/auth_test.go (3) + api/error_branches_test.go (8) + api/internal_coverage_test.go (2) — 跨 pkg 用 `auth.CookieName`, auth pkg 内用 `CookieName`. 16 leaf test 留字面 (单测 byte-identical wire bytes) |
| **CNC.3 3 drift cleanup + closure** | D1 ap-2-bundle.spec.ts fail-loud + D2 死 cookie line 真删 + D3 COL-B27 历史标记; REG-CNC-001..006 ⚪→🟢 + acceptance + 4 件套 spec 第一件 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) auth.CookieName SSOT 单源 + 字面值 byte-identical
grep -nE 'const CookieName' packages/server-go/internal/auth/middleware.go  # ==1
grep -nE '"borgee_token"' packages/server-go/internal/auth/middleware.go  # ==1 (仅 SSOT 一行)

# 2) production 7 hit 全走 SSOT (不留 hardcode)
grep -rEc '"borgee_token"' packages/server-go/internal/auth/middleware.go packages/server-go/internal/api/{auth,poll}.go packages/server-go/internal/ws/client.go
# auth/middleware.go ==1 (SSOT) / api/auth.go ==0 / api/poll.go ==0 / ws/client.go ==0

# 3) D1 silent-skip 漂收口 (e2e ap-2 §3.2 真跑)
grep -cE 'borgee_admin_token|admin_token' packages/e2e/tests/ap-2-bundle.spec.ts  # ==0
grep -cE 'throw new Error.*borgee_admin_session' packages/e2e/tests/ap-2-bundle.spec.ts  # ≥1 (fail-loud)

# 4) D2 死 cookie line 真删
grep -cE 'borgee_admin_token' packages/server-go/internal/api/internal_coverage_test.go  # ==0

# 5) D3 COL-B27 历史标记
grep -cE 'COOKIE-NAME-CLEANUP.*历史标记' docs/_archive/legacy-tasks/COL-B27/design.md  # ≥1

# 6) post-#633 haystack gate + 既有 test
go test -tags 'sqlite_fts5' -timeout=300s ./... && pnpm vitest run  # ALL PASS (含 admin auth e2e)
```

## 3. 不在范围 (留账)

- ❌ **cookie 字面值改** (反 user session 全失效)
- ❌ **admin SSOT 改** (`admin/auth.go::CookieName="borgee_admin_session"` 不动)
- ❌ **JWT 签名密钥 / cookie SameSite / Secure / HttpOnly attr 改** (留 v2+)
- ❌ **e2e TS fixture 跟随 SSOT** (TS 不能 import Go const, 留 leaf test 字面)
- ❌ **16 leaf test 字面替换** (跟 SSOT 边际收益<churn, 留 NAMING-2)
- ❌ **COL-B27 design.md 正文重写** (历史草稿不动, 仅加 header 标记)

## 4. 跨 milestone byte-identical 锁

- ADM-0.1 admin-rail SSOT 同模式 (admin/auth.go::CookieName) byte-identical
- NAMING-1 #614 / DEFERRED-UNWIND audit真删 + 字面 cleanup 立场承袭
- admin-spa-shape-fix #633 admin shape SSOT 边界拆死 (本 PR 0 admin cookie 改)
- 蓝图 ADM-0 §1.3 admin/user 路径分叉红线
