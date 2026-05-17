package auth

// TEST-FIX-3-COV: ContextWithUser 0% — direct call.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

func TestContextWithUser_RoundTrip(t *testing.T) {
	t.Parallel()
	u := &store.User{ID: "user-x", DisplayName: "X", Role: "member"}
	ctx := ContextWithUser(context.Background(), u)
	got := UserFromContext(ctx)
	if got == nil {
		t.Fatal("UserFromContext: nil after ContextWithUser")
	}
	if got.ID != "user-x" {
		t.Fatalf("UserFromContext: got %q want user-x", got.ID)
	}
	// Empty ctx round-trip → nil.
	if UserFromContext(context.Background()) != nil {
		t.Fatal("UserFromContext: expected nil for empty ctx")
	}
}

// TestAuthMiddleware_ShortCircuitsOnCtxUser pins the perf fix: when an
// upstream middleware (rate-limit) has already authenticated and injected
// the user into the request ctx via ContextWithUser, AuthMiddleware MUST
// skip the cookie/Bearer/dev-bypass paths entirely — saving the duplicate
// GetUserByID DB lookup per authenticated request.
//
// 反 X: 验证 cookie 是已知无效的 JWT — 如果短路逻辑漏掉, 中间件会走 cookie
// 分支, ValidateJWT 失败 → 401, handler 不会被调用. 期望: 短路命中, handler
// 跑, 拿到 ctx 里塞的 user.
func TestAuthMiddleware_ShortCircuitsOnCtxUser(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	user := &store.User{ID: "sc-user", DisplayName: "Short Circuit", Role: "member"}
	if err := s.CreateUser(user); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{JWTSecret: "test-secret", NodeEnv: "development"}

	handlerCalled := false
	var seen *store.User
	handler := AuthMiddleware(s, cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		seen = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	// 故意塞已知无效 JWT — 如果短路没命中, ValidateJWT 会失败, 中间件 401.
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "not-a-valid-jwt"})
	// 上游 (rate-limit) 已塞的 user.
	req = req.WithContext(ContextWithUser(req.Context(), user))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 via short-circuit, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !handlerCalled {
		t.Fatal("handler not called — short-circuit failed, middleware likely ran auth path on invalid cookie")
	}
	if seen == nil || seen.ID != user.ID {
		t.Fatalf("expected handler to see ctx user %q, got %+v", user.ID, seen)
	}
}

