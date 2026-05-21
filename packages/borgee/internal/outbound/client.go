package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Package outbound — PR-2 #1038 WebSocket transport for the daemon ↔
// server contract. Replaces the prior HTTP long-poll implementation.
// Public API surface preserved where the dispatcher consumes it
// (Ack/Result keep their signatures), and Poll is replaced by a new
// Receive that blocks on a WS read and returns one of: a leased job,
// a directive (stop_revoked / stop_stale_credential / ...), or an
// error (transport / unmarshal). The Client also runs a background
// 30-second ping/pong heartbeat that replaces the legacy POST /status
// freshness signal — server's existing 5min `connected`/`offline`
// window stays unchanged, the producer is now the WS pong.

// Directive enum carried in `{"type":"directive","code":...}` frames
// from server, and synthesized by the daemon on unauthorized closes.
type Directive string

const (
	DirectiveProcess             Directive = "process"
	DirectiveRetry               Directive = "retry"
	DirectiveStopUnauthorized    Directive = "stop_unauthorized"
	DirectiveStopStaleCredential Directive = "stop_stale_credential"
	DirectiveStopRevoked         Directive = "stop_revoked"
	DirectiveStopUninstalled     Directive = "stop_uninstalled"
	DirectiveDisplaced           Directive = "displaced"
)

// HelperWSSubprotocol mirrors the server-side ws.HelperWSSubprotocol.
// Hard-pinned so a future schema rev can bump and reject old clients.
const HelperWSSubprotocol = "borgee.helper.v1"

const (
	// PingInterval — application-level WS ping period. The server's
	// freshness window is 5min; 30s gives a 10x safety margin against
	// transient packet loss while keeping idle traffic cheap.
	PingInterval = 30 * time.Second

	// PingTimeout — per-ping deadline. Beyond this we count one miss.
	PingTimeout = 10 * time.Second

	// MissedPingsToReconnect — three consecutive ping failures trigger
	// a tear-down + reconnect (3x PingTimeout = ~30s budget which is
	// tight but well below the server's 5min `offline` cutover).
	MissedPingsToReconnect = 3

	// reconnect curve. exponential base→cap with 20% jitter, matches
	// the spec in #1038 §reconnect.
	reconnectBackoffBase = 1 * time.Second
	reconnectBackoffCap  = 30 * time.Second
)

type StaticCredentialSource struct {
	Credential     string
	HelperDeviceID string
}

type ClientOption func(*Client)

// WithHTTPClient overrides the underlying *http.Client used for the WS
// upgrade. Tests inject httptest.Server.Client() to share the test
// listener's TLS config; production keeps http.DefaultClient.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithLogger sets a logger callback used for transport-level diagnostics.
// nil → silent. Tests use this to capture reconnect / ping-miss lines.
func WithLogger(logger func(format string, v ...any)) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithPingInterval overrides the heartbeat cadence (tests only). Must
// be > 0; otherwise leaves the production default.
func WithPingInterval(d time.Duration) ClientOption {
	return func(c *Client) {
		if d > 0 {
			c.pingInterval = d
		}
	}
}

// WithReconnectBackoff overrides reconnect base/cap (tests only).
func WithReconnectBackoff(base, cap time.Duration) ClientOption {
	return func(c *Client) {
		if base > 0 {
			c.reconnectBase = base
		}
		if cap > 0 {
			c.reconnectCap = cap
		}
	}
}

type Client struct {
	serverOrigin string
	credential   StaticCredentialSource
	enrollmentID string
	httpClient   *http.Client
	logger       func(format string, v ...any)

	pingInterval  time.Duration
	reconnectBase time.Duration
	reconnectCap  time.Duration

	mu          sync.Mutex
	conn        *websocket.Conn
	connCtx     context.Context
	connCancel  context.CancelFunc
	missedPings int

	// inbound is the read-loop's output channel. Receive() reads from
	// it; the read goroutine writes one inboundFrame per WS frame
	// (decoded). Buffered 1 so a slow Receive doesn't deadlock the
	// reader (server unicasts one job at a time so it cannot get
	// ahead).
	inbound chan inboundFrame
}

