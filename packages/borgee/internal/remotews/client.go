package remotews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"borgee/internal/fsops"
)

// ErrAuthRejected is returned by Run when the server rejects the token — an
// HTTP 401/403 at the dial/upgrade, or an auth-shaped WS close code/reason.
// daemon.go branches on this sentinel to exit non-zero. A clean ctx-cancel
// (SIGINT/SIGTERM) returns nil instead.
var ErrAuthRejected = errors.New("remotews: auth rejected by server")

// Tuning constants mirror agent.ts (reconnect 1s→×2→cap 30s; heartbeat 30s;
// graceful close 1000 "shutting down").
const (
	initialBackoff      = 1 * time.Second
	maxBackoff          = 30 * time.Second
	backoffFactor       = 2
	heartbeatInterval   = 30 * time.Second
	gracefulCloseCode   = 1000
	gracefulCloseReason = "shutting down"
)

// Conn is the minimal websocket surface remotews needs. The default impl wraps
// *websocket.Conn; tests inject an in-memory fake so the run loop never touches
// a real socket.
type Conn interface {
	Read(ctx context.Context) (data []byte, err error)
	Write(ctx context.Context, data []byte) error
	Close(code int, reason string) error
}

// DialFunc dials the server and returns a Conn plus the handshake response.
// The *http.Response MAY be nil on a transport failure (connection refused /
// DNS) — the auth-reject predicate nil-checks it.
type DialFunc func(ctx context.Context, rawURL string) (Conn, *http.Response, error)

// Clock abstracts time so backoff + heartbeat are deterministic under test.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer mirrors the slice of *time.Timer remotews uses.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// FSOps is the read-only filesystem surface dispatched from inbound requests.
// The default impl wraps the fsops package.
type FSOps interface {
	Ls(allowed []string, path string) (any, string)
	Read(allowed []string, path string) (any, string)
	Stat(allowed []string, path string) (any, string)
}

// Config configures a Client.
type Config struct {
	ServerURL   string   // ws(s)://host — Client appends /ws/remote (token rides the Authorization header)
	Token       string   // opaque hex, sent as Authorization: Bearer <token> on the WS handshake
	AllowedDirs []string // from --dirs, passed straight to fsops

	OnFirstHandshake func(token string)            // persist-on-first-open seam
	OnAuthRejected   func(code int, reason string) // optional observer; the sentinel is authoritative

	// DI seams (nil → real impls).
	Dial  DialFunc
	Clock Clock
	FS    FSOps
}

// Client is the reverse-WS client. Construct with New, drive with Run.
type Client struct {
	cfg               Config
	dial              DialFunc
	clock             Clock
	fs                FSOps
	firstHandshakeHit bool
}

// New builds a Client, filling nil seams with the real implementations.
func New(cfg Config) *Client {
	c := &Client{cfg: cfg, dial: cfg.Dial, clock: cfg.Clock, fs: cfg.FS}
	if c.dial == nil {
		// Default dialer carries the token on the Authorization header (NOT in
		// the URL); the DialFunc seam stays URL-only so test fakes are unchanged.
		token := cfg.Token
		c.dial = func(ctx context.Context, rawURL string) (Conn, *http.Response, error) {
			return dialWebsocket(ctx, rawURL, token)
		}
	}
	if c.clock == nil {
		c.clock = realClock{}
	}
	if c.fs == nil {
		c.fs = fsopsAdapter{}
	}
	return c
}

// Run blocks, maintaining the connection with reconnect + heartbeat, until the
// context is cancelled (returns nil) or the server rejects the token (returns
// ErrAuthRejected).
func (c *Client) Run(ctx context.Context) error {
	backoff := initialBackoff
	dialURL, err := c.dialURL()
	if err != nil {
		return err
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		conn, resp, err := c.dial(ctx, dialURL)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if dialAuthReject(resp, err) {
				c.fireAuthRejected(authCode(resp), authReason(resp))
				return ErrAuthRejected
			}
			// Transient failure (including a nil resp from a refused
			// connection): back off and retry.
			if !c.sleep(ctx, backoff) {
				return nil
			}
			backoff = nextBackoff(backoff)
			continue
		}

		backoff = initialBackoff // reset on a successful open
		if !c.firstHandshakeHit {
			c.firstHandshakeHit = true
			c.fireFirstHandshake()
		}

		authRejected, err := c.serve(ctx, conn)
		if authRejected {
			c.fireAuthRejected(closeCode(err), closeReason(err))
			return ErrAuthRejected
		}
		if ctx.Err() != nil {
			return nil
		}
		// Connection dropped without an auth reject → reconnect after a backoff.
		if !c.sleep(ctx, backoff) {
			return nil
		}
		backoff = nextBackoff(backoff)
	}
}

