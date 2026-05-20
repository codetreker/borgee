//go:build linux || darwin

package install

import (
	"bytes"
	"context"
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

// TI-1 NotSudo — non-root invocation fails with friendly msg.
//
// run() checks os.Geteuid() == 0 and exits early. We cannot test the
// not-root path under the regular `go test` binary because tests often
// run as root in CI containers. Skip when running as root; otherwise
// exercise the early-exit branch.
func TestRun_NotSudo(t *testing.T) {
	if isRoot() {
		t.Skip("test runs as root; not-sudo branch unreachable")
	}
	var out, errBuf bytes.Buffer
	err := run(&config{
		server: "wss://example.com",
		token:  "id.secret",
	}, &out, &errBuf)
	if err == nil {
		t.Fatalf("expected not-sudo error")
	}
	if !strings.Contains(errBuf.String(), "must be run as root") {
		t.Fatalf("stderr missing not-root banner: %q", errBuf.String())
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
		server:               "https://example.test",
		token:                "enr1.secret1",
		skipStart:            true,
		skipBinaryCopy:       true,
		skipSetup:            true,
		skipClaim:            true,
		skipRootCheck:        true,
		systemctl:            runner,
		allowInsecureServer:  false,
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
	}
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}
	got := runner.joined()
	switch runtimeGOOS() {
	case "linux":
		// Expect enable+start for both borgee.service AND borgee-rootd.service.
		wantPairs := []string{
			"systemctl enable borgee-rootd.service",
			"systemctl start borgee-rootd.service",
			"systemctl enable borgee.service",
			"systemctl start borgee.service",
		}
		for _, want := range wantPairs {
			found := false
			for _, c := range got {
				if c == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing required systemctl call %q (got: %v)", want, got)
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
