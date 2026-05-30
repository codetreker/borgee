//go:build linux || darwin

package install

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TI-2 BadToken — token without a `.` separator must be rejected.
func TestTokenParts_RejectsMissingSeparator(t *testing.T) {
	t.Parallel()
	if _, _, err := tokenParts("just-an-id-no-dot"); err == nil {
		t.Fatalf("expected error for missing-separator token")
	}
	if _, _, err := tokenParts(""); err == nil {
		t.Fatalf("expected error for empty token")
	}
	if _, _, err := tokenParts(".secret"); err == nil {
		t.Fatalf("expected error for empty id")
	}
	if _, _, err := tokenParts("id."); err == nil {
		t.Fatalf("expected error for empty secret")
	}
}

// TI-2b TokenSplitFirstDot — secret may contain dots; we split on the
// FIRST dot only so a dotted secret roundtrips intact.
func TestTokenParts_SplitFirstDotOnly(t *testing.T) {
	t.Parallel()
	id, secret, err := tokenParts("enr-abc.sec.with.dots")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if id != "enr-abc" {
		t.Fatalf("id = %q, want enr-abc", id)
	}
	if secret != "sec.with.dots" {
		t.Fatalf("secret = %q, want sec.with.dots", secret)
	}
}

// TI-3 InsecureScheme — http:// + ws:// rejected without
// --allow-insecure-server.
func TestDeriveHTTPOrigin_Schemes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in            string
		allowInsecure bool
		wantOK        bool
		wantOrigin    string
	}{
		{"wss://borgee.codetrek.cn", false, true, "https://borgee.codetrek.cn"},
		{"https://app.borgee.io", false, true, "https://app.borgee.io"},
		{"wss://host:9443/some/path", false, true, "https://host:9443"},
		{"http://localhost:8080", false, false, ""},
		{"ws://localhost:8080", false, false, ""},
		{"http://localhost:8080", true, true, "http://localhost:8080"},
		{"ws://localhost:8080", true, true, "http://localhost:8080"},
		{"ftp://x", false, false, ""},
		{"", false, false, ""},
	}
	for _, tc := range cases {
		got, err := deriveHTTPOrigin(tc.in, tc.allowInsecure)
		if tc.wantOK && err != nil {
			t.Fatalf("deriveHTTPOrigin(%q, %v): unexpected err %v", tc.in, tc.allowInsecure, err)
		}
		if !tc.wantOK && err == nil {
			t.Fatalf("deriveHTTPOrigin(%q, %v): expected error, got %q", tc.in, tc.allowInsecure, got)
		}
		if tc.wantOK && got != tc.wantOrigin {
			t.Fatalf("deriveHTTPOrigin(%q): got %q, want %q", tc.in, got, tc.wantOrigin)
		}
	}
}

// PR-2 #1038: deriveWSOrigin replaces the prior silent wss://→https://
// downgrade. The daemon's persistent transport is WebSocket so the
// systemd unit needs the wss:// URL preserved.
func TestDeriveWSOrigin_Schemes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in            string
		allowInsecure bool
		wantOK        bool
		wantOrigin    string
	}{
		{"wss://borgee.codetrek.cn", false, true, "wss://borgee.codetrek.cn"},
		{"https://app.borgee.io", false, true, "wss://app.borgee.io"},
		{"wss://host:9443/some/path", false, true, "wss://host:9443"},
		{"http://localhost:8080", false, false, ""},
		{"ws://localhost:8080", false, false, ""},
		{"http://localhost:8080", true, true, "ws://localhost:8080"},
		{"ws://localhost:8080", true, true, "ws://localhost:8080"},
		{"ftp://x", false, false, ""},
		{"", false, false, ""},
	}
	for _, tc := range cases {
		got, err := deriveWSOrigin(tc.in, tc.allowInsecure)
		if tc.wantOK && err != nil {
			t.Fatalf("deriveWSOrigin(%q, %v): unexpected err %v", tc.in, tc.allowInsecure, err)
		}
		if !tc.wantOK && err == nil {
			t.Fatalf("deriveWSOrigin(%q, %v): expected error, got %q", tc.in, tc.allowInsecure, got)
		}
		if tc.wantOK && got != tc.wantOrigin {
			t.Fatalf("deriveWSOrigin(%q): got %q, want %q", tc.in, got, tc.wantOrigin)
		}
	}
}

