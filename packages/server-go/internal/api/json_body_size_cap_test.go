package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// #1108 F7+SK2 — every JSON request-body decode site caps the body at 1 MiB
// via http.MaxBytesReader and returns 413 on overflow. Before the fix, an
// unauthenticated multi-GB POST to register/login was decoded without bound
// → memory-exhaustion DoS. These are red→green: pre-fix they returned 400/200
// (the body was fully decoded); post-fix they return 413.

// oversizedJSONBody returns a syntactically-valid JSON object whose single
// string field is comfortably larger than the 1 MiB cap (1<<20 + 1 KiB of
// payload bytes), so the MaxBytesReader trips before Decode completes.
func oversizedJSONBody(field string) []byte {
	var b bytes.Buffer
	b.WriteString(`{"`)
	b.WriteString(field)
	b.WriteString(`":"`)
	b.WriteString(strings.Repeat("a", (1<<20)+1024))
	b.WriteString(`"}`)
	return b.Bytes()
}

func TestRegisterOversizedBodyReturns413(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupTest(t)

	body := oversizedJSONBody("email")
	if len(body) <= 1<<20 {
		t.Fatalf("test payload must exceed 1 MiB, got %d bytes", len(body))
	}
	resp, err := http.Post(ts.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized register body: status = %d, want 413", resp.StatusCode)
	}
}

func TestLoginOversizedBodyReturns413(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupTest(t)

	body := oversizedJSONBody("email")
	if len(body) <= 1<<20 {
		t.Fatalf("test payload must exceed 1 MiB, got %d bytes", len(body))
	}
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized login body: status = %d, want 413", resp.StatusCode)
	}
}

// Positive case: a normal (≤1 MiB) login body still works (no behavior change).
// Wrong password → 401, which proves the body decoded fine and the cap is
// transparent for in-bound requests (a too-large body would have short-circuited
// to 413 before reaching the credential check).
func TestLoginNormalBodyStillDecodes(t *testing.T) {
	t.Parallel()
	ts, s, _ := setupTest(t)
	createTestUser(t, s, "capuser@test.com", "password123", "member")

	body, _ := json.Marshal(map[string]string{"email": "capuser@test.com", "password": "wrong"})
	if len(body) >= 1<<20 {
		t.Fatalf("normal payload must be under 1 MiB, got %d bytes", len(body))
	}
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("normal login body: status = %d, want 401 (body must decode normally)", resp.StatusCode)
	}
}
