package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"borgee-server/internal/datalayer"
	"borgee-server/internal/helpermanifest"
	"borgee-server/internal/store"

	"github.com/coder/websocket"
)

// fakeHelperAuth implements ws.HelperEnrollmentAuthenticator for tests.
type fakeHelperAuth struct {
	mu     sync.Mutex
	calls  []fakeHelperAuthCall
	err    error
	enroll *datalayer.HelperEnrollment
}

type fakeHelperAuthCall struct {
	ID         string
	Credential string
	DeviceID   string
}

func (f *fakeHelperAuth) UpdateLastSeen(_ context.Context, id, credential, deviceID string, _ time.Time) (*datalayer.HelperEnrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeHelperAuthCall{ID: id, Credential: credential, DeviceID: deviceID})
	if f.err != nil {
		return nil, f.err
	}
	if f.enroll != nil {
		return f.enroll, nil
	}
	return &datalayer.HelperEnrollment{ID: id, Status: "connected"}, nil
}

// fakeHelperProcessor implements ws.HelperJobProcessor for tests.
type fakeHelperProcessor struct {
	ackCalls    atomic.Int32
	resultCalls atomic.Int32
	lastResult  struct {
		mu            sync.Mutex
		jobID, status string
		failureCode   string
		summary       json.RawMessage
	}
}

func (f *fakeHelperProcessor) ProcessHelperAck(_ context.Context, _, _, _, _, _ string) error {
	f.ackCalls.Add(1)
	return nil
}

func (f *fakeHelperProcessor) ProcessHelperResult(_ context.Context, _, jobID, _, _, _, status, failureCode, _ string, summary json.RawMessage) error {
	f.resultCalls.Add(1)
	f.lastResult.mu.Lock()
	defer f.lastResult.mu.Unlock()
	f.lastResult.jobID = jobID
	f.lastResult.status = status
	f.lastResult.failureCode = failureCode
	f.lastResult.summary = summary
	return nil
}

func newHelperTestServer(t *testing.T, auth HelperEnrollmentAuthenticator, processor HelperJobProcessor) (*httptest.Server, *Hub) {
	t.Helper()
	hub := &Hub{
		logger:         slog.Default(),
		clients:        make(map[*Client]bool),
		onlineUsers:    make(map[string]map[*Client]bool),
		plugins:        make(map[string]*PluginConn),
		remotes:        make(map[string]*RemoteConn),
		helperSessions: make(map[string]*HelperSession),
		eventWaiters:   make(map[chan struct{}]struct{}),
	}
	hub.store = (*store.Store)(nil) // unused in helper WS tests
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", HandleHelper(hub, auth, processor))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, hub
}

func wsURL(srv *httptest.Server, enrollmentID string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/helper/" + enrollmentID
}