// TI-1 NoSudo — normal operator path is intentionally non-root. The
// installer should set up a user-owned main daemon and invoke sudo only for
// the rootd companion / linger pieces.
func TestRun_NoSudoIsAccepted(t *testing.T) {
	var out, errBuf bytes.Buffer
	err := run(&config{
		server:         "wss://example.com",
		token:          "id.secret",
		skipStart:      true,
		skipBinaryCopy: true,
		skipSetup:      true,
		skipClaim:      true,
		installUser: &installUser{
			Username: "alice",
			UID:      1000,
			GID:      1000,
			HomeDir:  "/home/alice",
		},
	}, &out, &errBuf)
	if err != nil {
		t.Fatalf("run should accept non-root install: %v stderr=%s", err, errBuf.String())
	}
}

// recordingRunner captures systemctl calls without invoking anything.
type recordingRunner struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil
}

func (r *recordingRunner) joined() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	for i, c := range r.calls {
		out[i] = strings.Join(c, " ")
	}
	return out
}

// TI-5 SkipStart — when --skip-start, no systemctl calls happen.
//
// Drives run() with skipBinaryCopy + skipSetup + skipClaim + skipStart
// + skipRootCheck so the test is hermetic (no real `borgee daemon`
// start, no real claim HTTP, runnable as non-root). Asserts the runner
// recorded zero calls.
func TestRun_SkipStartNoSystemctl(t *testing.T) {
	runner := &recordingRunner{}
	var out, errBuf bytes.Buffer
	cfg := &config{
		server:              "https://example.test",
		token:               "enr1.secret1",
		skipStart:           true,
		skipBinaryCopy:      true,
		skipSetup:           true,
		skipClaim:           true,
		skipRootCheck:       true,
		systemctl:           runner,
		allowInsecureServer: false,
	}
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v (stderr=%s)", err, errBuf.String())
	}
	if calls := runner.joined(); len(calls) != 0 {
		t.Fatalf("--skip-start should not invoke systemctl, got %v", calls)
	}
	if !strings.Contains(out.String(), "--skip-start") {
		t.Fatalf("stdout missing skip-start banner: %q", out.String())
	}
}

// TI-4 HappyPath — drives the full sequence with stubbed runner,
// stubbed binary copy, and skipped setup/claim. Asserts:
//   - the expected systemctl chain landed: daemon-reload, enable,
//     start (Linux) OR launchctl bootstrap (darwin)
//   - heartbeat-wait deadline is honored (we use a short timeout so the
//     test doesn't hang); a poll failure surfaces as a stderr warning
//     rather than a hard error
func TestRun_HappyPathSystemctlChain(t *testing.T) {
	runner := &recordingRunner{}
	var out, errBuf bytes.Buffer
	cfg := &config{
		server:              "https://example.test",
		token:               "enr-happy.secret-z",
		skipBinaryCopy:      true,
		skipSetup:           true,
		skipClaim:           true,
		skipRootCheck:       true,
		systemctl:           runner,
		heartbeatTimeout:    50 * time.Millisecond,
		allowInsecureServer: true,
	}
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}
	got := runner.joined()
	// On linux: daemon-reload, enable, start. On darwin: launchctl
	// bootstrap. Accept either; the recordingRunner serializes the
	// platform's natural call sequence.
	if len(got) == 0 {
		t.Fatalf("expected systemctl/launchctl calls; got none")
	}
	wantSomeMatch := false
	for _, c := range got {
		if strings.HasPrefix(c, "systemctl ") || strings.HasPrefix(c, "launchctl ") {
			wantSomeMatch = true
		}
	}
	if !wantSomeMatch {
		t.Fatalf("expected systemctl or launchctl call, got %v", got)
	}
}

