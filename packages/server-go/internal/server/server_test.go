package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/config"
	"borgee-server/internal/store"
	"borgee-server/internal/ws"

	"github.com/gorilla/websocket"
)

type flushResponseRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushResponseRecorder) Flush() {
	r.flushed = true
}

func init() {
	// TEST-FIX-3-COV: ADM-0.2 admin.Bootstrap fail-loud env, set once at
	// package init so tests can use t.Parallel (t.Setenv blocks parallel).
	os.Setenv("BORGEE_ADMIN_LOGIN", "test-admin")
	os.Setenv("BORGEE_ADMIN_PASSWORD_HASH", "$2a$10$1TyjYX4YfwjnX5EpcGsH2uY5IUVuZZm4HFZBtMz1m5yBO4qM9Ulr6")
}

// testRateLimitCfg 给 rate-limit unit test 用的最小 cfg, 不走 Load().
func testRateLimitCfg() *config.Config {
	return &config.Config{
		RateLimitAuthPerSec: 5,
		RateLimitAuthBurst:  15,
		RateLimitUserPerSec: 20,
		RateLimitUserBurst:  60,
		RateLimitAnonPerSec: 100,
		RateLimitAnonBurst:  300,
	}
}

func testServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })

	cfg := &config.Config{
		JWTSecret:     "test-secret",
		NodeEnv:       "development",
		DevAuthBypass: false,
		UploadDir:     t.TempDir(),
		WorkspaceDir:  t.TempDir(),
		ClientDist:    t.TempDir(),
		CORSOrigin:    "*",

		RateLimitAuthPerSec: 5,
		RateLimitAuthBurst:  15,
		RateLimitUserPerSec: 20,
		RateLimitUserBurst:  60,
		RateLimitAnonPerSec: 100,
		RateLimitAnonBurst:  300,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(t.Context(), cfg, logger, s)
	return srv, s
}

func TestHealth(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStaticFallback(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)

	indexPath := filepath.Join(srv.cfg.ClientDist, "index.html")
	os.WriteFile(indexPath, []byte("<html></html>"), 0644)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/some-route")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for SPA fallback, got %d", resp.StatusCode)
	}
}

func TestStaticFile(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)

	os.WriteFile(filepath.Join(srv.cfg.ClientDist, "test.js"), []byte("var x=1;"), 0644)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for static file, got %d", resp.StatusCode)
	}
}