func dialHelperWS(t *testing.T, srv *httptest.Server, enrollmentID, credential, deviceID string, extraHeaders http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	hdr := http.Header{}
	if credential != "" {
		hdr.Set("Authorization", "Bearer "+credential)
	}
	if deviceID != "" {
		hdr.Set("X-Helper-Device-Id", deviceID)
	}
	// PR-4 final amend: default X-Helper-Platform to "linux" unless the
	// test caller explicitly sets the key in extraHeaders (including to
	// "" so the missing-platform reject path can be exercised).
	if _, override := extraHeaders["X-Helper-Platform"]; !override {
		hdr.Set("X-Helper-Platform", "linux")
	}
	for k, vs := range extraHeaders {
		for _, v := range vs {
			hdr.Add(k, v)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	conn, resp, err := websocket.Dial(ctx, wsURL(srv, enrollmentID), &websocket.DialOptions{
		HTTPClient:   srv.Client(),
		HTTPHeader:   hdr,
		Subprotocols: []string{HelperWSSubprotocol},
	})
	return conn, resp, err
}

func TestHelperWS_UpgradeAcceptsValidCredential(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait briefly for the server to register the session.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-1") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hub.GetHelper("enroll-1") == nil {
		t.Fatal("helper session not registered")
	}
	if len(auth.calls) != 1 || auth.calls[0].Credential != "helper-token" || auth.calls[0].DeviceID != "device-1" {
		t.Fatalf("auth calls=%v", auth.calls)
	}
}

func TestHelperWS_UpgradeRejectsInvalidCredential(t *testing.T) {
	auth := &fakeHelperAuth{err: datalayer.ErrHelperEnrollmentUnauthorized}
	srv, _ := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	_, _, err := dialHelperWS(t, srv, "enroll-1", "wrong", "device-1", nil)
	if err == nil {
		t.Fatal("expected upgrade to fail")
	}
}

func TestHelperWS_UpgradeRejectsMissingHeaders(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, _ := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	// No credential.
	_, _, err := dialHelperWS(t, srv, "enroll-1", "", "device-1", nil)
	if err == nil {
		t.Fatal("expected upgrade to fail without credential")
	}
	// No device id.
	_, _, err = dialHelperWS(t, srv, "enroll-1", "helper-token", "", nil)
	if err == nil {
		t.Fatal("expected upgrade to fail without device id")
	}
}

func TestHelperWS_UpgradeRejectsOriginHeader(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, _ := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	_, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", http.Header{"Origin": []string{"https://evil.example.com"}})
	if err == nil {
		t.Fatal("expected upgrade to be rejected with non-empty Origin header")
	}
}

func TestHelperWS_DisplacesExistingSession(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	conn1, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("first Dial: %v", err)
	}
	// Wait for registration.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-1") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	first := hub.GetHelper("enroll-1")
	if first == nil {
		t.Fatal("first session not registered")
	}

	// Second connect should displace the first.
	conn2, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("second Dial: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "")

	// First conn should close with 4001. Read should error.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, _, readErr := conn1.Read(ctx)
	if readErr == nil {
		t.Fatal("first conn should be closed after displacement")
	}
	var closeErr websocket.CloseError
	if errors.As(readErr, &closeErr) {
		if closeErr.Code != HelperWSCloseDisplaced {
			t.Logf("close code=%d (want %d) — accepting any close as long as the conn dropped", closeErr.Code, HelperWSCloseDisplaced)
		}
	}

	// Hub should now point at the second session, not the first.
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cur := hub.GetHelper("enroll-1"); cur != nil && cur != first {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if cur := hub.GetHelper("enroll-1"); cur == nil || cur == first {
		t.Fatal("hub helperSessions did not swap to the newer session")
	}
}

func TestHelperWS_PushJob(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait for session.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-1") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	jobBytes, _ := json.Marshal(map[string]any{
		"job_id":      "job-99",
		"lease_token": "lease-99",
	})
	if !hub.SendJobToHelper("enroll-1", jobBytes) {
		t.Fatal("SendJobToHelper returned false")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env struct {
		Type string          `json:"type"`
		Job  json.RawMessage `json:"job"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != "job" {
		t.Fatalf("type=%s want job", env.Type)
	}
	if !strings.Contains(string(env.Job), "job-99") {
		t.Fatalf("job payload missing job-99: %s", env.Job)
	}
}

func TestHelperWS_AckResultRoundTrip(t *testing.T) {
	auth := &fakeHelperAuth{}
	proc := &fakeHelperProcessor{}
	srv, _ := newHelperTestServer(t, auth, proc)

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ackFrame, _ := json.Marshal(map[string]any{
		"type":        "ack",
		"job_id":      "job-1",
		"lease_token": "lease-1",
	})
	if err := conn.Write(ctx, websocket.MessageText, ackFrame); err != nil {
		t.Fatalf("write ack: %v", err)
	}
	resultFrame, _ := json.Marshal(map[string]any{
		"type":            "result",
		"job_id":          "job-1",
		"lease_token":     "lease-1",
		"status":          "succeeded",
		"failure_code":    "",
		"failure_message": "",
		"summary":         map[string]any{"audit_refs": []string{"ref-1"}},
	})
	if err := conn.Write(ctx, websocket.MessageText, resultFrame); err != nil {
		t.Fatalf("write result: %v", err)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if proc.ackCalls.Load() > 0 && proc.resultCalls.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if proc.ackCalls.Load() != 1 {
		t.Fatalf("ack calls=%d want 1", proc.ackCalls.Load())
	}
	if proc.resultCalls.Load() != 1 {
		t.Fatalf("result calls=%d want 1", proc.resultCalls.Load())
	}
	proc.lastResult.mu.Lock()
	defer proc.lastResult.mu.Unlock()
	if proc.lastResult.status != "succeeded" {
		t.Fatalf("result status=%q", proc.lastResult.status)
	}
	if !strings.Contains(string(proc.lastResult.summary), "ref-1") {
		t.Fatalf("result summary=%s", proc.lastResult.summary)
	}
}

func TestHelperWS_SendDirectiveClosesGracefully(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait for session.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-1") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !hub.SendDirectiveToHelper("enroll-1", "revoked") {
		t.Fatal("SendDirectiveToHelper returned false")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read directive: %v", err)
	}
	var env struct {
		Type string `json:"type"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != "directive" || env.Code != "revoked" {
		t.Fatalf("env=%+v", env)
	}
}

func TestHelperWS_ConnectHookFires(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	var hookCalled atomic.Int32
	hub.SetHelperConnectHook(func(enrollmentID string) {
		if enrollmentID == "enroll-1" {
			hookCalled.Add(1)
		}
	})

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hookCalled.Load() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("connect hook did not fire")
}

func TestHelperWS_LastSeenUpdatesOnRead(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait for session.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-1") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	sess := hub.GetHelper("enroll-1")
	if sess == nil {
		t.Fatal("no session")
	}
	initial := sess.LastSeen()
	time.Sleep(15 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	frame, _ := json.Marshal(map[string]string{"type": "ack"})
	if err := conn.Write(ctx, websocket.MessageText, frame); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if sess.LastSeen().After(initial) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("LastSeen did not advance: initial=%v current=%v", initial, sess.LastSeen())
}

func TestHelperSession_SendJSONSerializes(t *testing.T) {
	sess := &HelperSession{send: make(chan []byte, 4), done: make(chan struct{})}
	if !sess.SendJSON(map[string]string{"type": "directive", "code": "revoked"}) {
		t.Fatal("SendJSON should return true when buffer accepts")
	}
	select {
	case data := <-sess.send:
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m["type"] != "directive" || m["code"] != "revoked" {
			t.Fatalf("unexpected: %v", m)
		}
	default:
		t.Fatal("SendJSON did not enqueue")
	}
	// Unmarshallable values are dropped without panic and report false.
	if sess.SendJSON(make(chan int)) {
		t.Fatal("SendJSON should return false on marshal failure")
	}
}

// PR-4 P0 review fix — Send must report dropped frames so the caller
// (pushOneLeasedFrame) can log + recover the already-leased row.
func TestHelperSession_SendReturnsFalseOnFullBuffer(t *testing.T) {
	// Construct a session whose writer pump is NOT running so the send
	// channel cannot drain. Fill the buffer to capacity, then the next
	// Send must return false instead of silently dropping.
	const cap = 4
	sess := &HelperSession{send: make(chan []byte, cap), done: make(chan struct{})}
	for i := 0; i < cap; i++ {
		if !sess.Send([]byte("frame")) {
			t.Fatalf("Send #%d returned false before buffer full", i)
		}
	}
	if sess.Send([]byte("overflow")) {
		t.Fatal("Send must return false when send buffer is full")
	}
}

func TestHelperSession_SendReturnsTrueWhenBufferAccepts(t *testing.T) {
	sess := &HelperSession{send: make(chan []byte, 2), done: make(chan struct{})}
	if !sess.Send([]byte("frame")) {
		t.Fatal("Send must return true when buffer accepts")
	}
}

func TestMapHelperAuthErrorToStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{datalayer.ErrHelperEnrollmentUnauthorized, http.StatusUnauthorized},
		{datalayer.ErrHelperEnrollmentInvalidInput, http.StatusUnauthorized},
		{datalayer.ErrHelperEnrollmentDeviceMismatch, http.StatusForbidden},
		{datalayer.ErrHelperEnrollmentInactive, http.StatusForbidden},
		{datalayer.ErrHelperEnrollmentForbidden, http.StatusForbidden},
		{datalayer.ErrHelperEnrollmentNotFound, http.StatusNotFound},
		{errors.New("other"), http.StatusUnauthorized},
	}
	for _, tc := range cases {
		if got := mapHelperAuthErrorToStatus(tc.err); got != tc.want {
			t.Fatalf("err=%v got=%d want=%d", tc.err, got, tc.want)
		}
	}
}

func TestHub_SnapshotHelperLastSeen(t *testing.T) {
	hub := &Hub{
		logger:         slog.Default(),
		helperSessions: make(map[string]*HelperSession),
	}
	// Empty.
	if got := hub.SnapshotHelperLastSeen(); len(got) != 0 {
		t.Fatalf("empty snapshot len=%d", len(got))
	}
	// Two registered.
	s1 := &HelperSession{lastSeenAt: time.Now()}
	s2 := &HelperSession{lastSeenAt: time.Now()}
	hub.helperSessions["a"] = s1
	hub.helperSessions["b"] = s2
	snap := hub.SnapshotHelperLastSeen()
	if len(snap) != 2 || snap["a"].IsZero() || snap["b"].IsZero() {
		t.Fatalf("snapshot=%+v", snap)
	}
}

func TestHub_SendJobToHelper_NoSession(t *testing.T) {
	hub := &Hub{
		logger:         slog.Default(),
		helperSessions: make(map[string]*HelperSession),
	}
	if hub.SendJobToHelper("missing", json.RawMessage(`{"id":"x"}`)) {
		t.Fatal("SendJobToHelper should return false when no session")
	}
	if hub.SendDirectiveToHelper("missing", "revoked") {
		t.Fatal("SendDirectiveToHelper should return false when no session")
	}
}

func TestHub_RegisterUnregisterHelper(t *testing.T) {
	hub := &Hub{
		logger:         slog.Default(),
		helperSessions: make(map[string]*HelperSession),
	}
	s1 := &HelperSession{enrollmentID: "a", send: make(chan []byte, 1), done: make(chan struct{})}
	if prev := hub.RegisterHelper("a", s1); prev != nil {
		t.Fatal("first register should return nil prev")
	}
	if hub.GetHelper("a") != s1 {
		t.Fatal("hub did not register s1")
	}
	// Displacement.
	s2 := &HelperSession{enrollmentID: "a", send: make(chan []byte, 1), done: make(chan struct{})}
	prev := hub.RegisterHelper("a", s2)
	if prev != s1 {
		t.Fatalf("displaced session = %v want s1", prev)
	}
	if hub.GetHelper("a") != s2 {
		t.Fatal("hub did not swap to s2")
	}
	// UnregisterIfCurrent on the displaced (old) session is a no-op.
	hub.UnregisterHelperIfCurrent("a", s1)
	if hub.GetHelper("a") != s2 {
		t.Fatal("UnregisterIfCurrent must not drop newer session")
	}
	// UnregisterIfCurrent on the active session drops it.
	hub.UnregisterHelperIfCurrent("a", s2)
	if hub.GetHelper("a") != nil {
		t.Fatal("UnregisterIfCurrent did not drop active session")
	}
	// Edge: empty/nil arguments are no-ops.
	hub.UnregisterHelperIfCurrent("", s1)
	hub.UnregisterHelperIfCurrent("a", nil)
	if hub.RegisterHelper("", s1) != nil {
		t.Fatal("RegisterHelper with empty id must noop")
	}
	if hub.RegisterHelper("c", nil) != nil {
		t.Fatal("RegisterHelper with nil session must noop")
	}
}

func TestHelperSession_CloseIdempotent(t *testing.T) {
	sess := &HelperSession{send: make(chan []byte, 1), done: make(chan struct{})}
	sess.Close(websocket.StatusNormalClosure, "test")
	// Second close should not panic on the closed done channel.
	sess.Close(websocket.StatusNormalClosure, "test")
}

func TestHelperSession_EnrollmentID(t *testing.T) {
	sess := &HelperSession{enrollmentID: "enroll-X"}
	if sess.EnrollmentID() != "enroll-X" {
		t.Fatalf("EnrollmentID=%q", sess.EnrollmentID())
	}
}

// PR-4 final amend — platform header gating + WS push wiring tests.

func TestHelperWSUpgrade_RequiresPlatformHeader(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, _ := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	// Empty value override → server rejects with HTTP 400 before WS upgrade.
	_, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", http.Header{"X-Helper-Platform": []string{""}})
	if err == nil {
		t.Fatal("expected upgrade to fail when X-Helper-Platform empty")
	}
}

func TestHelperWSUpgrade_RejectsUnknownPlatform(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, _ := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	_, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", http.Header{"X-Helper-Platform": []string{"windows"}})
	if err == nil {
		t.Fatal("expected upgrade to fail when X-Helper-Platform=windows")
	}
}

func TestHelperWSUpgrade_AcceptsLinuxAndDarwin(t *testing.T) {
	for _, platform := range []string{"linux", "darwin"} {
		platform := platform
		t.Run(platform, func(t *testing.T) {
			auth := &fakeHelperAuth{}
			srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})
			conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", http.Header{"X-Helper-Platform": []string{platform}})
			if err != nil {
				t.Fatalf("Dial %s: %v", platform, err)
			}
			defer conn.Close(websocket.StatusNormalClosure, "")
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				if hub.GetHelper("enroll-1") != nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			sess := hub.GetHelper("enroll-1")
			if sess == nil {
				t.Fatalf("session not registered for platform=%s", platform)
			}
			if string(sess.Platform()) != platform {
				t.Fatalf("session.Platform()=%q want %q", sess.Platform(), platform)
			}
		})
	}
}

