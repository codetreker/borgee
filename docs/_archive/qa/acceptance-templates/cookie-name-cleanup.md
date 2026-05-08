# Acceptance Template — cookie-name-cleanup (≤50 行 draft for feima 4 件套)

> 战马C · 2026-05-01 · `cookie-name-cleanup` milestone 行为不变量验收 v0.draft
> Spec brief: `cookie-name-cleanup-spec.md` (飞马待 commit; 战马 design notes 已 commit)
> Owner: 战马 实施 / 飞马 review / 烈马 验收

> **范围**: 5 处 cookie 名 drift 收口 — server SSOT `admin/auth.go::CookieName="borgee_admin_session"` + 5 test/e2e/comment 字面对齐. **0 server production code 改 / 0 schema / 0 endpoint** 真 test cleanup milestone. 立场承袭 NAMING-1 / DEFERRED-UNWIND audit真删 同精神.

## 验收清单 (跟 spec §1+§2+§3 1:1)

### §1 行为不变量 (CNC.1 const SSOT 不动 + 5 处字面对齐)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 server `internal/admin/auth.go::CookieName` 字面 byte-identical 不动 (`"borgee_admin_session"`) | 行为不变量 | `git diff main -- packages/server-go/internal/admin/auth.go` 0 行改 + `grep -c "borgee_admin_session" packages/server-go/internal/admin/auth.go` ==2 (const + comment) |
| 1.2 RequireAdmin middleware 行为不变 (cookie name 走 admin.CookieName 单源) | 行为不变量 | `internal/admin/middleware.go` 0 行改 + 既有 admin auth_handlers_test.go 全 PASS |
| 1.3 `e2e/tests/ap-2-bundle.spec.ts:163` fallback OR 删 — 仅查 `borgee_admin_session`, miss 则 fail-loud throw (反 silent-skip 漂 — §3.2 admin god-mode UI case 真跑) | 行为不变量 | `grep -cE "borgee_admin_token\|admin_token" packages/e2e/tests/ap-2-bundle.spec.ts` ==0 + e2e §3.2 case 真 PASS (反 silent-return) |
| 1.4 `internal/api/internal_coverage_test.go:137` 死 cookie line 真删 (jsonReq helper 多挂的 borgee_admin_token 无效, 删除 0 行为改) | 行为不变量 | `grep -c "borgee_admin_token" packages/server-go/internal/api/internal_coverage_test.go` ==0 + 既有 jsonReq caller test 全 PASS byte-identical |
| 1.5 `internal/testutil/server.go` legacy comment 字面对齐 (注释 `borgee_admin_token` 改 `borgee_admin_session` 或删历史 footnote) | 行为不变量 | `grep -c "borgee_admin_token" packages/server-go/internal/testutil/server.go` ==0 |

### §2 反向 grep 锚 (CNC.2 全 codebase 字面单源)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 全 codebase `borgee_admin_token` + `admin_token` 字面 0 hit (除 server CookieName const + 1 historical changelog) | 行为不变量 | `grep -rE "borgee_admin_token\|admin_token" packages/ --exclude-dir=node_modules` ==0 production hit |
| 2.2 `borgee_admin_session` 字面 ≥1 hit per: server const + middleware doc + 5+ test (test 跟随 SSOT) | 行为不变量 | `grep -rcE "borgee_admin_session" packages/server-go/internal/admin/ packages/server-go/internal/api/ packages/e2e/tests/` ≥7 hit |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有 server-go ./... + client vitest + e2e 全绿不破 (cookie 改 0 user session 漂) | full test + CI | go test ./... 全 PASS + post-#621 haystack gate TOTAL 85%+ + e2e admin SPA login 真过 |
| 3.2 立场承袭"一次做干净不留尾"用户铁律 + audit-反转 (旧 5 drift 跟 admin-spa-shape-fix Drift 边界拆死, 不混 scope) | inspect | spec §0 立场承袭 + 不动 admin client typegen drift (留 admin-spa-shape-fix wave) |

## REG-CNC-* 占号 (initial ⚪ → 🟢 post-impl)

- REG-CNC-001 🟢 server CookieName const + RequireAdmin middleware byte-identical 不动 (1.1+1.2)
- REG-CNC-002 🟢 ap-2-bundle.spec.ts:163 fallback OR 删 + fail-loud throw (1.3, silent-skip 漂收口)
- REG-CNC-003 🟢 internal_coverage_test.go:137 死 cookie line 真删 (1.4)
- REG-CNC-004 🟢 testutil/server.go legacy comment 对齐 (1.5)
- REG-CNC-005 🟢 全 codebase 反向 grep 0 hit + dot-notation SSOT byte-identical (2.1+2.2)
- REG-CNC-006 🟢 既有 test 全绿 + 0 production code 改 + 立场承袭"一次做干净不留尾" (3.1+3.2)

## 退出条件

- §1 (5) + §2 (2) + §3 (2) 全绿 — 一票否决
- 5 drift 真删/真改 + REG ⚪→🟢 翻
- 0 production code 改 (`git diff main -- packages/server-go/internal/admin/`) 0 行
- post-#621 haystack gate 三轨过 + 既有 e2e admin SPA login 真过

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 战马C | v0.draft — acceptance template 跟 spec §1+§2+§3 1:1, 5 验收 + 2 反向 grep + 2 closure. 立场承袭 NAMING-1 / DEFERRED-UNWIND audit真删 + admin-spa-shape-fix 边界拆死 (cookie-cleanup 仅字面 cleanup, admin-spa-shape-fix 是真 bug 修). |
| 2026-05-01 | 战马C | v1 实施 — 7 production callsite 真改 (auth/middleware ×2 + api/auth ×2 + api/poll ×2 + ws/client ×1) + 高 leverage test (testutil/server + auth/auth_coverage + api/auth_test + api/error_branches + api/internal_coverage) 走 SSOT + D1 ap-2-bundle.spec.ts fail-loud 替 silent-skip + D2 internal_coverage_test.go 死 cookie line 真删 + D3 COL-B27/design.md 加历史标记 header. cookie 字面值 byte-identical 不动 (auth.CookieName="borgee_token"). server-go ./internal/{auth,api,admin,server,ws,testutil}/ 全 PASS. REG-CNC-001..006 ⚪→🟢 全翻. |
