// Package api_test — host_manifest_prod_failclosed_test.go: #1108 F3
// fail-closed guard for the plugin-manifest endpoint when no signing key is
// configured.
//
// Finding F3 (option A): with no BORGEE_MANIFEST_SIGNING_KEY the server used
// to emit a fixed placeholder signature ("test-signature-placeholder-32by")
// AND empty per-entry signatures, returning HTTP 200. That makes a production
// deploy serve fake signatures that look real to a naive consumer. The fix
// fails closed in production (per-request HTTP 500) while keeping the dev/test
// seam: PluginManifestHandler.AllowUnsignedPlaceholder gates the placeholder.
//
//	SigningKey==nil + AllowUnsignedPlaceholder==false  → 500 (prod shape)
//	SigningKey==nil + AllowUnsignedPlaceholder==true   → 200 + placeholder (dev/test seam)
package api_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// theManifestPlaceholder mirrors the dev-seam placeholder literal in
// host_manifest.go::signPayload. Kept byte-identical so the assertions below
// fail loudly if the literal ever drifts.
const theManifestPlaceholder = "test-signature-placeholder-32by"

// serveManifest drives GET /api/v1/plugin-manifest through the real mux with a
// test stand-in middleware that injects the ctx user (matching the production
// "ctx already has a user -> short-circuit" semantics). It does not need a DB
// row — mustUser only requires a non-nil *store.User in context.
func serveManifest(t *testing.T, h *api.PluginManifestHandler, u *store.User) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	injectMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u != nil {
				r = r.WithContext(auth.ContextWithUser(r.Context(), u))
			}
			next.ServeHTTP(w, r)
		})
	}
	h.RegisterRoutes(mux, injectMw)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugin-manifest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func ownerForManifest() *store.User {
	email := "owner-manifest@test.com"
	return &store.User{DisplayName: "Owner", Role: "member", Email: &email, PasswordHash: "x"}
}

// TestHB_SignPayload_ProdNilKey_Refuses — production shape (SigningKey nil +
// AllowUnsignedPlaceholder false) must fail closed: HTTP 500 and the body must
// NOT contain the fake placeholder signature.
//
// RED before the fix: today the handler returns the placeholder + HTTP 200.
func TestHB_SignPayload_ProdNilKey_Refuses(t *testing.T) {
	t.Parallel()
	h := &api.PluginManifestHandler{
		Logger:                   slog.New(slog.NewTextHandler(io.Discard, nil)),
		SigningKey:               nil,
		AllowUnsignedPlaceholder: false, // prod default (fail-closed)
	}
	rec := serveManifest(t, h, ownerForManifest())

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("prod nil key must fail closed with 500, got %d (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), theManifestPlaceholder) {
		t.Fatalf("prod response must NOT leak the placeholder signature, body: %s", rec.Body.String())
	}
}

// TestHB_SignPayload_DevNilKey_Placeholder — dev/test seam preserved
// (SigningKey nil + AllowUnsignedPlaceholder true): still 200 and the
// top-level signature decodes to the byte-identical 31-byte placeholder.
func TestHB_SignPayload_DevNilKey_Placeholder(t *testing.T) {
	t.Parallel()
	h := &api.PluginManifestHandler{
		Logger:                   slog.New(slog.NewTextHandler(io.Discard, nil)),
		SigningKey:               nil,
		AllowUnsignedPlaceholder: true, // dev/test seam
	}
	rec := serveManifest(t, h, ownerForManifest())

	if rec.Code != http.StatusOK {
		t.Fatalf("dev seam must return 200, got %d (body %s)", rec.Code, rec.Body.String())
	}
	var payload api.PluginManifestPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sig := decodeB64(t, payload.Signature)
	if string(sig) != theManifestPlaceholder {
		t.Fatalf("dev seam must emit the byte-identical placeholder, got %q", string(sig))
	}
	if len(sig) != 31 {
		t.Fatalf("placeholder must be 31 bytes (seam survives), got %d", len(sig))
	}
}

// TestHB_SignPayload_ProdRealKey_Signs — production with a real key still
// works end-to-end (the fail-closed guard is gated only on a nil key).
func TestHB_SignPayload_ProdRealKey_Signs(t *testing.T) {
	t.Parallel()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	h := &api.PluginManifestHandler{
		Logger:                   slog.New(slog.NewTextHandler(io.Discard, nil)),
		SigningKey:               priv,
		AllowUnsignedPlaceholder: false, // prod; real key present so no refusal
	}
	rec := serveManifest(t, h, ownerForManifest())

	if rec.Code != http.StatusOK {
		t.Fatalf("prod with real key must return 200, got %d (body %s)", rec.Code, rec.Body.String())
	}
	var payload api.PluginManifestPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sig := decodeB64(t, payload.Signature)
	if string(sig) == theManifestPlaceholder {
		t.Fatalf("prod with real key must emit a real signature, not the placeholder")
	}
	if len(sig) != ed25519.SignatureSize {
		t.Fatalf("real ed25519 signature length: got %d, want %d", len(sig), ed25519.SignatureSize)
	}
}

func decodeB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("base64 decode %q: %v", s, err)
	}
	return b
}