func TestNotFoundAPI(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCORSHeaders(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/v1/channels", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	acao := resp.Header.Get("Access-Control-Allow-Origin")
	if acao == "" {
		t.Fatal("expected CORS header")
	}
}

func TestCORSProductionAllowedOrigin(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := corsMiddleware(false, "https://app.example", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusAccepted)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || !nextCalled {
		t.Fatalf("expected next handler status 202, got %d next=%v", rec.Code, nextCalled)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("expected allowed origin header, got %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("expected X-Content-Type-Options: nosniff")
	}
}

// nonDevServer builds a server whose config is NOT in development mode, so the
// HSTS header (gated to non-dev) is emitted. Mirrors testServer otherwise.
func nonDevServer(t *testing.T) *Server {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })

	cfg := &config.Config{
		JWTSecret:     "test-secret",
		NodeEnv:       "production",
		DevAuthBypass: false,
		UploadDir:     t.TempDir(),
		WorkspaceDir:  t.TempDir(),
		ClientDist:    t.TempDir(),
		CORSOrigin:    "https://example.test",

		RateLimitAuthPerSec: 5,
		RateLimitAuthBurst:  15,
		RateLimitUserPerSec: 20,
		RateLimitUserBurst:  60,
		RateLimitAnonPerSec: 100,
		RateLimitAnonBurst:  300,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(t.Context(), cfg, logger, s)
}

// TestSecurityHeadersCSP_HSTS asserts the #1108 frontend-F4 hardening headers:
// Content-Security-Policy, Permissions-Policy, and HSTS (the last gated to
// non-development mode).
func TestSecurityHeadersCSP_HSTS(t *testing.T) {
	t.Parallel()

	t.Run("non-dev emits CSP, Permissions-Policy and HSTS", func(t *testing.T) {
		t.Parallel()
		srv := nonDevServer(t)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		csp := resp.Header.Get("Content-Security-Policy")
		if csp == "" {
			t.Fatal("expected non-empty Content-Security-Policy header")
		}
		for _, want := range []string{
			"default-src 'self'",
			"frame-ancestors 'none'",
			"object-src 'none'",
		} {
			if !strings.Contains(csp, want) {
				t.Fatalf("expected CSP to contain %q, got %q", want, csp)
			}
		}
		if !strings.Contains(csp, "connect-src") || !strings.Contains(csp, "wss:") {
			t.Fatalf("expected CSP connect-src to include wss:, got %q", csp)
		}
		if !strings.Contains(csp, "style-src") || !strings.Contains(csp, "'unsafe-inline'") {
			t.Fatalf("expected CSP style-src to include 'unsafe-inline', got %q", csp)
		}

		pp := resp.Header.Get("Permissions-Policy")
		if !strings.Contains(pp, "camera=()") || !strings.Contains(pp, "microphone=()") {
			t.Fatalf("expected Permissions-Policy to disable camera and microphone, got %q", pp)
		}

		if got := resp.Header.Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
			t.Fatalf("expected HSTS max-age=31536000; includeSubDomains, got %q", got)
		}
	})

	t.Run("dev omits HSTS but keeps CSP", func(t *testing.T) {
		t.Parallel()
		srv, _ := testServer(t) // testServer default is NodeEnv=development
		if !srv.cfg.IsDevelopment() {
			t.Fatal("precondition: testServer cfg must be development")
		}
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if csp := resp.Header.Get("Content-Security-Policy"); csp == "" {
			t.Fatal("expected CSP to be present even in dev mode")
		}
		if got := resp.Header.Get("Strict-Transport-Security"); got != "" {
			t.Fatalf("expected HSTS to be empty in dev mode, got %q", got)
		}
	})
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]string{"test": "value"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test") {
		t.Fatal("expected body to contain 'test'")
	}
}

