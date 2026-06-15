package remotews

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ---- fake seams ----

// fakeTimer is a manually-fired timer. State is guarded by the shared clock
// mutex (clk) so the run loop's Stop() never races a test's fireNext().
type fakeTimer struct {
	clk     *sync.Mutex
	ch      chan time.Time
	d       time.Duration
	stopped bool
	fired   bool
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }
func (t *fakeTimer) Stop() bool {
	t.clk.Lock()
	t.stopped = true
	t.clk.Unlock()
	return true
}

// fakeClock hands out fakeTimers and records them so a test can fire a timer
// of a given duration deterministically (no wall-clock sleeps).
type fakeClock struct {
	mu     sync.Mutex
	timers []*fakeTimer
	now    time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Unix(0, 0)} }

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) NewTimer(d time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clk: &c.mu, ch: make(chan time.Time, 1), d: d}
	c.timers = append(c.timers, t)
	return t
}

// fireNext fires the oldest not-yet-fired live timer of duration d. It polls
// briefly for the timer to be registered (the run loop creates it from a
// goroutine).
func (c *fakeClock) fireNext(t *testing.T, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		for _, tm := range c.timers {
			if !tm.stopped && !tm.fired && tm.d == d {
				tm.fired = true
				tm.ch <- time.Time{}
				c.mu.Unlock()
				return
			}
		}
		c.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("no live timer of duration %s appeared", d)
		}
		time.Sleep(time.Millisecond)
	}
}

// firedDurations returns the durations of every timer that was actually fired
// via fireNext, in fire order. Unfired timers (e.g. a heartbeat 30s timer on a
// connection that is about to drop) are excluded, so a backoff-sequence
// assertion is not polluted by the heartbeat timer.
func (c *fakeClock) firedDurations() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]time.Duration, 0, len(c.timers))
	for _, tm := range c.timers {
		if tm.fired {
			out = append(out, tm.d)
		}
	}
	return out
}

// fakeConn is an in-memory Conn: the test pushes inbound frames via inbound and
// reads outbound frames via written.
type fakeConn struct {
	mu        sync.Mutex
	inbound   chan []byte
	written   chan []byte
	readErr   error // returned by Read once inbound is drained + this is set
	closeCode int
	closeArgd bool
}

func newFakeConn() *fakeConn {
	return &fakeConn{
		inbound: make(chan []byte, 16),
		written: make(chan []byte, 16),
	}
}

