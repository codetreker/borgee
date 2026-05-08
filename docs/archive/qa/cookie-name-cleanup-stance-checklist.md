# cookie-name-cleanup PM stance checklist (≤80 行)

> 战马C · 2026-05-01 · 跟 spec brief 1:1 byte-identical
> Owner: 野马 PM 立场 / 飞马 spec / 战马C 实施 / 烈马 验收

## 1. 立场单源 (5 立场, 反约束 byte-identical 跟 spec §0)

- **立场 ①**: `auth.CookieName="borgee_token"` SSOT 字面值 byte-identical 不动 — 反向 grep `"borgee_token"` in production .go 仅 1 hit (SSOT 自己). 改 cookie 字面值 = 全 user session 失效, 用户主权红线.
- **立场 ②**: 7 production callsite (auth/middleware ×2 + api/auth ×2 + api/poll ×2 + ws/client ×1) 全引用 SSOT, 反 hardcode 字面散落.
- **立场 ③**: 高 leverage test fixture (testutil + auth + api/auth_test + error_branches + internal_coverage) 引用 SSOT, 16 leaf test 留字面 (单测 byte-identical wire bytes, 改 churn>边际).
- **立场 ④**: 3 drift cleanup audit-反转 — D1 fail-loud 替 silent-skip (反 §3.2 case 永不跑) + D2 死 cookie line 真删 (反误导 ADM-0 §1.3 红线 review) + D3 COL-B27 历史标记 (反 v0.1 草稿误导新读者).
- **立场 ⑤**: admin SSOT (`admin/auth.go::CookieName="borgee_admin_session"`) 不破 + 边界跟 admin-spa-shape-fix 拆死 (本 PR 0 admin cookie 改).

## 2. 反约束 (4 项)

- ❌ cookie 字面值改 (反 user session 失效红线)
- ❌ JWT secret / SameSite / HttpOnly / Secure attr 改 (留 v2+ session hardening)
- ❌ TS e2e leaf test 跟 Go SSOT (跨语言不可达, 留字面)
- ❌ COL-B27 design.md 正文重写 (历史草稿不动, 仅 header 标记)

## 3. 跨 milestone 锁链 (4 处)

- ADM-0.1 admin-rail SSOT 同模式 (admin/auth.go::CookieName) byte-identical
- admin-spa-shape-fix #633 边界拆死 (本 PR 0 admin cookie 改)
- NAMING-1 #614 / DEFERRED-UNWIND audit真删 + 字面 cleanup 立场承袭
- 蓝图 ADM-0 §1.3 admin/user 路径分叉红线

## 4. PM 拆死决策 (3 段)

- **SSOT vs 字面散落拆死** — production callsite 全 SSOT, 反 hardcode (cookie 字面跨 7 处不漂)
- **高 leverage vs leaf test 拆死** — fixture/helper 走 SSOT, leaf 单测留字面 (反 churn 不带价值)
- **字面 cleanup vs 真 bug 修拆死** — 本 PR 仅字面 + drift cleanup, **0 production 行为改**; admin shape 真 bug 留 admin-spa-shape-fix #633 边界

## 5. 用户主权红线 (4 项)

- ✅ cookie 字面值不动 (user session 不失效)
- ✅ admin/user 路径分叉不混
- ✅ 既有 ACL gate / cookie attr (HttpOnly/Secure/SameSite) byte-identical 不动
- ✅ admin god-mode 不挂 user-rail (ADM-0 §1.3 红线)

## 6. PR 出来 5 核对疑点

1. cookie 字面值 byte-identical (`grep "borgee_token" packages/server-go/internal/auth/middleware.go` ==1 SSOT 一行)
2. 7 production callsite 全引用 SSOT (反 hardcode 0 hit)
3. D1 silent-skip 真收口 (e2e §3.2 真跑, throw fail-loud)
4. D2 死 cookie line 真删 + D3 COL-B27 header 加历史标记
5. 既有 server-go ./... + e2e admin SPA login 全绿 (cookie 不破)

| 2026-05-01 | 战马C | v0 stance checklist — 5 立场 byte-identical 跟 spec brief, 4 反约束 + 4 跨链 + 3 拆死决策 + 4 红线 + 5 PR 核对疑点. 立场承袭 ADM-0.1 同模式 + admin-spa-shape-fix #633 边界拆死. |