// serve runs the read loop + heartbeat for one open connection. It returns
// (true, closeErr) when the disconnect was an auth reject, else (false, _).
// On ctx-cancel it closes gracefully and returns (false, nil).
func (c *Client) serve(ctx context.Context, conn Conn) (authRejected bool, err error) {
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	send := make(chan Frame, 16)
	done := make(chan struct{})
	defer close(done)

	// Single write pump (mirrors the server's writePump).
	go c.writePump(connCtx, conn, send, done)
	// Heartbeat: a ping every heartbeatInterval.
	go c.heartbeat(connCtx, send, done)

	for {
		data, readErr := conn.Read(connCtx)
		if readErr != nil {
			cancel()
			if ctx.Err() != nil {
				// Graceful shutdown requested by the caller.
				_ = conn.Close(gracefulCloseCode, gracefulCloseReason)
				return false, nil
			}
			if closeAuthReject(readErr) {
				return true, readErr
			}
			return false, readErr
		}

		var fr Frame
		if jsonErr := json.Unmarshal(data, &fr); jsonErr != nil {
			continue // ignore malformed frames, mirror server-side
		}
		switch fr.Type {
		case "request":
			resp := c.handleRequest(fr)
			select {
			case send <- resp:
			case <-connCtx.Done():
			}
		case "ping":
			select {
			case send <- Frame{Type: "pong"}:
			case <-connCtx.Done():
			}
		default:
			// "pong" / anything else: ignore (mark-alive is implicit).
		}
	}
}

// writePump serializes outbound frames onto the single connection.
func (c *Client) writePump(ctx context.Context, conn Conn, send <-chan Frame, done <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case fr := <-send:
			b, err := json.Marshal(fr)
			if err != nil {
				continue
			}
			if err := conn.Write(ctx, b); err != nil {
				return
			}
		}
	}
}

// heartbeat enqueues a ping frame every heartbeatInterval until the connection
// closes.
func (c *Client) heartbeat(ctx context.Context, send chan<- Frame, done <-chan struct{}) {
	timer := c.clock.NewTimer(heartbeatInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-timer.C():
			select {
			case send <- Frame{Type: "ping"}:
			case <-ctx.Done():
				return
			case <-done:
				return
			}
			timer = c.clock.NewTimer(heartbeatInterval)
		}
	}
}

// handleRequest dispatches one inbound request frame to fsops and builds the
// response frame echoing the same id.
func (c *Client) handleRequest(fr Frame) Frame {
	var rd RequestData
	_ = json.Unmarshal(fr.Data, &rd) // a malformed body yields an empty action → unknown branch

	var (
		result  any
		errCode string
	)
	switch rd.Action {
	case "ls":
		result, errCode = c.fs.Ls(c.cfg.AllowedDirs, rd.Path)
	case "read":
		result, errCode = c.fs.Read(c.cfg.AllowedDirs, rd.Path)
	case "stat":
		result, errCode = c.fs.Stat(c.cfg.AllowedDirs, rd.Path)
	default:
		// Mirror agent.ts: a free-text error string (the server turns any
		// non-special error into a 502 carrying the raw text).
		return Frame{Type: "response", ID: fr.ID, Data: errorData(fmt.Sprintf("Unknown action: %s", rd.Action))}
	}

	if errCode != "" {
		return Frame{Type: "response", ID: fr.ID, Data: errorData(errCode)}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return Frame{Type: "response", ID: fr.ID, Data: errorData(err.Error())}
	}
	return Frame{Type: "response", ID: fr.ID, Data: data}
}

// errorData builds a {"error":"<code>"} payload.
func errorData(code string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"error": code})
	return b
}