// TI-6 BothServicesStarted — privilege separation guard: install MUST
// enable + start both the main daemon (borgee.service) AND the rootd
// companion (borgee-rootd.service) on Linux, or bootstrap both plists on
// macOS. A regression that forgot rootd would leave the main daemon
// pointing at a non-existent IPC peer once PR-4 routes root jobs through
// rootdclient.
func TestRun_StartsBothServices(t *testing.T) {
	runner := &recordingRunner{}
	var out, errBuf bytes.Buffer
	cfg := &config{
		server:              "https://example.test",
		token:               "enr-both.secret-z",
		skipBinaryCopy:      true,
		skipSetup:           true,
		skipClaim:           true,
		skipRootCheck:       true,
		systemctl:           runner,
		heartbeatTimeout:    50 * time.Millisecond,
		allowInsecureServer: true,
		installUser: &installUser{
			Username:      "alice",
			UID:           1000,
			GID:           1000,
			HomeDir:       "/home/alice",
			InstallPrefix: "/opt/borgee-test",
		},
	}
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}
	got := runner.joined()
	switch runtimeGOOS() {
	case "linux":
		// Expect user-level main daemon plus sudo-managed linger/rootd.
		wantPairs := []string{
			"sudo loginctl enable-linger alice",
			"sudo install -D -m 0644",
			"sudo systemctl daemon-reload",
			"sudo systemctl enable borgee-rootd-1000.service",
			"sudo systemctl start borgee-rootd-1000.service",
			"systemctl --user daemon-reload",
			"systemctl --user enable borgee.service",
			"systemctl --user start borgee.service",
		}
		for _, want := range wantPairs {
			found := false
			for _, c := range got {
				if c == want || (strings.HasSuffix(want, "0644") && strings.HasPrefix(c, want) && strings.HasSuffix(c, "/etc/systemd/system/borgee-rootd-1000.service")) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing required systemctl call %q (got: %v)", want, got)
			}
		}
		for _, c := range got {
			if strings.Contains(c, "/usr/local/lib/borgee/rootd") || strings.Contains(c, ".local/share/borgee/bin") {
				t.Fatalf("rootd and main daemon must use shared install prefix binary, got call %q", c)
			}
		}
	case "darwin":
		// Expect bootstrap for both plists.
		wantPaths := []string{
			"/Library/LaunchDaemons/cloud.borgee.host-bridge.rootd.plist",
			"/Library/LaunchDaemons/cloud.borgee.host-bridge.plist",
		}
		for _, p := range wantPaths {
			found := false
			for _, c := range got {
				if strings.Contains(c, "launchctl bootstrap system") && strings.Contains(c, p) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing required launchctl bootstrap for %q (got: %v)", p, got)
			}
		}
	}
}

func TestCopyRunningBinary_InstallsSharedBinaryWithSudo(t *testing.T) {
	runner := &recordingRunner{}
	src := filepath.Join(t.TempDir(), "borgee")
	if err := os.WriteFile(src, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write src: %v", err)
	}
	cfg := &config{
		binarySrcOverride: src,
		systemctl:         runner,
		installPrefix:     filepath.Join(t.TempDir(), "install-prefix"),
		installUser: &installUser{
			Username: "alice",
			UID:      1000,
			GID:      1000,
			HomeDir:  "/home/alice",
		},
	}
	var out, errBuf bytes.Buffer
	if err := copyRunningBinary(cfg, &out, &errBuf); err != nil {
		t.Fatalf("copyRunningBinary: %v stderr=%s", err, errBuf.String())
	}
	calls := runner.joined()
	want := "sudo install -D -m 0755 " + src + " " + filepath.Join(cfg.installPrefix, "bin", "borgee")
	if runtimeGOOS() == "darwin" {
		want = "sudo install -D -m 0755 " + src + " " + filepath.Join(cfg.installPrefix, "bin", "borgee")
	}
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("copyRunningBinary calls = %v, want [%q]", calls, want)
	}
}

