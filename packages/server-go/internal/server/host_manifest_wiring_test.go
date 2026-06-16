// Package server — host_manifest_wiring_test.go: #1108 F3 server-construction
// wiring guard for the plugin-manifest endpoint.
//
// The api-package test (host_manifest_prod_failclosed_test.go) pins the HANDLER
// behavior given a hand-set AllowUnsignedPlaceholder bool. It does NOT exercise
// the security-critical WIRING in server.go (~line 345):
//
//	&api.PluginManifestHandler{... AllowUnsignedPlaceholder: s.cfg.IsDevelopment()}
//
// If someone reverts that expression to `AllowUnsignedPlaceholder: true`, the
// hole silently re-opens (a production deploy with no signing key serves a fake
// placeholder signature + HTTP 200) while every api-package test stays green.
//
// This test closes that gap: it builds the server the way production does —
// server.New(...) with a NON-development config (NodeEnv != "development") AND
// no BORGEE_MANIFEST_SIGNING_KEY — then drives GET /api/v1/plugin-manifest
// through the real mux + auth middleware with a valid owner Bearer api-key and
// asserts the endpoint fails closed (HTTP 500, body NOT the placeholder). The
// test pins the wiring EXPRESSION, so changing server.go to a literal `true`
// makes it fail.
package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

// manifestPlaceholderLiteral mirrors host_manifest.go::signPayload's dev-seam
// placeholder. Kept byte-identical so the assertion fails loudly if the literal
// ever drifts.
const manifestPlaceholderLiteral = "test-signature-placeholder-32by"

// manifestPlaceholderB64 is the placeholder as it appears on the wire (the
// signature field is base64-encoded). The fail-closed prod path must leak
// NEITHER the raw literal NOR its base64 form.
var manifestPlaceholderB64 = base64.StdEncoding.EncodeToString([]byte(manifestPlaceholderLiteral))

// prodCfgNoSigningKey returns a config shaped like a production deploy:
// NodeEnv != "development" (so cfg.IsDevelopment() == false), with the
// non-dev-required fields (JWTSecret + CORSOrigin) populated. The manifest
// signing key is supplied via the BORGEE_MANIFEST_SIGNING_KEY env (left empty
// by the test), NOT via this struct — server.New reads it through
// api.LoadSigningKey at boot.
func prodCfgNoSigningKey(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{
		JWTSecret:     "test-secret",
		NodeEnv:       "production", // NOT development → IsDevelopment() == false
		DevAuthBypass: false,
		UploadDir:     t.TempDir(),
		WorkspaceDir:  t.TempDir(),
		ClientDist:    t.TempDir(),
		CORSOrigin:    "https://prod.example.com",

		RateLimitAuthPerSec: 5,
		RateLimitAuthBurst:  15,
		RateLimitUserPerSec: 20,
		RateLimitUserBurst:  60,
		RateLimitAnonPerSec: 100,
		RateLimitAnonBurst:  300,
	}
	if cfg.IsDevelopment() {
		t.Fatalf("test bug: prod cfg must NOT be development (NodeEnv=%q)", cfg.NodeEnv)
	}
	return cfg
}

// seedOwnerWithAPIKey creates a member user with a Bearer api-key so the
// real auth middleware (Authorization: Bearer <api_key>) authenticates it.
func seedOwnerWithAPIKey(t *testing.T, s *store.Store, apiKey string) {
	t.Helper()
	email := "manifest-owner@test.com"
	owner := &store.User{
		DisplayName:  "Manifest Owner",
		Role:         "member",
		Email:        &email,
		PasswordHash: "x",
	}
	if err := s.CreateUser(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := s.SetAPIKey(owner.ID, apiKey); err != nil {
		t.Fatalf("set owner api key: %v", err)
	}
}

// TestPluginManifest_ProdWiring_FailsClosed builds the server through the REAL
// production construction path (server.New with a non-dev cfg + no signing key)
// and asserts the wired handler fails closed.
//
// This is the wiring guard the api-package test cannot provide: it reads
// AllowUnsignedPlaceholder through server.go's `s.cfg.IsDevelopment()`
// expression. Flipping that expression to a literal `true` re-opens the hole
// and makes THIS test go RED (200 + placeholder), even though the api-package
// handler tests stay green.
//
// Not parallel: it pins BORGEE_MANIFEST_SIGNING_KEY="" via t.Setenv to make the
// nil-key branch deterministic regardless of the ambient CI env.
func TestPluginManifest_ProdWiring_FailsClosed(t *testing.T) {
	// Force the signing key unset so api.LoadSigningKey returns nil at boot —
	// the production "no key configured" condition. t.Setenv("", "") makes
	// os.Getenv return "" deterministically and restores the prior value on
	// cleanup (so it does not bleed into other tests).
	t.Setenv(api.EnvManifestSigningKey, "")

	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })

	const apiKey = "prod-wiring-owner-api-key-0001"
	seedOwnerWithAPIKey(t, s, apiKey)

	cfg := prodCfgNoSigningKey(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(t.Context(), cfg, logger, s)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/plugin-manifest", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Sanity: the owner Bearer key authenticated (not a 401 short-circuit) —
	// otherwise the 500 below would be vacuous.
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("owner api-key failed to authenticate (401) — wiring test would be vacuous; body: %s", body)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("prod wiring (NodeEnv=%q, no signing key) must fail closed with 500, got %d (body %s)\n"+
			"if this is 200+placeholder, server.go's AllowUnsignedPlaceholder wiring was changed away from cfg.IsDevelopment()",
			cfg.NodeEnv, resp.StatusCode, body)
	}
	if strings.Contains(string(body), manifestPlaceholderLiteral) || strings.Contains(string(body), manifestPlaceholderB64) {
		t.Fatalf("prod wiring must NOT leak the placeholder signature, body: %s", body)
	}
}

// TestPluginManifest_DevWiring_ServesPlaceholder is the seam-survives twin:
// the SAME real construction path with a development cfg (cfg.IsDevelopment()
// == true) still serves the placeholder + 200. This pins the wiring as a true
// conditional: a literal `false` would break dev, a literal `true` would break
// the prod test above — only the `cfg.IsDevelopment()` expression keeps BOTH
// green.
func TestPluginManifest_DevWiring_ServesPlaceholder(t *testing.T) {
	t.Setenv(api.EnvManifestSigningKey, "")

	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })

	const apiKey = "dev-wiring-owner-api-key-0001"
	seedOwnerWithAPIKey(t, s, apiKey)

	cfg := prodCfgNoSigningKey(t)
	cfg.NodeEnv = "development" // flip to dev → IsDevelopment() == true
	if !cfg.IsDevelopment() {
		t.Fatalf("test bug: dev cfg must be development")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(t.Context(), cfg, logger, s)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/plugin-manifest", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dev wiring seam must serve 200, got %d (body %s)", resp.StatusCode, body)
	}
	var payload api.PluginManifestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal dev manifest: %v (body %s)", err, body)
	}
	sig, err := base64.StdEncoding.DecodeString(payload.Signature)
	if err != nil {
		t.Fatalf("decode dev signature %q: %v", payload.Signature, err)
	}
	if string(sig) != manifestPlaceholderLiteral {
		t.Fatalf("dev wiring seam must serve the byte-identical placeholder (dev needs no crypto setup); got %q", string(sig))
	}
}
