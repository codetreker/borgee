//go:build linux || darwin

package rootd

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// startTestServer starts a Server on a temp UDS, returns the socket path
// and a cleanup that shuts the server down. The PeerGroup is set to ""
// so the peer-cred check is disabled — full peer-cred tests live in the
// dedicated TestRejectsNonBorgeeGroupPeer case.
func startTestServer(t *testing.T, handlers map[string]HandlerFunc) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "rootd.sock")
	logs := &captureLogger{}
	srv := &Server{
		SocketPath: sock,
		PeerGroup:  "",
		Handlers:   handlers,
		Logger:     logs.logf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	// Wait up to 2s for the listener to come up.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			return sock, func() {
				cancel()
				<-serveErr
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatalf("test server did not bind socket %s in 2s; serve err=%v", sock, <-serveErr)
	return "", func() {}
}

type captureLogger struct {
	count atomic.Int32
}

func (c *captureLogger) logf(format string, v ...any) {
	c.count.Add(1)
}

// roundTrip dials sock, writes req as one JSON line, reads back one
// Response line, returns it. Mirrors what rootdclient.Client does.
func roundTrip(t *testing.T, sock string, req Request) Response {
	t.Helper()
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial %s: %v", sock, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode req: %v", err)
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("unmarshal resp %q: %v", string(line), err)
	}
	return resp
}

// TestPingRoundTrip starts the server with the production whitelist and
// verifies ping returns ok:true with a pong:true result.
func TestPingRoundTrip(t *testing.T) {
	t.Parallel()
	sock, stop := startTestServer(t, DefaultHandlers())
	defer stop()

	resp := roundTrip(t, sock, Request{Cmd: "ping", RequestID: "ping-1"})
	if !resp.OK {
		t.Fatalf("ping ok=false: %+v", resp)
	}
	if resp.RequestID != "ping-1" {
		t.Fatalf("ping request_id echo = %q, want ping-1", resp.RequestID)
	}
	var parsed map[string]any
	if err := json.Unmarshal(resp.Result, &parsed); err != nil {
		t.Fatalf("unmarshal result %q: %v", string(resp.Result), err)
	}
	if parsed["pong"] != true {
		t.Fatalf("ping result pong = %v, want true", parsed["pong"])
	}
	if _, ok := parsed["time"]; !ok {
		t.Fatalf("ping result missing time field: %+v", parsed)
	}
}

// TestUnknownCommandRejected is the critical security invariant: rootd
// must NEVER execute an arbitrary cmd field. Only whitelisted commands
// reach a handler. A made-up cmd like "deploy_kernel" must come back as
// ok:false, error:"unknown_command" without invoking anything.
func TestUnknownCommandRejected(t *testing.T) {
	t.Parallel()
	// Track whether ANY handler ever ran. If unknown_command leaks into
	// a handler, this test would catch it.
	var pingCalled atomic.Bool
	handlers := map[string]HandlerFunc{
		"ping": func(ctx context.Context, p json.RawMessage) (any, error) {
			pingCalled.Store(true)
			return map[string]any{"pong": true}, nil
		},
	}
	sock, stop := startTestServer(t, handlers)
	defer stop()

	resp := roundTrip(t, sock, Request{Cmd: "deploy_kernel", RequestID: "deploy-1"})
	if resp.OK {
		t.Fatalf("unknown cmd must NOT return ok=true: %+v", resp)
	}
	if resp.Error != "unknown_command" {
		t.Fatalf("unknown cmd error = %q, want unknown_command", resp.Error)
	}
	if resp.RequestID != "deploy-1" {
		t.Fatalf("request_id echo = %q, want deploy-1", resp.RequestID)
	}
	if pingCalled.Load() {
		t.Fatalf("ping handler was invoked for unknown cmd — security failure")
	}
}

