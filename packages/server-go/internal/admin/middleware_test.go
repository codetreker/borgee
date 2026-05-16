package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"borgee-server/internal/admin"

	"gorm.io/gorm"
)

// TestMiddleware_1F_DualRailCoexistence covers review checklist invariant 1.F
// as adapted by ADM-0.2: the admin-rail accepts only its own opaque session
// token via `borgee_admin_session`. Crossing in via a `borgee_token` user
// cookie (or feeding the raw admin id, which ADM-0.1 used to accept) returns
// (nil, nil).
//
// In other words: the new admin auth path is fully isolated. ADM-0.2 §1
// 反向断言 2.A & 2.B are the live form of this invariant.
func TestMiddleware_1F_DualRailCoexistence(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)
	if err := admin.BootstrapWith(db, "root", hashAt(t, "pw", 10), ""); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// 1. AdminFromRequest with NO cookie → (nil, nil).
	r := httptest.NewRequest(http.MethodGet, "/admin-api/v1/orgs", nil)
	got, err := admin.AdminFromRequest(db, r)
	if err != nil {
		t.Fatalf("AdminFromRequest no cookie: err = %v", err)
	}
	if got != nil {
		t.Fatalf("AdminFromRequest no cookie: got = %v, want nil", got)
	}

	// 2. AdminFromRequest with ONLY a legacy `borgee_token` cookie → (nil, nil).
	r2 := httptest.NewRequest(http.MethodGet, "/admin-api/v1/orgs", nil)
	r2.AddCookie(&http.Cookie{Name: "borgee_token", Value: "legacy-jwt"})
	got2, err := admin.AdminFromRequest(db, r2)
	if err != nil {
		t.Fatalf("AdminFromRequest legacy cookie: err = %v", err)
	}
	if got2 != nil {
		t.Fatal("AdminFromRequest must not resolve legacy borgee_token cookie")
	}

	// 3. ADM-0.2 反向断言: feeding the raw admin id as the session cookie
	//    value must NOT resolve. Only an opaque token from CreateSession works.
	id := newAdminID(t, db, "dual-rail-tester")
	r3 := httptest.NewRequest(http.MethodGet, "/admin-api/v1/orgs", nil)
	r3.AddCookie(&http.Cookie{Name: admin.CookieName, Value: id})
	got3, err := admin.AdminFromRequest(db, r3)
	if err != nil {
		t.Fatalf("AdminFromRequest raw id: err = %v", err)
	}
	if got3 != nil {
		t.Fatalf("AdminFromRequest raw admin id MUST NOT resolve, got %+v", got3)
	}

	// 4. With a real CreateSession token → resolves to the admin.
	tok, err := admin.CreateSession(db, id, time.Now())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	r4 := httptest.NewRequest(http.MethodGet, "/admin-api/v1/orgs", nil)
	r4.AddCookie(&http.Cookie{Name: admin.CookieName, Value: tok})
	got4, err := admin.AdminFromRequest(db, r4)
	if err != nil {
		t.Fatalf("AdminFromRequest session token: err = %v", err)
	}
	if got4 == nil || got4.Login != "dual-rail-tester" {
		t.Fatalf("AdminFromRequest session token: got = %+v, want login=dual-rail-tester", got4)
	}
}

// newAdminID inserts an admins row directly and returns the id. Used by the
// dual-rail test to set up a `borgee_admin_session` cookie value.
func newAdminID(t *testing.T, db *gorm.DB, login string) string {
	t.Helper()
	id := login + "-id-static"
	err := db.Exec(
		`INSERT INTO admins (id, login, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		id, login, "$2a$10$0000000000000000000000000000000000000000000000000000",
		time.Now().UnixMilli(),
	).Error
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	return id
}