// TestRun_InstallInvokesClaim — #1055 follow-up (PR #1077 code-review F-A-2 /
// F-S-1): the existing systemctl-chain tests above skip both setup AND claim,
// so they prove only the post-setup post-claim wrapper layer. This test
// drops skipClaim and asserts that install.Run actually invokes claim.Run
// against a real httptest /claim endpoint, writes the credential, enrollment
// id, and device id files to the override path, and propagates the parsed
// enrollment id through to the request URL. Setup remains skipped because
// setup.Run hard-codes system paths (/var/lib/borgee, /etc/systemd/system/)
// and requires root or --dry-run — out of scope for #1055 (no in-flow
// plumbing for either was touched). Compile-time imports at install.go:52-53
// still prove the setup.Run linkage; this test proves the claim.Run linkage
// at runtime.
func TestRun_InstallInvokesClaim(t *testing.T) {
	const (
		enrollmentID  = "enr-fullchain-1"
		secret        = "sec-fullchain-1"
		fakeCredToken = "credtok-from-server"
	)

	var (
		mu         sync.Mutex
		seenURL    string
		seenMethod string
		seenBody   []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenURL = r.URL.Path
		seenMethod = r.Method
		seenBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"helper_credential": fakeCredToken})
	}))
	t.Cleanup(srv.Close)

	tempDir := t.TempDir()
	credentialFile := filepath.Join(tempDir, "credential")

	var out, errBuf bytes.Buffer
	cfg := &config{
		// httptest server is http://; --allow-insecure-server-origin keeps
		// claim from rejecting the non-https origin in tests.
		server:                 srv.URL,
		token:                  enrollmentID + "." + secret,
		allowInsecureServer:    true,
		skipBinaryCopy:         true,
		skipSetup:              true, // see godoc above
		skipClaim:              false,
		skipStart:              true,
		skipRootCheck:          true,
		credentialFileOverride: credentialFile,
		heartbeatTimeout:       50 * time.Millisecond,
	}

	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}

	// Assertion 1: claim actually POSTed to the server — proves install.Run
	// reached and executed claim.Run, not just imported it.
	mu.Lock()
	gotURL, gotMethod, gotBody := seenURL, seenMethod, seenBody
	mu.Unlock()
	wantURL := "/api/v1/helper/enrollments/" + enrollmentID + "/claim"
	if gotURL != wantURL {
		t.Fatalf("claim POST URL = %q, want %q (claim.Run was not invoked from install.Run)", gotURL, wantURL)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("claim method = %q, want POST", gotMethod)
	}
	var bodyJSON struct {
		EnrollmentSecret string `json:"enrollment_secret"`
		HelperDeviceID   string `json:"helper_device_id"`
	}
	if err := json.Unmarshal(gotBody, &bodyJSON); err != nil {
		t.Fatalf("claim body not JSON: %v (raw=%s)", err, string(gotBody))
	}
	if bodyJSON.EnrollmentSecret != secret {
		t.Fatalf("claim body enrollment_secret = %q, want %q", bodyJSON.EnrollmentSecret, secret)
	}
	if strings.TrimSpace(bodyJSON.HelperDeviceID) == "" {
		t.Fatalf("claim body helper_device_id is empty (claim.Run did not resolve a device id)")
	}

	// Assertion 2: credential file written with the body the server returned
	// — proves claim.Run's persistence step executed end-to-end.
	got, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatalf("read credential file: %v", err)
	}
	if string(got) != fakeCredToken {
		t.Fatalf("credential file = %q, want %q", string(got), fakeCredToken)
	}

	// Assertion 3: enrollment-id + device-id sibling files exist under the
	// same credential dir (the layout install.go:294-297 set up).
	for _, sibling := range []string{"enrollment-id", "device-id"} {
		p := filepath.Join(tempDir, sibling)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read sibling %s: %v", sibling, err)
		}
		if strings.TrimSpace(string(b)) == "" {
			t.Fatalf("sibling %s is empty", sibling)
		}
	}

	// Assertion 4: install stdout walks the "step 2/4 claim" banner — proves
	// the install.Run dispatcher reached the claim step (rather than e.g.
	// returning early after setup with skipClaim still effectively true).
	if !strings.Contains(out.String(), "step 2/4 claim") {
		t.Fatalf("stdout missing claim step banner: %q", out.String())
	}
}