// inboundFrame is the read-loop's decoded representation of one WS
// text frame. Exactly one of {Job, Directive, Pong} is set.
type inboundFrame struct {
	Job       *LeasedJob
	Directive Directive
}

// PollOptions kept for backward compat with code that still constructs
// the struct; ignored by Receive (WS is push-based).
type PollOptions struct {
	WaitMS int `json:"wait_ms,omitempty"`
}

// PollResult — kept for backward compat with the dispatcher's outer
// loop signature. Receive returns directly typed values; PollResult
// only appears in legacy adapter shims.
type PollResult struct {
	Status     string
	Directive  Directive
	RetryAfter time.Duration
	Job        *LeasedJob
}

const (
	PollStatusLeased = "leased"
	PollStatusNoWork = "no_work"
)

type LeasedJob struct {
	JobID          string          `json:"job_id"`
	EnrollmentID   string          `json:"enrollment_id"`
	JobType        string          `json:"job_type"`
	SchemaVersion  int             `json:"schema_version"`
	Payload        json.RawMessage `json:"payload"`
	ManifestDigest string          `json:"manifest_digest"`
	// ManifestJSON / ManifestBindingJSON carry the signed policy manifest
	// (per jobpolicy.PolicyManifest / ManifestBinding). The dispatcher's
	// jobpolicy.Evaluate verifies signature + binding ⊆ manifest paths
	// before the executor runs; the no-root executors then call
	// manifestpath.Resolve on these same bytes to translate a PathID
	// (e.g. openclaw_agent_config) into the real filesystem root they
	// must write under. Empty bytes → executor fails loud with
	// manifest_invalid; the daemon does not synthesize fallback paths.
	ManifestJSON        json.RawMessage `json:"manifest_json,omitempty"`
	ManifestBindingJSON json.RawMessage `json:"manifest_binding_json,omitempty"`
	LeaseToken          string          `json:"lease_token"`
	LeaseExpiresAt      int64           `json:"lease_expires_at"`
	Attempt             int             `json:"attempt"`
}

type JobState struct {
	JobID       string    `json:"job_id"`
	Status      string    `json:"status"`
	FailureCode string    `json:"failure_code,omitempty"`
	Directive   Directive `json:"-"`
}

type ResultSummary struct {
	AuditRefs []string `json:"audit_refs,omitempty"`
	LogRefs   []string `json:"log_refs,omitempty"`
}

type ResultRequest struct {
	LeaseToken     string
	Status         string
	FailureCode    string
	FailureMessage string
	ResultSummary  ResultSummary
}

// NewClient constructs an outbound client bound to a prepared config +
// helper credential pair. Does NOT dial — Dial() establishes the WS
// connection. The two-step shape lets the caller wire context first
// (heartbeat / dispatcher loops share the daemon's SIGTERM ctx).
func NewClient(cfg PreparedConfig, credential StaticCredentialSource, opts ...ClientOption) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("outbound client requires enabled prepared config")
	}
	if strings.TrimSpace(cfg.ServerOrigin) == "" {
		return nil, fmt.Errorf("outbound client requires server origin")
	}
	u, err := url.Parse(cfg.ServerOrigin)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid prepared server origin")
	}
	credential.Credential = strings.TrimSpace(credential.Credential)
	credential.HelperDeviceID = strings.TrimSpace(credential.HelperDeviceID)
	if credential.Credential == "" || credential.HelperDeviceID == "" {
		return nil, fmt.Errorf("helper credential and device id are required")
	}
	c := &Client{
		serverOrigin:  strings.TrimRight(cfg.ServerOrigin, "/"),
		credential:    credential,
		httpClient:    http.DefaultClient,
		pingInterval:  PingInterval,
		reconnectBase: reconnectBackoffBase,
		reconnectCap:  reconnectBackoffCap,
		inbound:       make(chan inboundFrame, 4),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// SetEnrollmentID binds the enrollment id used as the WS path segment
// for Dial. Required before Dial; the dispatcher's main.go wiring
// reads the enrollment-id from disk and calls this before the dial
// loop starts.
func (c *Client) SetEnrollmentID(id string) {
	c.enrollmentID = strings.TrimSpace(id)
}

// EnrollmentID returns the bound enrollment id (for tests / logs).
func (c *Client) EnrollmentID() string {
	return c.enrollmentID
}

// Dial establishes the WS connection. Spawns the read goroutine + the
// ping goroutine. Returns nil on a successful handshake; the read loop
// closes the connection on any inbound error and Receive will surface
// the disconnect. The dispatcher's outer loop calls Reconnect on
// transport errors.
func (c *Client) Dial(ctx context.Context) error {
	if err := validatePathID(c.enrollmentID); err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	dialURL, err := c.dialURL()
	if err != nil {
		return err
	}

	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+c.credential.Credential)
	hdr.Set("X-Helper-Device-Id", c.credential.HelperDeviceID)
	// PR-4 final amend: declare runtime.GOOS so the server picks the
	// matching signed canonical manifest body. Server gates the WS
	// upgrade on this header — missing or unknown values get rejected
	// with HTTP 400 helper_platform_required / unsupported_platform.
	// v1 enum: {linux, darwin}; runtime.GOOS naturally matches.
	hdr.Set("X-Helper-Platform", runtime.GOOS)

	conn, _, err := websocket.Dial(ctx, dialURL, &websocket.DialOptions{
		HTTPClient:   c.httpClient,
		HTTPHeader:   hdr,
		Subprotocols: []string{HelperWSSubprotocol},
	})
	if err != nil {
		return fmt.Errorf("dial helper ws: %w", err)
	}

	connCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.conn = conn
	c.connCtx = connCtx
	c.connCancel = cancel
	c.missedPings = 0
	c.mu.Unlock()

	go c.readLoop(connCtx, conn)
	go c.pingLoop(connCtx, conn)

	return nil
}