func TestJSONError(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	JSONError(rec, http.StatusBadRequest, "bad request")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestReadJSON(t *testing.T) {
	t.Parallel()
	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/", body)
	var dst map[string]string
	err := ReadJSON(req, &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst["key"] != "value" {
		t.Fatal("expected value")
	}
}

func TestReadJSON_Invalid(t *testing.T) {
	t.Parallel()
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/", body)
	var dst map[string]string
	err := ReadJSON(req, &dst)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadJSON_TooLarge(t *testing.T) {
	t.Parallel()
	body := strings.NewReader(`{"payload":"` + strings.Repeat("x", 1<<20) + `"}`)
	req := httptest.NewRequest("POST", "/", body)
	var dst map[string]string
	err := ReadJSON(req, &dst)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected too large error, got %v", err)
	}
}

func TestParseIDParam(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/", nil)
	id := ParseIDParam(req, "id")
	if id != "" {
		t.Fatal("expected empty string")
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	reqID := resp.Header.Get("X-Request-Id")
	if reqID == "" {
		t.Fatal("expected X-Request-Id header")
	}
}

func TestRequestIDFromContextMissing(t *testing.T) {
	t.Parallel()
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty request id, got %q", got)
	}
}

func TestRecoverMiddlewareWritesErrorOnPanic(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := recoverMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Internal server error") {
		t.Fatalf("expected error body, got %q", rec.Body.String())
	}
}

func TestStatusRecorderFlush(t *testing.T) {
	t.Parallel()
	base := &flushResponseRecorder{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: base, status: http.StatusOK}
	rec.WriteHeader(http.StatusCreated)
	rec.Flush()

	if rec.status != http.StatusCreated {
		t.Fatalf("expected recorded status 201, got %d", rec.status)
	}
	if !base.flushed {
		t.Fatal("expected underlying flusher to be called")
	}
	if rec.Unwrap() != base {
		t.Fatal("expected unwrap to return underlying response writer")
	}
}

func TestRateLimiter(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(t.Context(), testRateLimitCfg())
	ip := "127.0.0.1"

	for i := 0; i < 10; i++ {
		if !rl.allow("anon:"+ip, rl.anonRate, rl.anonMax) {
			t.Fatal("should allow within rate limit")
		}
	}
}

func TestRateLimiterUsesAuthBucket(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(t.Context(), testRateLimitCfg())
	rl.authRate = 0
	rl.authMax = 1
	rl.anonMax = 0
	ip := "198.51.100.12"

	if !rl.allow("auth:"+ip, rl.authRate, rl.authMax) {
		t.Fatal("expected first auth request to be allowed")
	}
	if rl.allow("auth:"+ip, rl.authRate, rl.authMax) {
		t.Fatal("expected exhausted auth bucket to reject request")
	}
}

func TestRateLimitMiddlewareRejectsExhaustedClient(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	rl := newRateLimiter(t.Context(), srv.cfg)
	rl.anonRate = 0
	rl.anonMax = 1

	nextCalls := 0
	cfg := &config.Config{NodeEnv: ""} // not dev → no bypass
	handler := rateLimitMiddleware(rl, srv.store, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest("GET", "/api/v1/channels", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected first request accepted, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limited response, got %d", rec.Code)
	}
	if nextCalls != 1 {
		t.Fatalf("expected next called once, got %d", nextCalls)
	}
}

// TestRateLimitBypass_RequiresBothHeaderAndDevMode pins the two-gate e2e bypass:
// only `IsDevelopment=true` AND `X-E2E-Test: 1` together skip the limiter.
// Either gate alone (header in prod / dev without header / both off) MUST
// fall through to the normal rate-limit path.
//
// Why both gates are required:
//   - header alone is forgeable from outside in prod → would be a DoS-bypass hole
//   - dev mode alone weakens local development hygiene (real browser tab traffic
//     would silently bypass the limiter, masking real client bugs)
//
// See middleware.go:rateLimitMiddleware doc comment for the full rationale.
func TestRateLimitBypass_RequiresBothHeaderAndDevMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		isDevelopment bool
		header        string
		// expectBypass = true means the second request (after exhaustion)
		// should still be served with 202 (limiter skipped). false means
		// the limiter rejects with 429 as usual.
		expectBypass bool
	}{
		{name: "dev + header → bypass", isDevelopment: true, header: "1", expectBypass: true},
		{name: "dev only (no header) → enforce", isDevelopment: true, header: "", expectBypass: false},
		{name: "header only (prod) → enforce", isDevelopment: false, header: "1", expectBypass: false},
		{name: "neither → enforce", isDevelopment: false, header: "", expectBypass: false},
		// Defensive: stray header values must not be treated as the magic "1".
		{name: "dev + header=true (not the literal 1) → enforce", isDevelopment: true, header: "true", expectBypass: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := testServer(t)
			rl := newRateLimiter(t.Context(), srv.cfg)
			rl.anonRate = 0
			rl.anonMax = 1

			cfg := &config.Config{}
			if tc.isDevelopment {
				cfg.NodeEnv = "development"
			}
			handler := rateLimitMiddleware(rl, srv.store, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			}))

			mkReq := func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/channels", nil)
				req.RemoteAddr = "203.0.113.42:1234"
				if tc.header != "" {
					req.Header.Set("X-E2E-Test", tc.header)
				}
				return req
			}

			// First request: always succeeds (bucket has 1 token, or bypass).
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, mkReq())
			if rec.Code != http.StatusAccepted {
				t.Fatalf("first request: expected 202, got %d", rec.Code)
			}

			// Second request: bypass cases stay 202; enforced cases hit 429.
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, mkReq())
			if tc.expectBypass {
				if rec.Code != http.StatusAccepted {
					t.Fatalf("second request: expected bypass (202), got %d", rec.Code)
				}
			} else {
				if rec.Code != http.StatusTooManyRequests {
					t.Fatalf("second request: expected 429 (limiter enforced), got %d", rec.Code)
				}
			}
		})
	}
}