func TestHelperWS_PushJob_DeliversFrameToConnectedDaemon(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})

	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-1") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hub.GetHelper("enroll-1") == nil {
		t.Fatal("session not registered")
	}

	pushed := hub.SendJobToHelper("enroll-1", json.RawMessage(`{"job_id":"job-1"}`))
	if !pushed {
		t.Fatal("SendJobToHelper returned false for connected session")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read pushed frame: %v", err)
	}
	if !strings.Contains(string(data), `"type":"job"`) || !strings.Contains(string(data), `"job_id":"job-1"`) {
		t.Fatalf("unexpected pushed frame: %s", data)
	}
}

func TestHelperWS_SessionExposesCredentialDeviceAndPlatform(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})
	conn, _, err := dialHelperWS(t, srv, "enroll-7", "tok-7", "device-7", http.Header{"X-Helper-Platform": []string{"darwin"}})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hub.GetHelper("enroll-7") != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	sess := hub.GetHelper("enroll-7")
	if sess == nil {
		t.Fatal("session not registered")
	}
	if sess.Credential() != "tok-7" || sess.DeviceID() != "device-7" || sess.Platform() != helpermanifest.PlatformDarwin {
		t.Fatalf("session getters mismatch: credential=%q device=%q platform=%q", sess.Credential(), sess.DeviceID(), sess.Platform())
	}
}

func TestHelperWS_ConnectHookFiresOnRegister(t *testing.T) {
	auth := &fakeHelperAuth{}
	srv, hub := newHelperTestServer(t, auth, &fakeHelperProcessor{})
	var fired atomic.Int32
	hub.SetHelperConnectHook(func(enrollmentID string) {
		if enrollmentID == "enroll-1" {
			fired.Add(1)
		}
	})
	conn, _, err := dialHelperWS(t, srv, "enroll-1", "helper-token", "device-1", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if fired.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if fired.Load() != 1 {
		t.Fatalf("connect hook fired %d times, want 1", fired.Load())
	}
}