func (c *Client) dialURL() (string, error) {
	u, err := url.Parse(c.serverOrigin)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
		// already a WS URL
	default:
		return "", fmt.Errorf("unsupported origin scheme %q", u.Scheme)
	}
	u.Path = "/ws/helper/" + c.enrollmentID
	return u.String(), nil
}

// Close tears down the connection. Safe to call concurrently with
// Receive — the read loop returns once the conn is closed and Receive
// surfaces context.Canceled (via the connCtx).
func (c *Client) Close() error {
	c.mu.Lock()
	conn := c.conn
	cancel := c.connCancel
	c.conn = nil
	c.connCancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "client closed")
	}
	return nil
}

// readLoop pumps frames from the WS into c.inbound. Decodes the JSON
// envelope, dispatches per `type`. Returns on any read error (which
// triggers the dispatcher's outer reconnect loop).
func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) {
	defer func() {
		// Signal disconnect to Receive by sending a synthetic empty
		// directive — distinguishes "transport down" from "no work
		// yet" cleanly. Receive's caller maps this to the reconnect
		// path.
		select {
		case c.inbound <- inboundFrame{Directive: DirectiveRetry}:
		case <-ctx.Done():
		}
		c.mu.Lock()
		if cancel := c.connCancel; cancel != nil {
			cancel()
		}
		c.mu.Unlock()
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var env struct {
			Type string          `json:"type"`
			Job  json.RawMessage `json:"job,omitempty"`
			Code string          `json:"code,omitempty"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		switch env.Type {
		case "job":
			if len(env.Job) == 0 {
				continue
			}
			var job LeasedJob
			if err := json.Unmarshal(env.Job, &job); err != nil {
				continue
			}
			if job.JobID == "" || job.LeaseToken == "" {
				continue
			}
			select {
			case c.inbound <- inboundFrame{Job: &job}:
			case <-ctx.Done():
				return
			}
		case "directive":
			d := mapDirectiveCode(env.Code)
			select {
			case c.inbound <- inboundFrame{Directive: d}:
			case <-ctx.Done():
				return
			}
			if isStopDirective(d) {
				return
			}
		default:
			// Unknown frame — soft-drop for forward-compat.
		}
	}
}

func mapDirectiveCode(code string) Directive {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "revoked":
		return DirectiveStopRevoked
	case "stale_credential":
		return DirectiveStopStaleCredential
	case "uninstalled":
		return DirectiveStopUninstalled
	case "unauthorized":
		return DirectiveStopUnauthorized
	case "displaced":
		return DirectiveDisplaced
	default:
		return DirectiveRetry
	}
}

// pingLoop sends an application-level WS ping every PingInterval. Three
// consecutive failures tear the connection down (read loop returns).
func (c *Client) pingLoop(ctx context.Context, conn *websocket.Conn) {
	interval := c.pingInterval
	if interval <= 0 {
		interval = PingInterval
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			pingCtx, cancel := context.WithTimeout(ctx, PingTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			c.mu.Lock()
			if err != nil {
				c.missedPings++
				c.logf("borgee-helper: outbound ping failed (%d/%d): %v", c.missedPings, MissedPingsToReconnect, err)
				if c.missedPings >= MissedPingsToReconnect {
					connCancel := c.connCancel
					c.mu.Unlock()
					if connCancel != nil {
						connCancel()
					}
					_ = conn.Close(websocket.StatusAbnormalClosure, "ping timeout")
					return
				}
			} else {
				c.missedPings = 0
			}
			c.mu.Unlock()
			timer.Reset(interval)
		}
	}
}

// Receive blocks until one of:
//   - a job arrives — returns (job, DirectiveProcess, nil)
//   - a directive arrives — returns (nil, directive, nil)
//   - the connection is torn down — returns (nil, DirectiveRetry, error)
//   - ctx is cancelled — returns (nil, "", ctx.Err())
//
// On a stop directive the outer dispatch loop closes the client and
// exits the process (systemd Restart=on-failure handles the rebound).
// On any other directive (Retry / Displaced) the dispatcher tears down
// and reconnects via Dial.
func (c *Client) Receive(ctx context.Context) (*LeasedJob, Directive, error) {
	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	case frame := <-c.inbound:
		if frame.Job != nil {
			return frame.Job, DirectiveProcess, nil
		}
		// Directive frame. DirectiveRetry from the read loop signals
		// transport down (synthetic frame inserted by the deferred
		// teardown in readLoop). Surface as an error so the caller's
		// reconnect logic kicks in.
		if frame.Directive == DirectiveRetry {
			return nil, DirectiveRetry, errors.New("helper ws: transport closed")
		}
		return nil, frame.Directive, nil
	}
}

// Ack sends a `{"type":"ack",...}` frame. Returns a zero-value
// JobState (status="received") on a successful queue. The server's
// shared ProcessHelperAck mutation is fire-and-forget over WS (no
// reply frame); a credential error closes the connection with the
// appropriate directive code.
func (c *Client) Ack(ctx context.Context, enrollmentID, jobID, leaseToken string) (JobState, error) {
	if err := validatePathID(enrollmentID); err != nil {
		return JobState{}, err
	}
	if err := validatePathID(jobID); err != nil {
		return JobState{}, err
	}
	frame := map[string]any{
		"type":        "ack",
		"job_id":      jobID,
		"lease_token": leaseToken,
	}
	if err := c.writeFrame(ctx, frame); err != nil {
		return JobState{}, err
	}
	return JobState{JobID: jobID, Status: "received"}, nil
}

// Result sends a `{"type":"result",...}` frame with the terminal job
// status. Like Ack, fire-and-forget; the server's ProcessHelperResult
// mutation closes the lease and a directive comes back later if the
// credential turned out to be stale.
func (c *Client) Result(ctx context.Context, enrollmentID, jobID string, result ResultRequest) (JobState, error) {
	if err := validatePathID(enrollmentID); err != nil {
		return JobState{}, err
	}
	if err := validatePathID(jobID); err != nil {
		return JobState{}, err
	}
	frame := map[string]any{
		"type":        "result",
		"job_id":      jobID,
		"lease_token": strings.TrimSpace(result.LeaseToken),
		"status":      strings.TrimSpace(result.Status),
	}
	if strings.TrimSpace(result.FailureCode) != "" {
		frame["failure_code"] = strings.TrimSpace(result.FailureCode)
	}
	if strings.TrimSpace(result.FailureMessage) != "" {
		frame["failure_message"] = strings.TrimSpace(result.FailureMessage)
	}
	if len(result.ResultSummary.AuditRefs) > 0 || len(result.ResultSummary.LogRefs) > 0 {
		frame["summary"] = result.ResultSummary
	}
	if err := c.writeFrame(ctx, frame); err != nil {
		return JobState{}, err
	}
	return JobState{JobID: jobID, Status: result.Status, FailureCode: result.FailureCode}, nil
}

// Ping issues one application-level ping (used by tests / external
// liveness probes). The background pingLoop handles production
// cadence; production callers don't need to invoke this directly.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return errors.New("not connected")
	}
	return conn.Ping(ctx)
}

func (c *Client) writeFrame(ctx context.Context, frame map[string]any) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return errors.New("not connected")
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func (c *Client) logf(format string, v ...any) {
	if c.logger != nil {
		c.logger(format, v...)
	}
}

// RunWithReconnect drives the long-lived WS client: dials, hands the
// connection to onReceive (the dispatcher's per-frame body), and on
// any transport tear-down sleeps with exponential backoff before
// redialing. On a stop directive (revoked / stale_credential /
// uninstalled / unauthorized) returns that directive — the caller is
// expected to exit the process so systemd Restart=on-failure handles
// the rebound under StartLimit caps.
func (c *Client) RunWithReconnect(ctx context.Context, onJob func(context.Context, *LeasedJob), onDirective func(context.Context, Directive)) Directive {
	backoff := c.reconnectBase
	if backoff <= 0 {
		backoff = reconnectBackoffBase
	}
	cap := c.reconnectCap
	if cap <= 0 {
		cap = reconnectBackoffCap
	}
	for {
		if err := ctx.Err(); err != nil {
			return ""
		}
		if err := c.Dial(ctx); err != nil {
			c.logf("borgee-helper: outbound dial failed: %v (next in %s)", err, backoff)
			if !sleepJittered(ctx, backoff) {
				return ""
			}
			backoff = nextBackoff(backoff, cap)
			continue
		}
		c.logf("borgee-helper: outbound connected enrollment_id=%s", c.enrollmentID)
		backoff = c.reconnectBase
		if backoff <= 0 {
			backoff = reconnectBackoffBase
		}

		stopDir := c.receiveLoop(ctx, onJob, onDirective)
		_ = c.Close()
		if isStopDirective(stopDir) {
			return stopDir
		}
		if ctx.Err() != nil {
			return ""
		}
		c.logf("borgee-helper: outbound disconnected; reconnecting in %s", backoff)
		if !sleepJittered(ctx, backoff) {
			return ""
		}
		backoff = nextBackoff(backoff, cap)
	}
}

func (c *Client) receiveLoop(ctx context.Context, onJob func(context.Context, *LeasedJob), onDirective func(context.Context, Directive)) Directive {
	for {
		job, dir, err := c.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ""
			}
			return ""
		}
		if isStopDirective(dir) {
			if onDirective != nil {
				onDirective(ctx, dir)
			}
			return dir
		}
		if dir == DirectiveDisplaced {
			if onDirective != nil {
				onDirective(ctx, dir)
			}
			return ""
		}
		if job != nil && dir == DirectiveProcess {
			if onJob != nil {
				onJob(ctx, job)
			}
			continue
		}
	}
}

func isStopDirective(d Directive) bool {
	switch d {
	case DirectiveStopUnauthorized, DirectiveStopStaleCredential, DirectiveStopRevoked, DirectiveStopUninstalled:
		return true
	}
	return false
}

func nextBackoff(current, cap time.Duration) time.Duration {
	next := current * 2
	if next > cap {
		return cap
	}
	if next < current {
		return cap
	}
	return next
}

// sleepJittered sleeps `d` ± 20% jitter, returns false if ctx cancels
// during the wait. Replaces the dispatcher's earlier sleepOrDone now
// that backoff lives in outbound.
func sleepJittered(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	jitter := time.Duration(rand.Int63n(int64(d) / 5))
	if rand.Intn(2) == 0 {
		d = d - jitter
	} else {
		d = d + jitter
	}
	if d <= 0 {
		d = time.Millisecond
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func validatePathID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "://") || strings.ContainsAny(id, "/\\?#") || strings.Contains(id, "..") {
		return fmt.Errorf("unsafe helper identifier")
	}
	return nil
}