// TestRateLimitThreeTier_BucketSelection 覆盖三档桶切换:
//
//	(a) auth path → auth 桶 (per-IP)
//	(b) 登录用户访问非 auth path → user 桶 (per-user_id)
//	(c) 登录用户访问 auth path 仍走 auth 桶 (防爆破)
//	(d) 匿名 → anon 桶 (per-IP)
//	(e) 同 user 不同 IP 共享 user 桶
//	(f) 不同 user 互不影响
func TestRateLimitThreeTier_BucketSelection(t *testing.T) {
	t.Parallel()
	_, s := testServer(t)

	// 两个真用户, 各自配 API key.
	emailA := "alice@test.com"
	keyA := "rl-test-key-alice-aaaaaaaaaaaaaaaa"
	userA := &store.User{DisplayName: "alice", Role: "member", Email: &emailA, PasswordHash: "h", APIKey: &keyA}
	if err := s.CreateUser(userA); err != nil {
		t.Fatal(err)
	}
	emailB := "bob@test.com"
	keyB := "rl-test-key-bob-bbbbbbbbbbbbbbbbb"
	userB := &store.User{DisplayName: "bob", Role: "member", Email: &emailB, PasswordHash: "h", APIKey: &keyB}
	if err := s.CreateUser(userB); err != nil {
		t.Fatal(err)
	}

	// cfg: 关 dev 防 X-E2E-Test bypass 走偏; 桶很小好观测.
	cfg := &config.Config{
		JWTSecret:           "test-secret",
		NodeEnv:             "", // not dev
		RateLimitAuthPerSec: 0,
		RateLimitAuthBurst:  1,
		RateLimitUserPerSec: 0,
		RateLimitUserBurst:  2,
		RateLimitAnonPerSec: 0,
		RateLimitAnonBurst:  3,
	}
	rl := newRateLimiter(t.Context(), cfg)

	handler := rateLimitMiddleware(rl, s, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	mk := func(path, ip, apiKey string) *http.Request {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = ip + ":12345"
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		return req
	}
	do := func(req *http.Request) int {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	// (a) auth path anon — burst=1, 第二请求 429
	if c := do(mk("/api/v1/auth/login", "203.0.113.1", "")); c != http.StatusAccepted {
		t.Fatalf("auth #1 expected 202, got %d", c)
	}
	if c := do(mk("/api/v1/auth/login", "203.0.113.1", "")); c != http.StatusTooManyRequests {
		t.Fatalf("auth #2 expected 429, got %d", c)
	}
	// /admin-api/auth/ 同样走 auth 桶, 同 IP 仍然 429 (桶 key 是 auth:<ip>)
	if c := do(mk("/admin-api/auth/login", "203.0.113.1", "")); c != http.StatusTooManyRequests {
		t.Fatalf("admin-api/auth same-IP expected 429, got %d", c)
	}

	// (b) 登录用户访问非 auth path → user 桶 (burst=2)
	if c := do(mk("/api/v1/channels", "10.0.0.1", keyA)); c != http.StatusAccepted {
		t.Fatalf("userA #1 expected 202, got %d", c)
	}
	if c := do(mk("/api/v1/channels", "10.0.0.1", keyA)); c != http.StatusAccepted {
		t.Fatalf("userA #2 expected 202, got %d", c)
	}
	// (e) 同 user 换 IP — 应共享 user 桶, 已被 a 耗光 → 429
	if c := do(mk("/api/v1/channels", "10.0.0.99", keyA)); c != http.StatusTooManyRequests {
		t.Fatalf("userA different IP should share user bucket → 429, got %d", c)
	}

	// (f) 不同 user 互不影响 — userB 还有满 burst=2
	if c := do(mk("/api/v1/channels", "10.0.0.1", keyB)); c != http.StatusAccepted {
		t.Fatalf("userB #1 (separate bucket) expected 202, got %d", c)
	}
	if c := do(mk("/api/v1/channels", "10.0.0.1", keyB)); c != http.StatusAccepted {
		t.Fatalf("userB #2 expected 202, got %d", c)
	}
	if c := do(mk("/api/v1/channels", "10.0.0.1", keyB)); c != http.StatusTooManyRequests {
		t.Fatalf("userB #3 expected 429, got %d", c)
	}

	// (c) 登录用户访问 auth path — 仍走 auth 桶 (按 IP)
	// 用一个全新 IP 防跟 (a) 的 auth:<ip> 撞.
	if c := do(mk("/api/v1/auth/logout", "198.18.0.1", keyA)); c != http.StatusAccepted {
		t.Fatalf("authed user on auth path #1 expected 202, got %d", c)
	}
	if c := do(mk("/api/v1/auth/logout", "198.18.0.1", keyA)); c != http.StatusTooManyRequests {
		t.Fatalf("authed user on auth path #2 expected 429 (auth bucket per-IP), got %d", c)
	}

	// (d) 匿名非 auth path → anon 桶 (burst=3)
	for i := 0; i < 3; i++ {
		if c := do(mk("/api/v1/channels", "172.16.0.1", "")); c != http.StatusAccepted {
			t.Fatalf("anon #%d expected 202, got %d", i+1, c)
		}
	}
	if c := do(mk("/api/v1/channels", "172.16.0.1", "")); c != http.StatusTooManyRequests {
		t.Fatalf("anon #4 expected 429, got %d", c)
	}
	// 不同 IP 的 anon 桶独立
	if c := do(mk("/api/v1/channels", "172.16.0.2", "")); c != http.StatusAccepted {
		t.Fatalf("anon different IP separate bucket expected 202, got %d", c)
	}
}

// TestRateLimitThreeTier_InvalidTokenFallsBackToAnon 探测 token 无效时
// fall back IP 桶, 不 panic.
func TestRateLimitThreeTier_InvalidTokenFallsBackToAnon(t *testing.T) {
	t.Parallel()
	_, s := testServer(t)

	cfg := &config.Config{
		JWTSecret:           "test-secret",
		NodeEnv:             "",
		RateLimitAuthPerSec: 0,
		RateLimitAuthBurst:  1,
		RateLimitUserPerSec: 0,
		RateLimitUserBurst:  1,
		RateLimitAnonPerSec: 0,
		RateLimitAnonBurst:  2,
	}
	rl := newRateLimiter(t.Context(), cfg)
	handler := rateLimitMiddleware(rl, s, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	mk := func() *http.Request {
		req := httptest.NewRequest("GET", "/api/v1/channels", nil)
		req.RemoteAddr = "192.0.2.50:1234"
		req.Header.Set("Authorization", "Bearer not-a-real-key-XXXXXXXXXXXXXXXXXX")
		return req
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, mk())
	if rec.Code != http.StatusAccepted {
		t.Fatalf("anon #1 (bad token) expected 202, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, mk())
	if rec.Code != http.StatusAccepted {
		t.Fatalf("anon #2 expected 202, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, mk())
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("anon #3 expected 429 (fell back to anon bucket burst=2), got %d", rec.Code)
	}
}

// TestClientIPSources pins the trusted-proxy-aware clientIP semantics (#1108 F2).
//
// This test was deliberately rewritten away from the OLD blind-trust behavior
// (which returned X-Forwarded-For[0], then X-Real-IP). Under the new model the
// default (trustedProxyCount=0) ignores those client-controlled headers entirely
// and uses host(RemoteAddr); only trustedProxyCount>0 reads the XFF chain, and
// then only the rightmost N trusted hops — an attacker's left-injected entries
// are never returned.
func TestClientIPSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		setup             func(*http.Request)
		remote            string
		trustedProxyCount int
		want              string
	}{
		{
			// Default (count=0): XFF must be IGNORED, RemoteAddr host wins.
			name: "default ignores forwarded-for",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", " 198.51.100.7, 198.51.100.8")
			},
			remote:            "10.0.0.1:1111",
			trustedProxyCount: 0,
			want:              "10.0.0.1",
		},
		{
			// Default (count=0): X-Real-IP must be IGNORED too (second bypass path).
			name: "default ignores real-ip",
			setup: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "198.51.100.9")
			},
			remote:            "10.0.0.1:1111",
			trustedProxyCount: 0,
			want:              "10.0.0.1",
		},
		{
			name:              "default remote without port",
			setup:             func(r *http.Request) {},
			remote:            "198.51.100.10",
			trustedProxyCount: 0,
			want:              "198.51.100.10",
		},
		{
			// count=1: chain = [fake, realclient, proxy(XFF)] ++ [RemoteAddr].
			// Trust rightmost 1 hop (RemoteAddr); idx=len-1-1 picks the proxy entry.
			// Here the last XFF hop IS the real edge proxy and RemoteAddr is the LB.
			name: "count=1 picks entry left of trusted hop, ignores left injection",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.5, 70.70.70.70")
			},
			remote:            "10.0.0.1:1111",
			trustedProxyCount: 1,
			want:              "70.70.70.70",
		},
		{
			// count=1 with a single real client hop in XFF:
			// chain = [203.0.113.50] ++ [10.0.0.1]; idx=len-1-1=0 → real client.
			// An attacker who prepends a fake left entry cannot reach this slot.
			name: "count=1 single hop returns real client",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.50")
			},
			remote:            "10.0.0.1:1111",
			trustedProxyCount: 1,
			want:              "203.0.113.50",
		},
		{
			// count exceeds chain length → clamp to chain[0] (safe degrade,
			// never returns an arbitrary attacker-chosen value past the chain).
			name: "count exceeds chain clamps to leftmost",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.60")
			},
			remote:            "10.0.0.1:1111",
			trustedProxyCount: 9,
			want:              "203.0.113.60",
		},
		{
			// count>0 but no XFF at all → chain = [RemoteAddr]; clamp → RemoteAddr.
			name:              "count=1 no xff falls back to remote",
			setup:             func(r *http.Request) {},
			remote:            "198.51.100.20:2222",
			trustedProxyCount: 1,
			want:              "198.51.100.20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			tt.setup(req)
			if got := clientIP(req, tt.trustedProxyCount); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

// TestRateLimitXFFSpoofSharesAuthBucket is the #1108 F2 red→green regression.
//
// With the safe default (TrustedProxyCount=0), an unauthenticated attacker who
// sends >burst auth-path requests from the SAME RemoteAddr but a UNIQUE
// X-Forwarded-For each MUST still hit a 429 — proving all requests collapse to a
// single `auth:<remoteaddr>` bucket. On the OLD blind-trust code each unique XFF
// got a fresh bucket and a 429 never appeared (the bypass). The auth rate is set
// to 0 so the bucket never refills mid-test → deterministic, no token-refill flake.
func TestRateLimitXFFSpoofSharesAuthBucket(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	cfg := &config.Config{
		NodeEnv: "", // not dev → no E2E bypass
		// auth rate 0 = no refill mid-test; burst 15 = 15 allowed then 429.
		RateLimitAuthPerSec: 0,
		RateLimitAuthBurst:  15,
		RateLimitUserPerSec: 20,
		RateLimitUserBurst:  60,
		RateLimitAnonPerSec: 100,
		RateLimitAnonBurst:  300,
		TrustedProxyCount:   0, // safe default — XFF must not influence the key
	}
	rl := newRateLimiter(t.Context(), cfg)
	handler := rateLimitMiddleware(rl, srv.store, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	const burst = 15
	saw429 := false
	for i := 0; i < burst+5; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.77:54321" // SAME source every request
		// UNIQUE spoofed XFF per request — would defeat the OLD code.
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.18.0.%d", i))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			saw429 = true
			break
		}
	}
	if !saw429 {
		t.Fatalf("expected a 429 from the shared auth:<remoteaddr> bucket; "+
			"per-XFF buckets (the #1108 F2 bypass) would never throttle (burst=%d)", burst)
	}
}