// sleep waits d using the injected clock, returning false if the context was
// cancelled first.
func (c *Client) sleep(ctx context.Context, d time.Duration) bool {
	timer := c.clock.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C():
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Client) fireFirstHandshake() {
	if c.cfg.OnFirstHandshake == nil {
		return
	}
	defer func() { _ = recover() }() // a misbehaving callback must not kill the loop
	c.cfg.OnFirstHandshake(c.cfg.Token)
}

func (c *Client) fireAuthRejected(code int, reason string) {
	if c.cfg.OnAuthRejected == nil {
		return
	}
	defer func() { _ = recover() }()
	c.cfg.OnAuthRejected(code, reason)
}

func (c *Client) dialURL() (string, error) {
	base := strings.TrimRight(c.cfg.ServerURL, "/")
	if base == "" {
		return "", errors.New("remotews: empty server URL")
	}
	// Token is NOT in the URL — it rides the Authorization: Bearer header set by
	// dialWebsocket, so it never leaks into proxy/access logs or referrers.
	return base + "/ws/remote", nil
}

func nextBackoff(cur time.Duration) time.Duration {
	next := cur * backoffFactor
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// dialAuthReject inspects the dial result. resp MAY be nil (coder/websocket
// returns nil,nil,err on a transport failure like connection-refused/DNS);
// reading resp.StatusCode without the nil-guard would panic on every refused
// retry. A nil resp → transient (reconnect).
func dialAuthReject(resp *http.Response, err error) bool {
	if err == nil {
		return false
	}
	if resp == nil {
		return false // transport failure → transient
	}
	return resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden
}

func authCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func authReason(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Status
}

// closeAuthReject mirrors agent.ts: a WS close code in {4001,4003,1008} or a
// close reason mentioning unauthorized / invalid token / token revoked.
func closeAuthReject(err error) bool {
	switch int(websocket.CloseStatus(err)) {
	case 4001, 4003, 1008:
		return true
	}
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		r := strings.ToLower(ce.Reason)
		return strings.Contains(r, "unauthorized") ||
			strings.Contains(r, "invalid token") ||
			strings.Contains(r, "token revoked")
	}
	return false
}

func closeCode(err error) int {
	return int(websocket.CloseStatus(err))
}

func closeReason(err error) string {
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return ce.Reason
	}
	return ""
}

// ---- default seam implementations ----

// dialWebsocket is the production DialFunc body: it dials via coder/websocket,
// putting the token on the Authorization: Bearer header (never in the URL), and
// adapts *websocket.Conn to the Conn interface.
func dialWebsocket(ctx context.Context, rawURL string, token string) (Conn, *http.Response, error) {
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}
	conn, resp, err := websocket.Dial(ctx, rawURL, opts)
	if err != nil {
		return nil, resp, err
	}
	// The server may send large frames (a 2 MiB file read); lift the default
	// read limit so a legitimate response is never truncated/closed.
	conn.SetReadLimit(-1)
	return &wsConn{conn: conn}, resp, nil
}

type wsConn struct {
	conn *websocket.Conn
}

func (w *wsConn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := w.conn.Read(ctx) // server always sends text; discard the type
	return data, err
}

func (w *wsConn) Write(ctx context.Context, data []byte) error {
	return w.conn.Write(ctx, websocket.MessageText, data)
}

func (w *wsConn) Close(code int, reason string) error {
	return w.conn.Close(websocket.StatusCode(code), reason)
}

// realClock is the production Clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{t: time.NewTimer(d)}
}

type realTimer struct{ t *time.Timer }

func (r *realTimer) C() <-chan time.Time { return r.t.C }
func (r *realTimer) Stop() bool          { return r.t.Stop() }

// fsopsAdapter is the production FSOps, wrapping the fsops package and
// collapsing the (result, ErrCode, error) returns to (any, string). The error
// return is reserved (always nil today).
type fsopsAdapter struct{}

func (fsopsAdapter) Ls(allowed []string, path string) (any, string) {
	res, code, _ := fsops.Ls(allowed, path)
	return res, string(code)
}

func (fsopsAdapter) Read(allowed []string, path string) (any, string) {
	res, code, _ := fsops.Read(allowed, path)
	return res, string(code)
}

func (fsopsAdapter) Stat(allowed []string, path string) (any, string) {
	res, code, _ := fsops.Stat(allowed, path)
	return res, string(code)
}
