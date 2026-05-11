package api_test

// PERF-JWT-CLOCK unit tests verify AuthHandler.Clock injection matches the
// time.Now() production path and that fake clocks advance JWT iat/exp without
// sleeping.

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
	"borgee-server/internal/testutil/clock"
)

// TestAuthHandler_NilClock_FallsBackToTimeNow pins production path:
// when Clock=nil (default), AuthHandler.now() returns time.Now() — minted
// JWT iat is within ~1s of wall-clock. Regression guard: the production path
// keeps the pre-PERF-JWT-CLOCK direct time.Now() behavior.
func TestAuthHandler_NilClock_FallsBackToTimeNow(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	before := time.Now().Unix()
	tok := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	after := time.Now().Unix()

	// Decode JWT payload (HS256 unsigned read for iat).
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("bad JWT shape: %d parts", len(parts))
	}
	payload, err := jwtBase64Decode(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		IAT int64 `json:"iat"`
		EXP int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims.IAT < before-1 || claims.IAT > after+1 {
		t.Errorf("nil clock should mint iat near wall-clock: got %d, want in [%d, %d]",
			claims.IAT, before, after)
	}
	if claims.EXP-claims.IAT != 7*24*3600 {
		t.Errorf("exp - iat must be 7d (production constant): got %d s",
			claims.EXP-claims.IAT)
	}
}

// TestAuthHandler_FakeClock_AdvancesIAT pins the perf-test path: fake clock
// Advance(N) makes subsequent JWT mint use the advanced timestamp (no real
// sleep). This replaces time.Sleep(1100ms) while still proving token rotation
// advances iat.
func TestAuthHandler_FakeClock_AdvancesIAT(t *testing.T) {
	t.Parallel()
	ts, _, _, fake := testutil.NewTestServerWithFakeClock(t)

	tok1 := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	iat1 := decodeIAT(t, tok1)

	// Advance 5s — second mint must reflect the advance.
	fake.Advance(5 * time.Second)
	tok2 := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	iat2 := decodeIAT(t, tok2)

	if delta := iat2 - iat1; delta < 5 || delta > 6 {
		t.Errorf("fake clock Advance(5s) should bump iat by ~5s: got delta=%d", delta)
	}
	if tok1 == tok2 {
		t.Error("fake clock advance must mint a different token (different iat)")
	}
}

// TestAuthHandler_FakeClock_NoRealSleep — wall-clock invariant: fake clock
// Advance is sub-millisecond regardless of duration argument.
func TestAuthHandler_FakeClock_NoRealSleep(t *testing.T) {
	t.Parallel()
	_, _, _, fake := testutil.NewTestServerWithFakeClock(t)
	start := time.Now()
	fake.Advance(1 * time.Hour) // Should return immediately.
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("fake.Advance(1h) should be <100ms wall-clock: got %v", elapsed)
	}
}

// TestAuthHandler_StructFieldExposed documents that AuthHandler.Clock is a
// public injection point tests can construct directly with a fake clock.
func TestAuthHandler_StructFieldExposed(t *testing.T) {
	t.Parallel()
	fake := clock.NewFake(time.Now())
	h := &api.AuthHandler{Clock: fake}
	// Compile-time guard: the field type accepts the clock.Clock interface.
	if h.Clock == nil {
		t.Fatal("Clock field accepts *Fake")
	}
}

// TestAuthHandler_ProductionPath_NoBehaviorChange guards production behavior.
// signAndSetCookie calls h.now(); with nil Clock it uses time.Now() and keeps
// cookie name / MaxAge / HttpOnly / SameSite unchanged.
func TestAuthHandler_ProductionPath_NoBehaviorChange(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	body := strings.NewReader(`{"email":"admin@test.com","password":"password123"}`)
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", body)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", resp.StatusCode)
	}
	var found *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "borgee_token" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("borgee_token cookie missing")
	}
	if !found.HttpOnly {
		t.Error("HttpOnly must be true (production invariant)")
	}
	if found.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite=Lax invariant: got %v", found.SameSite)
	}
	if found.MaxAge != 604800 {
		t.Errorf("MaxAge=7d invariant: got %d", found.MaxAge)
	}
}

// jwtBase64Decode strips JWT-flavored base64url (no padding).
func jwtBase64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func decodeIAT(t *testing.T, tok string) int64 {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("bad JWT shape: %d parts", len(parts))
	}
	payload, err := jwtBase64Decode(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var c struct {
		IAT int64 `json:"iat"`
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return c.IAT
}

// Avoid unused import lint by referencing stdlib types.
var (
	_ = httptest.NewServer
	_ store.User
)