func TestRateLimiterRefills(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(t.Context(), testRateLimitCfg())
	rl.anonRate = 10
	rl.anonMax = 2
	ip := "198.51.100.11"
	key := "anon:" + ip

	if !rl.allow(key, rl.anonRate, rl.anonMax) || !rl.allow(key, rl.anonRate, rl.anonMax) {
		t.Fatal("expected initial tokens to allow requests")
	}
	if rl.allow(key, rl.anonRate, rl.anonMax) {
		t.Fatal("expected exhausted bucket to reject request")
	}

	rl.mu.Lock()
	rl.clients[key].lastTime = time.Now().Add(-time.Second)
	rl.mu.Unlock()
	if !rl.allow(key, rl.anonRate, rl.anonMax) {
		t.Fatal("expected elapsed time to refill bucket")
	}
}

func TestHandleStaticNotFoundBranches(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)

	for _, path := range []string{"/ws/missing", "/missing.js", "/nested-route"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.handleStatic(rec, httptest.NewRequest("GET", path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", rec.Code)
			}
		})
	}
}

func TestProtectedMessageRouteResolvesChannelScope(t *testing.T) {
	t.Parallel()
	srv, s := testServer(t)
	srv.cfg.DevAuthBypass = true

	user := &store.User{DisplayName: "Scoped Sender", Role: "member"}
	if err := s.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.GrantDefaultPermissions(user.ID, "member"); err != nil {
		t.Fatalf("grant permissions: %v", err)
	}

	ch := &store.Channel{Name: "scoped", Visibility: "public", CreatedBy: user.ID, Type: "channel", Position: store.GenerateInitialRank()}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: user.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/"+ch.ID+"/messages", strings.NewReader(`{"content":"scoped hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dev-User-Id", user.ID)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected message creation through protected route, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHub(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	if srv.Hub() == nil {
		t.Fatal("expected hub")
	}
}

