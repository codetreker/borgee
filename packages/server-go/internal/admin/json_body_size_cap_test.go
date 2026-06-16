package admin_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// #1108 F7+SK2 — the unauthenticated admin-login endpoint caps the JSON body
// at 1 MiB via http.MaxBytesReader and returns 413 on overflow. Pre-fix the
// inline json.NewDecoder(r.Body) buffered an attacker-controlled multi-GB body
// without bound → OOM DoS on an endpoint reachable without credentials.
// red→green: pre-fix this returned 400/401 (body decoded); post-fix → 413.
func TestAdminLoginOversizedBodyReturns413(t *testing.T) {
	t.Parallel()
	srv, _ := newLoginServer(t)

	var b bytes.Buffer
	b.WriteString(`{"login":"`)
	b.WriteString(strings.Repeat("a", (1<<20)+1024))
	b.WriteString(`","password":"x"}`)
	if b.Len() <= 1<<20 {
		t.Fatalf("test payload must exceed 1 MiB, got %d bytes", b.Len())
	}

	resp, err := http.Post(srv.URL+"/admin-api/auth/login",
		"application/json", bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized admin-login body: status = %d, want 413", resp.StatusCode)
	}
}

// Positive case: a normal (≤1 MiB) admin-login body still works (no behavior
// change). Valid env credentials → 200, proving the cap is transparent for
// in-bound requests.
func TestAdminLoginNormalBodyStillDecodes(t *testing.T) {
	t.Parallel()
	srv, plain := newLoginServer(t)

	resp := postLogin(t, srv.URL, "root", plain)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("normal admin-login body: status = %d, want 200 (body must decode normally)", resp.StatusCode)
	}
}
