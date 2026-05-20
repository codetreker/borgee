//go:build linux || darwin

package rootdclient

import (
	"context"
	"os"
	"path/filepath"
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