func TestAdapters(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)
	hub := srv.Hub()
	hub.CommandStore().Register("conn-1", "agent-1", "Agent One", []ws.AgentCommand{
		{Name: "deploy", Description: "Deploy service", Usage: "/deploy <service>"},
	})

	// hubCommandAdapter
	ca := &hubCommandAdapter{hub: hub}
	cmds := ca.GetAllCommands()
	if len(cmds) != 1 || cmds[0].AgentID != "agent-1" || len(cmds[0].Commands) != 1 {
		t.Fatalf("unexpected commands: %#v", cmds)
	}
	if cmds[0].Commands[0].Name != "deploy" || cmds[0].Commands[0].Usage == "" {
		t.Fatalf("unexpected command mapping: %#v", cmds[0].Commands[0])
	}

	// hubRemoteAdapter
	ra := &hubRemoteAdapter{hub: hub}
	if ra.IsNodeOnline("nonexistent") {
		t.Fatal("expected false")
	}
	_, err := ra.ProxyRequest("nonexistent", "ls", "/")
	if err == nil {
		t.Fatal("expected error for offline node")
	}

	// hubBroadcastAdapter
	ba := &hubBroadcastAdapter{hub: hub}
	ba.BroadcastEventToChannel("ch-1", "test", map[string]string{})
	ba.BroadcastEventToAll("test", map[string]string{})
	ba.BroadcastEventToUser("user-1", "test", map[string]string{})
	ba.SignalNewEvents()

	// hubPluginAdapter
	pa := &hubPluginAdapter{hub: hub}
	_, _, err = pa.ProxyPluginRequest("nonexistent", "read_file", "/test", nil)
	if err == nil {
		t.Fatal("expected error for disconnected plugin")
	}
}