func (f *fakeConn) Read(ctx context.Context) ([]byte, error) {
	select {
	case b := <-f.inbound:
		return b, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *fakeConn) Write(ctx context.Context, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	select {
	case f.written <- cp:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeConn) Close(code int, reason string) error {
	f.mu.Lock()
	f.closeCode = code
	f.closeArgd = true
	f.mu.Unlock()
	return nil
}

// nextWritten reads one outbound frame or fails after a short timeout.
func nextWritten(t *testing.T, f *fakeConn) Frame {
	t.Helper()
	select {
	case b := <-f.written:
		var fr Frame
		if err := json.Unmarshal(b, &fr); err != nil {
			t.Fatalf("unmarshal written frame %q: %v", b, err)
		}
		return fr
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a written frame")
		return Frame{}
	}
}

// ---- AC-3: token-persist ----

func TestClient_FirstHandshakePersistsOnce(t *testing.T) {
	var persisted int
	var mu sync.Mutex

	dialCount := 0
	dial := func(ctx context.Context, rawURL string) (Conn, *http.Response, error) {
		dialCount++
		switch dialCount {
		case 1, 2:
			// Open, then drop immediately (Read errors) to force a reconnect.
			// A drop after a successful open resets the backoff to 1s.
			return &dropConn{}, nil, nil
		default:
			// Stay open forever (block on read until ctx cancel).
			return &blockConn{}, nil, nil
		}
	}

	clk := newFakeClock()
	c := New(Config{
		ServerURL: "ws://x",
		Token:     "tok",
		Dial:      dial,
		Clock:     clk,
		OnFirstHandshake: func(string) {
			mu.Lock()
			persisted++
			mu.Unlock()
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	// Open #1 drops → backoff 1s; open #2 drops → backoff 1s (reset on open);
	// open #3 stays. Three opens, OnFirstHandshake must fire exactly once.
	clk.fireNext(t, 1*time.Second)
	clk.fireNext(t, 1*time.Second)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if persisted != 1 {
		t.Errorf("OnFirstHandshake fired %d times; want exactly 1 across reconnects", persisted)
	}
}

// dropConn errors on the first Read (simulates an immediate disconnect).
type dropConn struct{}

func (dropConn) Read(ctx context.Context) ([]byte, error) { return nil, errors.New("dropped") }
func (dropConn) Write(ctx context.Context, b []byte) error { return nil }
func (dropConn) Close(int, string) error                  { return nil }

// blockConn blocks Read until the context is cancelled.
type blockConn struct{}

func (blockConn) Read(ctx context.Context) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }
func (blockConn) Write(ctx context.Context, b []byte) error { return nil }
func (blockConn) Close(int, string) error                  { return nil }

// ---- AC-3: ping-pong ----

func TestClient_Heartbeat_PingEvery30s(t *testing.T) {
	fc := newFakeConn()
	dial := func(ctx context.Context, rawURL string) (Conn, *http.Response, error) {
		return fc, nil, nil
	}
	clk := newFakeClock()
	c := New(Config{ServerURL: "ws://x", Token: "tok", Dial: dial, Clock: clk})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	clk.fireNext(t, heartbeatInterval)
	if fr := nextWritten(t, fc); fr.Type != "ping" {
		t.Fatalf("first heartbeat frame = %q; want ping", fr.Type)
	}
	clk.fireNext(t, heartbeatInterval)
	if fr := nextWritten(t, fc); fr.Type != "ping" {
		t.Fatalf("second heartbeat frame = %q; want ping", fr.Type)
	}
}

func TestClient_RespondsPongToServerPing(t *testing.T) {
	fc := newFakeConn()
	dial := func(ctx context.Context, rawURL string) (Conn, *http.Response, error) {
		return fc, nil, nil
	}
	c := New(Config{ServerURL: "ws://x", Token: "tok", Dial: dial, Clock: newFakeClock()})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	fc.inbound <- []byte(`{"type":"ping"}`)
	if fr := nextWritten(t, fc); fr.Type != "pong" {
		t.Fatalf("reply to server ping = %q; want pong", fr.Type)
	}

	// A server pong must NOT produce a reply.
	fc.inbound <- []byte(`{"type":"pong"}`)
	select {
	case b := <-fc.written:
		t.Fatalf("server pong produced an unexpected reply: %q", b)
	case <-time.After(50 * time.Millisecond):
		// good — no reply
	}
}

// ---- AC-3: reconnect backoff ----

func TestClient_ReconnectBackoffSequence(t *testing.T) {
	var mu sync.Mutex
	openOnAttempt := 7 // attempts 1..6 fail transiently, the 7th opens
	attempts := 0
	dial := func(ctx context.Context, rawURL string) (Conn, *http.Response, error) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < openOnAttempt {
			return nil, nil, errors.New("refused") // transient (resp nil)
		}
		return &blockConn{}, nil, nil
	}
	clk := newFakeClock()
	c := New(Config{ServerURL: "ws://x", Token: "tok", Dial: dial, Clock: clk})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	// Six transient failures → six backoff sleeps: 1,2,4,8,16,30 (capped).
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}
	for _, d := range want {
		clk.fireNext(t, d)
	}
	// Give the loop a moment to reach the open.
	time.Sleep(20 * time.Millisecond)

	got := clk.firedDurations()
	// The first len(want) backoff timers must match the doubling/capped seq.
	if len(got) < len(want) {
		t.Fatalf("only %d timers created; want >= %d", len(got), len(want))
	}
	for i, d := range want {
		if got[i] != d {
			t.Errorf("backoff[%d] = %s; want %s (full=%v)", i, got[i], d, got[:len(want)])
		}
	}
}

func TestClient_BackoffResetsAfterOpen(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	dial := func(ctx context.Context, rawURL string) (Conn, *http.Response, error) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		switch n {
		case 1:
			return nil, nil, errors.New("refused") // transient → backoff 1s
		case 2:
			return &dropConn{}, nil, nil // opens then immediately drops → backoff should be 1s again (reset)
		default:
			return &blockConn{}, nil, nil
		}
	}
	clk := newFakeClock()
	c := New(Config{ServerURL: "ws://x", Token: "tok", Dial: dial, Clock: clk})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	clk.fireNext(t, 1*time.Second) // backoff after attempt 1
	// attempt 2 opens then drops; backoff resets to 1s (NOT 2s).
	clk.fireNext(t, 1*time.Second)
	time.Sleep(20 * time.Millisecond)

	got := clk.firedDurations()
	if len(got) < 2 {
		t.Fatalf("only %d timers; want >= 2", len(got))
	}
	if got[0] != 1*time.Second || got[1] != 1*time.Second {
		t.Errorf("backoff seq = %v; want [1s 1s] (reset-on-open)", got[:2])
	}
}

// ---- AC-3: auth-reject ----

func TestClient_AuthReject(t *testing.T) {
	run := func(t *testing.T, dial DialFunc) (error, int, bool) {
		var rejectedCode int
		var rejected bool
		c := New(Config{
			ServerURL: "ws://x", Token: "tok", Dial: dial, Clock: newFakeClock(),
			OnAuthRejected: func(code int, reason string) { rejectedCode = code; rejected = true },
		})
		errCh := make(chan error, 1)
		go func() { errCh <- c.Run(context.Background()) }()
		select {
		case err := <-errCh:
			return err, rejectedCode, rejected
		case <-time.After(3 * time.Second):
			t.Fatal("Run did not return")
			return nil, 0, false
		}
	}

	t.Run("dial_401_stops", func(t *testing.T) {
		err, code, rejected := run(t, func(ctx context.Context, u string) (Conn, *http.Response, error) {
			return nil, &http.Response{StatusCode: 401, Status: "401 Unauthorized"}, errors.New("upgrade rejected")
		})
		if !errors.Is(err, ErrAuthRejected) {
			t.Errorf("err = %v; want ErrAuthRejected", err)
		}
		if !rejected || code != 401 {
			t.Errorf("OnAuthRejected fired=%v code=%d; want true/401", rejected, code)
		}
	})

	t.Run("dial_403_stops", func(t *testing.T) {
		err, _, _ := run(t, func(ctx context.Context, u string) (Conn, *http.Response, error) {
			return nil, &http.Response{StatusCode: 403, Status: "403 Forbidden"}, errors.New("forbidden")
		})
		if !errors.Is(err, ErrAuthRejected) {
			t.Errorf("err = %v; want ErrAuthRejected", err)
		}
	})

	t.Run("close_4001_stops", func(t *testing.T) {
		first := true
		err, _, rejected := run(t, func(ctx context.Context, u string) (Conn, *http.Response, error) {
			if first {
				first = false
				return &closeErrConn{err: websocket.CloseError{Code: 4001, Reason: "bad token"}}, nil, nil
			}
			return &blockConn{}, nil, nil
		})
		if !errors.Is(err, ErrAuthRejected) {
			t.Errorf("err = %v; want ErrAuthRejected", err)
		}
		if !rejected {
			t.Error("OnAuthRejected not fired for close 4001")
		}
	})

	t.Run("close_reason_token_revoked_stops", func(t *testing.T) {
		err, _, _ := run(t, func(ctx context.Context, u string) (Conn, *http.Response, error) {
			return &closeErrConn{err: websocket.CloseError{Code: 1011, Reason: "token revoked"}}, nil, nil
		})
		if !errors.Is(err, ErrAuthRejected) {
			t.Errorf("err = %v; want ErrAuthRejected", err)
		}
	})

	t.Run("dial_refused_resp_nil_reconnects", func(t *testing.T) {
		// resp is nil (connection refused). Must NOT panic and must NOT
		// auth-reject — it reconnects, then opens on attempt 2.
		var mu sync.Mutex
		attempts := 0
		clk := newFakeClock()
		var rejected bool
		c := New(Config{
			ServerURL: "ws://x", Token: "tok", Clock: clk,
			OnAuthRejected: func(int, string) { rejected = true },
			Dial: func(ctx context.Context, u string) (Conn, *http.Response, error) {
				mu.Lock()
				attempts++
				n := attempts
				mu.Unlock()
				if n == 1 {
					return nil, nil, errors.New("connection refused") // resp NIL
				}
				return &blockConn{}, nil, nil
			},
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- c.Run(ctx) }()
		clk.fireNext(t, 1*time.Second) // the reconnect backoff after the refused dial
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		n := attempts
		mu.Unlock()
		if n < 2 {
			t.Errorf("attempts = %d; want >= 2 (reconnected after refused)", n)
		}
		if rejected {
			t.Error("OnAuthRejected fired on a refused (resp-nil) dial; want reconnect")
		}
		select {
		case err := <-errCh:
			t.Fatalf("Run returned early (err=%v); want still reconnecting", err)
		default:
		}
	})

	t.Run("dial_502_reconnects", func(t *testing.T) {
		var mu sync.Mutex
		attempts := 0
		clk := newFakeClock()
		c := New(Config{
			ServerURL: "ws://x", Token: "tok", Clock: clk,
			Dial: func(ctx context.Context, u string) (Conn, *http.Response, error) {
				mu.Lock()
				attempts++
				n := attempts
				mu.Unlock()
				if n == 1 {
					return nil, &http.Response{StatusCode: 502, Status: "502 Bad Gateway"}, errors.New("bad gateway")
				}
				return &blockConn{}, nil, nil
			},
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go func() { errCh <- c.Run(ctx) }()
		clk.fireNext(t, 1*time.Second)
		time.Sleep(20 * time.Millisecond)
		select {
		case err := <-errCh:
			t.Fatalf("Run returned early on 502 (err=%v); want reconnect", err)
		default:
		}
	})
}

// closeErrConn returns a fixed error (a websocket.CloseError) on Read.
type closeErrConn struct{ err error }

func (c closeErrConn) Read(ctx context.Context) ([]byte, error) { return nil, c.err }
func (closeErrConn) Write(ctx context.Context, b []byte) error  { return nil }
func (closeErrConn) Close(int, string) error                    { return nil }

// ---- AC-2: integration against a real coder/websocket server ----

// TestClient_Integration_LsReadStat stands up a real coder/websocket server in
// an httptest.Server, lets the DEFAULT dialWebsocket adapter dial it, then
// drives ls/read/stat (plus unknown + outside-dir) request frames and asserts
// the daemon's responses.
func TestClient_Integration_LsReadStat(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hi there"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}

	type req struct {
		id     string
		action string
		path   string
	}
	type want struct {
		id     string
		assert func(t *testing.T, data json.RawMessage)
	}

	reqs := []req{
		{"r1", "ls", dir},
		{"r2", "read", filePath},
		{"r3", "stat", filePath},
		{"r4", "bogus", filePath},
		{"r5", "read", "/etc/shadow"}, // outside the allowed dir
	}

	responses := make(chan Frame, len(reqs))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("server accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()
		// Send each request, then read the matching response.
		for _, rq := range reqs {
			payload, _ := json.Marshal(RequestData{Action: rq.action, Path: rq.path})
			fr, _ := json.Marshal(Frame{Type: "request", ID: rq.id, Data: payload})
			if err := conn.Write(ctx, websocket.MessageText, fr); err != nil {
				return
			}
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var resp Frame
			if err := json.Unmarshal(data, &resp); err != nil {
				t.Errorf("server: bad response frame %q: %v", data, err)
				return
			}
			responses <- resp
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := New(Config{ServerURL: wsURL, Token: "tok", AllowedDirs: []string{dir}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	collected := make(map[string]Frame)
	for i := 0; i < len(reqs); i++ {
		select {
		case fr := <-responses:
			collected[fr.ID] = fr
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %d responses", i)
		}
	}

	// r1 ls → entries with hello.txt + sub.
	var ls struct {
		Entries []struct {
			Name        string `json:"name"`
			IsDirectory bool   `json:"isDirectory"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(collected["r1"].Data, &ls); err != nil {
		t.Fatalf("ls data %q: %v", collected["r1"].Data, err)
	}
	names := map[string]bool{}
	for _, e := range ls.Entries {
		names[e.Name] = e.IsDirectory
	}
	if _, ok := names["hello.txt"]; !ok {
		t.Errorf("ls missing hello.txt; got %v", names)
	}
	if isDir, ok := names["sub"]; !ok || !isDir {
		t.Errorf("ls missing sub dir (isDir=%v ok=%v)", isDir, ok)
	}

	// r2 read → content + mimeType + size.
	var rd struct {
		Content  string `json:"content"`
		MimeType string `json:"mimeType"`
		Size     int64  `json:"size"`
	}
	if err := json.Unmarshal(collected["r2"].Data, &rd); err != nil {
		t.Fatalf("read data %q: %v", collected["r2"].Data, err)
	}
	if rd.Content != "hi there" || rd.MimeType != "text/plain" || rd.Size != 8 {
		t.Errorf("read = %+v; want content=hi there mime=text/plain size=8", rd)
	}

	// r3 stat → size + isDirectory.
	var st struct {
		Size        int64 `json:"size"`
		IsDirectory bool  `json:"isDirectory"`
	}
	if err := json.Unmarshal(collected["r3"].Data, &st); err != nil {
		t.Fatalf("stat data %q: %v", collected["r3"].Data, err)
	}
	if st.Size != 8 || st.IsDirectory {
		t.Errorf("stat = %+v; want size=8 isDirectory=false", st)
	}

	// r4 unknown action → {"error":"Unknown action: bogus"}.
	if got := errString(t, collected["r4"].Data); got != "Unknown action: bogus" {
		t.Errorf("unknown action error = %q; want \"Unknown action: bogus\"", got)
	}

	// r5 outside-dir read → {"error":"path_not_allowed"}.
	if got := errString(t, collected["r5"].Data); got != "path_not_allowed" {
		t.Errorf("outside-dir error = %q; want path_not_allowed", got)
	}
}

func errString(t *testing.T, data json.RawMessage) string {
	t.Helper()
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("unmarshal error data %q: %v", data, err)
	}
	return e.Error
}

// TestClient_Integration_Dial401Rejects is the review Nit: exercise the
// DEFAULT dialWebsocket adapter against a server that returns HTTP 401 at the
// upgrade (before websocket.Accept), proving the dial-time auth reject end to
// end — not just the pure predicate.
func TestClient_Integration_Dial401Rejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	var gotCode int
	c := New(Config{
		ServerURL:      wsURL,
		Token:          "bad",
		OnAuthRejected: func(code int, reason string) { gotCode = code },
	})

	errCh := make(chan error, 1)
	go func() { errCh <- c.Run(context.Background()) }()
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrAuthRejected) {
			t.Fatalf("Run err = %v; want ErrAuthRejected from a real 401 upgrade", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return on a real 401")
	}
	if gotCode != http.StatusUnauthorized {
		t.Errorf("OnAuthRejected code = %d; want 401", gotCode)
	}
}

// TestClient_Integration_AuthHeaderNotURL proves the migrated default dialer
// carries the token on the Authorization: Bearer header and that the request
// URI does NOT contain the token (no ?token= query) — the whole point of the
// ws-auth-unify task-2 migration.
func TestClient_Integration_AuthHeaderNotURL(t *testing.T) {
	const token = "deadbeefcafe"
	gotAuthCh := make(chan string, 1)
	gotURICh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case gotAuthCh <- r.Header.Get("Authorization"):
		default:
		}
		select {
		case gotURICh <- r.URL.RequestURI():
		default:
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("server accept: %v", err)
			return
		}
		// Hold the connection open briefly so the client treats it as a clean
		// open; the test only cares about the handshake request.
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, _, _ = conn.Read(r.Context())
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := New(Config{ServerURL: wsURL, Token: token})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	select {
	case gotAuth := <-gotAuthCh:
		if gotAuth != "Bearer "+token {
			t.Errorf("Authorization header = %q; want %q", gotAuth, "Bearer "+token)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never received the handshake")
	}

	gotURI := <-gotURICh
	if strings.Contains(gotURI, "token") {
		t.Errorf("request URI %q contains the token; it must ride the header only", gotURI)
	}
	if !strings.HasSuffix(gotURI, "/ws/remote") {
		t.Errorf("request URI = %q; want it to end with /ws/remote (no query)", gotURI)
	}
}
