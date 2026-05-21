//go:build linux || darwin

package rootdclient

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"borgee/internal/cli/rootd"
)

// startRootdForTest starts a real rootd.Server on a temp UDS without the
// peer-cred check (so tests work as the current non-root user). Returns
// the socket path + a cleanup.
func startRootdForTest(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "rootd.sock")
	srv := &rootd.Server{
		SocketPath: sock,
		PeerGroup:  "", // disable peer-cred check for hermetic test
		Handlers:   rootd.DefaultHandlers(),
		Logger:     func(string, ...any) {},
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(sock); err == nil && info.Mode()&os.ModeSocket != 0 {
			return sock, func() {
				cancel()
				<-serveErr
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatalf("rootd test server did not bind in 2s; err=%v", <-serveErr)
	return "", func() {}
}

// TestClientPingRoundTrip is the integration: real Server + real Client +
// the same JSON wire shape. Should return pong:true with a numeric time.
func TestClientPingRoundTrip(t *testing.T) {
	t.Parallel()
	sock, stop := startRootdForTest(t)
	defer stop()

	c := &Client{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping err: %v", err)
	}
	if got["pong"] != true {
		t.Fatalf("pong = %v, want true (full result: %+v)", got["pong"], got)
	}
	if _, ok := got["time"]; !ok {
		t.Fatalf("ping result missing time field: %+v", got)
	}
}

// TestClientNoSocket — when the socket does not exist, Ping returns a
// dial error (not a panic, not a silent nil).
func TestClientNoSocket(t *testing.T) {
	t.Parallel()
	c := &Client{SocketPath: "/nonexistent/path/rootd.sock", DialTimeout: 100 * time.Millisecond}
	_, err := c.Ping(context.Background())
	if err == nil {
		t.Fatalf("expected dial error against missing socket")
	}
}

// TestClientEmptySocketPath — defensive: empty SocketPath returns a
// clean error rather than dialing "".
func TestClientEmptySocketPath(t *testing.T) {
	t.Parallel()
	c := &Client{}
	_, err := c.Ping(context.Background())
	if err == nil {
		t.Fatalf("expected error for empty SocketPath")
	}
}

// startRootdWithCustomHandlers boots a Server with a caller-supplied
// handlers map so per-method tests can swap in stubs without touching
// the production handler code (which would require root for the real
// systemctl / install-butler paths).
func startRootdWithCustomHandlers(t *testing.T, handlers map[string]rootd.HandlerFunc) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "rootd.sock")
	srv := &rootd.Server{
		SocketPath: sock,
		PeerGroup:  "",
		Handlers:   handlers,
		Logger:     func(string, ...any) {},
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(sock); err == nil && info.Mode()&os.ModeSocket != 0 {
			return sock, func() {
				cancel()
				<-serveErr
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatalf("rootd test server did not bind in 2s; err=%v", <-serveErr)
	return "", func() {}
}

// TestClientInstallPluginRoundTrip verifies the typed InstallPlugin
// method serializes a fully-typed request, sends it through the wire
// shape rootd expects, and decodes a fully-typed response.
func TestClientInstallPluginRoundTrip(t *testing.T) {
	t.Parallel()
	stub := func(_ context.Context, params json.RawMessage) (any, error) {
		var got map[string]any
		if err := json.Unmarshal(params, &got); err != nil {
			t.Fatalf("server-side decode params: %v", err)
		}
		if got["plugin_id"] != "openclaw" || got["dry_run"] != true {
			t.Fatalf("install_plugin params not round-tripped: %v", got)
		}
		return map[string]any{
			"installed":      false,
			"target_path":    got["target_path"],
			"stdout_summary": "dry-run plan ok",
		}, nil
	}
	sock, stop := startRootdWithCustomHandlers(t, map[string]rootd.HandlerFunc{"install_plugin": stub})
	defer stop()

	c := &Client{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.InstallPlugin(ctx, InstallPluginRequest{
		ManifestURL:  "https://example/manifest.json",
		PubKeyBase64: "AA",
		PluginID:     "openclaw",
		TargetPath:   "/usr/local/lib/borgee/openclaw",
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("InstallPlugin err: %v", err)
	}
	if got.Installed || got.TargetPath != "/usr/local/lib/borgee/openclaw" {
		t.Fatalf("InstallPlugin unexpected response: %+v", got)
	}
	if got.StdoutSummary != "dry-run plan ok" {
		t.Fatalf("StdoutSummary lost: %q", got.StdoutSummary)
	}
}

// TestClientServiceLifecycleRoundTrip mirrors the install_plugin test
// for the service_lifecycle typed method.
func TestClientServiceLifecycleRoundTrip(t *testing.T) {
	t.Parallel()
	stub := func(_ context.Context, params json.RawMessage) (any, error) {
		var got map[string]any
		_ = json.Unmarshal(params, &got)
		if got["manager"] != "systemd" || got["operation"] != "restart" {
			t.Fatalf("service_lifecycle params not round-tripped: %v", got)
		}
		return map[string]any{
			"exit_code": 0,
			"stdout":    "ok",
		}, nil
	}
	sock, stop := startRootdWithCustomHandlers(t, map[string]rootd.HandlerFunc{"service_lifecycle": stub})
	defer stop()

	c := &Client{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.ServiceLifecycle(ctx, ServiceLifecycleRequest{
		Manager: "systemd", Unit: "openclaw.service", Operation: "restart",
	})
	if err != nil {
		t.Fatalf("ServiceLifecycle err: %v", err)
	}
	if got.ExitCode != 0 || got.Stdout != "ok" {
		t.Fatalf("ServiceLifecycle unexpected response: %+v", got)
	}
}

// TestClientDelegationRevokeRoundTrip mirrors the install_plugin test
// for the delegation_revoke typed method.
func TestClientDelegationRevokeRoundTrip(t *testing.T) {
	t.Parallel()
	stub := func(_ context.Context, params json.RawMessage) (any, error) {
		var got map[string]any
		_ = json.Unmarshal(params, &got)
		if got["enrollment_id"] != "enroll-x" {
			t.Fatalf("delegation_revoke params not round-tripped: %v", got)
		}
		return map[string]any{
			"disabled":         true,
			"credential_wiped": true,
			"wiped_paths":      []string{"/var/lib/borgee/credential/credential"},
		}, nil
	}
	sock, stop := startRootdWithCustomHandlers(t, map[string]rootd.HandlerFunc{"delegation_revoke": stub})
	defer stop()

	c := &Client{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.DelegationRevoke(ctx, DelegationRevokeRequest{
		EnrollmentID:    "enroll-x",
		CredentialPaths: []string{"/var/lib/borgee/credential/credential"},
	})
	if err != nil {
		t.Fatalf("DelegationRevoke err: %v", err)
	}
	if !got.Disabled || !got.CredentialWiped || len(got.WipedPaths) != 1 {
		t.Fatalf("DelegationRevoke unexpected response: %+v", got)
	}
}

// TestClientPropagatesHandlerError verifies the typed methods surface
// rootd's `ok:false, error:"..."` envelope as a Go error (with the
// reason string preserved so the daemon executor can map onto a
// terminal failure_code).
func TestClientPropagatesHandlerError(t *testing.T) {
	t.Parallel()
	stub := func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, errors.New("manifest_fetch_failed: GET https://x.example: dial tcp: lookup")
	}
	sock, stop := startRootdWithCustomHandlers(t, map[string]rootd.HandlerFunc{"install_plugin": stub})
	defer stop()

	c := &Client{SocketPath: sock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.InstallPlugin(ctx, InstallPluginRequest{
		ManifestURL: "https://x.example/m.json", PubKeyBase64: "AA",
		PluginID: "openclaw", TargetPath: "/usr/local/lib/borgee/openclaw",
	})
	if err == nil || !strings.Contains(err.Error(), "manifest_fetch_failed") {
		t.Fatalf("expected manifest_fetch_failed propagation, got %v", err)
	}
}