func TestHubPluginAdapterProxySuccess(t *testing.T) {
	t.Parallel()
	srv, s := testServer(t)
	apiKey := "bgr_plugin_proxy_success"
	agent := &store.User{DisplayName: "Proxy Bot", Role: "agent", APIKey: &apiKey}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/plugin?apiKey=" + apiKey
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial plugin ws: %v", err)
	}
	defer conn.Close()

	// Wait for HandlePlugin to register the PluginConn on the hub. The WS
	// handshake completes before RegisterPlugin runs server-side, so without
	// this poll ProxyPluginRequest may observe nil and return "agent not
	// connected" instantly — the test would then block forever on ReadJSON
	// and trip the 10-minute package timeout (CI flake).
	deadline := time.Now().Add(2 * time.Second)
	for srv.Hub().GetPlugin(agent.ID) == nil {
		if time.Now().After(deadline) {
			t.Fatal("plugin registration timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}

	type proxyResult struct {
		status int
		body   []byte
		err    error
	}
	done := make(chan proxyResult, 1)
	adapter := &hubPluginAdapter{hub: srv.Hub()}
	go func() {
		status, body, err := adapter.ProxyPluginRequest(agent.ID, http.MethodGet, "/files", nil)
		done <- proxyResult{status: status, body: body, err: err}
	}()

	// Bound the read so a missing/lost upstream request fails fast instead of
	// hanging until the package-level timeout.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var req map[string]any
	if err := conn.ReadJSON(&req); err != nil {
		t.Fatalf("read proxy request: %v", err)
	}
	if req["type"] != "request" || req["id"] == "" {
		t.Fatalf("unexpected proxy request: %v", req)
	}
	if err := conn.WriteJSON(map[string]any{
		"type": "response",
		"id":   req["id"],
		"data": map[string]any{"ok": true},
	}); err != nil {
		t.Fatalf("write proxy response: %v", err)
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("proxy request failed: %v", result.err)
		}
		if result.status != http.StatusOK || !strings.Contains(string(result.body), "ok") {
			t.Fatalf("unexpected proxy result status=%d body=%s", result.status, result.body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy result")
	}
}

func TestWriteErrorResponse(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	writeErrorResponse(rec, http.StatusInternalServerError, "test error")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRespondNotImplemented(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	respondNotImplemented(rec, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestRateLimiterCleanup_CtxCancelExits — TEST-FIX-2 covers the cleanup
// goroutine's ctx.Done() branch + ticker tick + delete loop. Pre-fix the
// goroutine was unbounded; post-fix it exits when ctx cancelled (caller's
// t.Context() in tests, srv ctx in production).
func TestRateLimiterCleanup_CtxCancelExits(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	rl := newRateLimiter(ctx, testRateLimitCfg())
	// Seed an old client so the cleanup loop's delete branch can fire.
	rl.mu.Lock()
	rl.clients["1.2.3.4"] = &clientBucket{lastTime: time.Now().Add(-20 * time.Minute)}
	rl.mu.Unlock()
	// Force the cleanup loop to exit promptly (don't wait for 5min ticker).
	cancel()
	// Brief wait for goroutine to observe Done.
	time.Sleep(50 * time.Millisecond)
}

// TestRateLimiterCleanup_TickFiresDelete — TEST-FIX-2 covers the
// evictStale eviction logic (extracted from cleanup() ticker.C body so it's
// unit-testable without waiting 5min). Drops 10min+ stale entries, keeps fresh.
func TestRateLimiterCleanup_TickFiresDelete(t *testing.T) {
	t.Parallel()
	rl := &rateLimiter{
		clients:  make(map[string]*clientBucket),
		authRate: 1,
		authMax:  1,
		userRate: 1,
		userMax:  1,
		anonRate: 1,
		anonMax:  1,
	}
	// Seed old + fresh entries; cleanup should drop only old.
	rl.clients["old"] = &clientBucket{lastTime: time.Now().Add(-20 * time.Minute)}
	rl.clients["fresh"] = &clientBucket{lastTime: time.Now()}
	rl.evictStale(time.Now())
	if _, ok := rl.clients["old"]; ok {
		t.Fatal("expected old entry deleted")
	}
	if _, ok := rl.clients["fresh"]; !ok {
		t.Fatal("expected fresh entry kept")
	}
}

func TestBuildRequestData_FlatShape(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(buildRequestData("ls", "/x"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"path"`) {
		t.Errorf("frame data must contain \"path\"; got %s", s)
	}
	if !strings.Contains(s, `"/x"`) {
		t.Errorf("frame data must carry the path value; got %s", s)
	}
	if strings.Contains(s, `"params"`) {
		t.Errorf("frame data must NOT contain \"params\" (flat shape required); got %s", s)
	}
	if strings.Contains(s, `"action":"ls"`) == false {
		t.Errorf("frame data must contain action verb; got %s", s)
	}
}
