# Cookie Name 单一来源 — user-rail (≤30 行)

> 落地: `feat/cookie-name-cleanup` (post-#633 admin-spa-shape-fix wave). user-rail session cookie 字面单一来源 + audit-反转 cleanup.
> 关联: admin-rail 单一来源 `internal/admin/auth.go::CookieName="borgee_admin_session"` (mirror 模式)

## 1. 单一来源 — `internal/auth/middleware.go`

```go
// CookieName is the user-rail session cookie literal 单一来源 (ADM-0.1 +
// COOKIE-NAME-CLEANUP). Mirror of admin-rail `internal/admin/auth.go::
// CookieName="borgee_admin_session"`. Keep the literal value here; refactor
// callsites to use this const so any future rename touches one line.
const CookieName = "borgee_token"
```

## 2. 7 production callsite (全引用单一来源)

- `internal/auth/middleware.go::AuthMiddleware` — `r.Cookie(CookieName)`
- `internal/auth/middleware.go::AuthenticateFlexible` — `r.Cookie(CookieName)`
- `internal/api/auth.go::handleLogin` — `Name: auth.CookieName` (Set-Cookie)
- `internal/api/auth.go::handleLogout` — `Name: auth.CookieName` (clear)
- `internal/api/poll.go::handlePoll` ×2 — `r.Cookie(auth.CookieName)`
- `internal/ws/client.go::ServeWS` — `r.Cookie(auth.CookieName)`

## 3. 反向约束

- ❌ cookie 字面值改 (反 user session 全失效红线)
- ❌ JWT secret / SameSite / HttpOnly / Secure attr 改 (留 v2+ session hardening)
- ❌ admin 单一来源 跟 user 单一来源 混 (admin/auth.go::CookieName 字面值不同, 拆死)
- ❌ admin god-mode 走 user CookieName (ADM-0 §1.3 红线)

## 4. 测试

- `internal/auth/auth_coverage_test.go` + `internal/api/auth_test.go` + `internal/api/error_branches_test.go` + `internal/api/internal_coverage_test.go` + `internal/testutil/server.go` 高 leverage fixture 全引用单一来源
- 16 leaf test 留字面 (单测 byte-identical wire bytes, 改 churn>单一来源 边际)
