package auth

// TEST-FIX-3-COV: ContextWithUser 0% — direct call.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"borgee-server/internal/config"
	"borgee-server/internal/store"

	"github.com/golang-jwt/jwt/v5"
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

// TestValidateJWT_RejectsNonHS256 pins F5 (#1108): the JWT parser MUST pin the
// signing algorithm to HS256. Pre-fix the keyfunc returns []byte(secret) for
// ANY signing method, so a token forged with HS512 (using the same secret)
// produces a structurally valid signature and ValidateJWT happily returns a
// user — an attacker who learns the symmetric secret could also pivot through
// algorithm confusion. After pinning via jwt.WithValidMethods([]{"HS256"}),
// the parser rejects HS512 before the keyfunc result is trusted.
//
// 反 X: HS512 token 用同一 secret 签名 (签名本身有效) → 必须仍被拒 (返回 nil),
// 证明拒绝来自算法 pin 而非签名校验. HS256 正常 token 不受影响 (无回归).
func TestValidateJWT_RejectsNonHS256(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	user := &store.User{ID: "alg-user", DisplayName: "Alg", Role: "member"}
	if err := s.CreateUser(user); err != nil {
		t.Fatal(err)
	}

	const secret = "test-secret"
	claims := &Claims{UserID: user.ID, Email: "alg@example.com"}

	// RED: forge a token with HS512 using the SAME secret. The signature is
	// valid under the keyfunc (which returns []byte(secret) regardless of alg),
	// so pre-fix ValidateJWT returns the user; post-fix the parser errors with
	// "signing method HS512 is invalid".
	hs512Token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	hs512Str, err := hs512Token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if got := ValidateJWT(s, secret, hs512Str); got != nil {
		t.Fatalf("ValidateJWT accepted an HS512-signed token (alg confusion); got user %q, want nil", got.ID)
	}

	// GREEN companion: a normally-signed HS256 token still validates — no
	// regression on the legitimate path.
	hs256Token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	hs256Str, err := hs256Token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	got := ValidateJWT(s, secret, hs256Str)
	if got == nil {
		t.Fatal("ValidateJWT rejected a valid HS256 token; expected the user")
	}
	if got.ID != user.ID {
		t.Fatalf("ValidateJWT: got user %q want %q", got.ID, user.ID)
	}
}

