package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"borgee-server/internal/admin"
)

// newLoginServer returns a Handler-backed test server with a single
// bootstrapped admin (login "root" / password "correct-horse"). Used by the
// 1.C / 1.D / 1.E tests below.
func newLoginServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	db := openMigratedDB(t)
	plain := "correct-horse"
	if err := admin.BootstrapWith(db, "root", hashAt(t, plain, 10), ""); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	mux := http.NewServeMux()
	(&admin.Handler{DB: db, IsDevelopment: true}).RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, plain
}

func postLogin(t *testing.T, base, login, password string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"login":    login,
		"password": password,
	})
	resp, err := http.Post(base+"/admin-api/auth/login",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	return resp
}

// TestLogin_1C_ValidEnvLogin covers review checklist invariant 1.C:
// "POST /admin-api/auth/login 用 env login → 返 200 + Set-Cookie borgee_admin_session".
func TestLogin_1C_ValidEnvLogin(t *testing.T) {
	t.Parallel()
	srv, plain := newLoginServer(t)

	resp := postLogin(t, srv.URL, "root", plain)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Cookie name is the locked literal (review checklist 红线).
	var got *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == admin.CookieName {
			got = c
		}
	}
	if got == nil {
		t.Fatalf("missing %q cookie; got cookies=%v", admin.CookieName, resp.Cookies())
	}
	if got.Name != "borgee_admin_session" {
		t.Fatalf("cookie name = %q, want literal \"borgee_admin_session\"", got.Name)
	}
	if !got.HttpOnly {
		t.Fatal("admin session cookie must be HttpOnly")
	}
	if got.Value == "" {
		t.Fatal("admin session cookie value empty")
	}
}

// TestLogin_1D_NonAdminRejected covers review checklist invariant 1.D:
// "POST /admin-api/auth/login 用普通 user login → 返 401 (auth path 隔离)".
//
// We do not (and must not) seed a row in `users`; the admin path looks at
// `admins` only. Any login not present there must 401.
func TestLogin_1D_NonAdminRejected(t *testing.T) {
	t.Parallel()
	srv, _ := newLoginServer(t)

	// "alice" is not in admins (only "root" was bootstrapped). Even if she
	// existed in users with role='admin' (dual-rail 1.F), the new admin path
	// must NOT see her.
	resp := postLogin(t, srv.URL, "alice", "any-password")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("non-admin login: status = %d, want 401", resp.StatusCode)
	}

	// Also: correct admin login but wrong password must still 401.
	resp2 := postLogin(t, srv.URL, "root", "wrong-password")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: status = %d, want 401", resp2.StatusCode)
	}
}

func TestVerifyPassword_E2EFastAdminPassword(t *testing.T) {
	t.Setenv(admin.EnvTestFastAdminPassword, "e2e-admin-pass")

	if !admin.VerifyPassword("not-a-bcrypt-hash", "e2e-admin-pass") {
		t.Fatal("test fast admin password should accept the configured plaintext")
	}
	if admin.VerifyPassword("not-a-bcrypt-hash", "wrong") {
		t.Fatal("test fast admin password should still reject the wrong plaintext")
	}
}
