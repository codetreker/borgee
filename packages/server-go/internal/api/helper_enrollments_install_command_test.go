package api

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"
)

// #1052 — `buildHelperInstallCommand` origin selection priority.
//
// These are pure-function tests (no httptest) covering the new
// BORGEE_PUBLIC_HELPER_ORIGIN override path AND locking the prior
// r.Host / X-Forwarded-* derivation as the fallback. The end-to-end
// behavior through the HTTP handler is still covered by the existing
// TestHelperEnrollmentCreate_* tests in helper_enrollments_test.go;
// this file is the unit-level matrix.

const testToken = "enr-abc.secret-xyz"

func mkReq(t *testing.T, host string, tls bool, hdrs map[string]string) *http.Request {
	t.Helper()
	scheme := "http"
	if tls {
		scheme = "https"
	}
	req, err := http.NewRequest(http.MethodPost, scheme+"://"+host+"/api/v1/helper/enrollments", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Host = host
	if tls {
		req.TLS = &tlsConnState
	}
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
	return req
}

// non-nil sentinel so r.TLS != nil triggers wss in the fallback path.
var tlsConnState = (func() tls.ConnectionState { return tls.ConnectionState{} })()

func TestBuildHelperInstallCommand_PublicOriginOverridesRHost(t *testing.T) {
	t.Parallel()
	req := mkReq(t, "127.0.0.1:4900", false, nil)
	got := buildHelperInstallCommand(req, testToken, "ws://borgee-server:4900")
	want := "npx @codetreker/borgee-remote-agent install --server ws://borgee-server:4900 --token " + testToken
	if got != want {
		t.Fatalf("override branch mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBuildHelperInstallCommand_PublicOriginOverridesProxyHeaders(t *testing.T) {
	t.Parallel()
	// Even if a reverse proxy IS setting X-Forwarded-*, the explicit env
	// override wins (operator told us THIS is the public address; trust it).
	req := mkReq(t, "internal-server:4900", false, map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "borgee.example.com",
	})
	got := buildHelperInstallCommand(req, testToken, "wss://borgee.codetrek.cn")
	if !strings.Contains(got, "--server wss://borgee.codetrek.cn") {
		t.Fatalf("public origin must win over X-Forwarded-*: %q", got)
	}
	if strings.Contains(got, "borgee.example.com") || strings.Contains(got, "internal-server") {
		t.Fatalf("public origin override must replace request-derived host completely: %q", got)
	}
}

func TestBuildHelperInstallCommand_PublicOriginTrimsWhitespace(t *testing.T) {
	t.Parallel()
	// Defensive: env file readers sometimes leave trailing spaces. The
	// override is treated as empty (fallback path) only when it is empty
	// AFTER trim; non-empty trimmed value is used verbatim.
	req := mkReq(t, "127.0.0.1:4900", false, nil)
	got := buildHelperInstallCommand(req, testToken, "   ws://borgee-server:4900   ")
	if !strings.Contains(got, "--server ws://borgee-server:4900 --token") {
		t.Fatalf("public origin whitespace not trimmed: %q", got)
	}
}

func TestBuildHelperInstallCommand_EmptyPublicOriginFallsBackToRHost(t *testing.T) {
	t.Parallel()
	// Empty override (the common single-host on-prem case) must preserve
	// the pre-#1052 derivation: r.TLS=nil + no X-Forwarded-* → ws://r.Host.
	req := mkReq(t, "borgee.local:4900", false, nil)
	got := buildHelperInstallCommand(req, testToken, "")
	want := "npx @codetreker/borgee-remote-agent install --server ws://borgee.local:4900 --token " + testToken
	if got != want {
		t.Fatalf("fallback branch mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBuildHelperInstallCommand_WhitespaceOnlyPublicOriginFallsBack(t *testing.T) {
	t.Parallel()
	// `BORGEE_PUBLIC_HELPER_ORIGIN="   "` in an env file must NOT be a
	// silent break — it falls back as if unset.
	req := mkReq(t, "borgee.local:4900", false, nil)
	got := buildHelperInstallCommand(req, testToken, "   ")
	if !strings.Contains(got, "--server ws://borgee.local:4900") {
		t.Fatalf("whitespace-only override must behave as unset: %q", got)
	}
}

func TestBuildHelperInstallCommand_FallbackHonorsXForwardedProto(t *testing.T) {
	t.Parallel()
	// Fallback path still respects nginx-style X-Forwarded-* — locks the
	// existing TestHelperEnrollmentCreate_InstallCommandHonorsForwardedProto
	// invariant at the unit level.
	req := mkReq(t, "127.0.0.1:4900", false, map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "borgee.codetrek.cn",
	})
	got := buildHelperInstallCommand(req, testToken, "")
	if !strings.Contains(got, "--server wss://borgee.codetrek.cn") {
		t.Fatalf("fallback X-Forwarded-* not honored: %q", got)
	}
}

func TestBuildHelperInstallCommand_FallbackDirectTLSYieldsWss(t *testing.T) {
	t.Parallel()
	// r.TLS != nil (direct TLS, no proxy headers) → wss://r.Host.
	req := mkReq(t, "borgee.codetrek.cn", true, nil)
	got := buildHelperInstallCommand(req, testToken, "")
	if !strings.Contains(got, "--server wss://borgee.codetrek.cn") {
		t.Fatalf("direct-TLS fallback should yield wss://: %q", got)
	}
}