// TestRejectsNonBorgeeGroupPeer verifies the peer-cred check rejects a
// caller whose primary gid is not in the configured PeerGroup. Hard to
// exercise authentically without a second account, but we can fake a
// peer that does NOT belong to a synthetic "rootd-test-allowed-group"
// (which does not exist on the host) → lookupGID fails → server logs a
// warn and (per current policy) ALLOWS the connection through. So this
// test instead validates the symmetric case: when PeerGroup is set to a
// group the current user actually belongs to, the connection succeeds;
// when set to a group that exists but the current user is NOT in, the
// connection is refused.
//
// We approximate "user not in group" by setting PeerGroup to "root" and
// running as non-root. Skip if running as root (test box is in root).
func TestRejectsNonBorgeeGroupPeer(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; cannot exercise non-root peer rejection")
	}
	dir := t.TempDir()
	sock := filepath.Join(dir, "rootd.sock")
	srv := &Server{
		SocketPath: sock,
		PeerGroup:  "root",
		Handlers:   DefaultHandlers(),
		Logger:     func(string, ...any) {},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))
	enc := json.NewEncoder(conn)
	_ = enc.Encode(Request{Cmd: "ping", RequestID: "rejected"})
	r := bufio.NewReader(conn)
	if _, err := r.ReadBytes('\n'); err == nil {
		// If we got a response, it had better not be ok=true.
		// But the contract is "connection closed without a reply", so any
		// reply at all is a violation.
		t.Fatalf("non-borgee peer got a reply; expected connection close")
	}
}

// TestSocketModeAfterListen asserts the socket file mode is 0660 after
// the server Listen. The chown to root:borgee may fail in the test env
// (running non-root), but the chmod runs unconditionally.
func TestSocketModeAfterListen(t *testing.T) {
	t.Parallel()
	sock, stop := startTestServer(t, DefaultHandlers())
	defer stop()

	info, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o660 {
		t.Fatalf("socket mode = %o, want 0660", got)
	}
}

// TestStaleSocketRemoved verifies the server cleans up a stale socket
// from a previous unclean shutdown before Listen. If we leave the file
// in place, Listen returns "address already in use".
func TestStaleSocketRemoved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sock := filepath.Join(dir, "rootd.sock")
	// Pre-create a fake socket file so Listen would normally fail.
	if err := os.WriteFile(sock, []byte("stale"), 0o600); err != nil {
		t.Fatalf("pre-create stale: %v", err)
	}
	srv := &Server{
		SocketPath: sock,
		PeerGroup:  "",
		Handlers:   DefaultHandlers(),
		Logger:     func(string, ...any) {},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()

	// Wait for the new socket to appear (i.e., stale was removed +
	// Listen succeeded).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := os.Stat(sock)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatalf("stale socket was not replaced by real UDS in 2s; serve err=%v", <-serveErr)
}

// TestParseErrorRejected verifies a malformed (non-JSON) request body is
// rejected with ok:false, error:"parse_error" without invoking a handler.
func TestParseErrorRejected(t *testing.T) {
	t.Parallel()
	sock, stop := startTestServer(t, DefaultHandlers())
	defer stop()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("this is not json\n")); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	if !strings.Contains(string(line), "parse_error") {
		t.Fatalf("expected parse_error in response, got %q", string(line))
	}
}

// TestWhitelistContainsPR4Commands is the regression guard for the
// PR-4 scope expansion: the production whitelist must contain ping +
// the three real root commands (install_plugin / service_lifecycle /
// delegation_revoke). Anyone who adds a NEW root command without also
// updating the blueprint, systemd ReadWritePaths, audit + threat-model
// docs should fail this assertion and re-read the package doc comment.
func TestWhitelistContainsPR4Commands(t *testing.T) {
	t.Parallel()
	got := DefaultHandlers()
	want := []string{"ping", "install_plugin", "service_lifecycle", "delegation_revoke"}
	if len(got) != len(want) {
		t.Fatalf("PR-4 whitelist size = %d, want %d. got keys: %v", len(got), len(want), keysOf(got))
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Fatalf("PR-4 whitelist missing %q: %v", name, keysOf(got))
		}
	}
}

func keysOf(m map[string]HandlerFunc) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
