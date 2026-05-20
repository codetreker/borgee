package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// runHelperWSTestServer spins up an httptest server with the helper WS
// endpoint at /ws/helper/{enrollmentId}. The handler validates the
// Bearer credential + X-Helper-Device-Id headers (via the supplied
// auth function), accepts the upgrade, and runs onConn against the
// upgraded conn — tests use onConn to inject push frames and assert
// inbound frames.
type helperWSTestServer struct {
	t        *testing.T
	srv      *httptest.Server
	mu       sync.Mutex
	gotFrames []string
}

func newHelperWSTestServer(t *testing.T, onConn func(ctx context.Context, conn *websocket.Conn, srv *helperWSTestServer)) *helperWSTestServer {
	t.Helper()
	s := &helperWSTestServer{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer helper-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("X-Helper-Device-Id") != "device-1" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{HelperWSSubprotocol},
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("accept failed: %v", err)
			return
		}
		ctx := r.Context()
		if onConn != nil {
			onConn(ctx, conn, s)
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func newTestClient(t *testing.T, srv *helperWSTestServer, enrollmentID string, extra ...ClientOption) *Client {
	t.Helper()
	opts := []ClientOption{WithHTTPClient(srv.srv.Client())}
	opts = append(opts, extra...)
	c, err := NewClient(
		PreparedConfig{Enabled: true, ServerOrigin: srv.srv.URL},
		StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"},
		opts...,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetEnrollmentID(enrollmentID)
	return c
}

func TestClient_DialReceivesPushedJob(t *testing.T) {
	srv := newHelperWSTestServer(t, func(ctx context.Context, conn *websocket.Conn, _ *helperWSTestServer) {
		// Push one job frame.
		job := map[string]any{
			"type": "job",
			"job": map[string]any{
				"job_id":           "job-1",
				"enrollment_id":    "enroll-1",
				"job_type":         "openclaw.configure_agent",
				"schema_version":   1,
				"payload":          map[string]any{"agent_id": "agent-1"},
				"manifest_digest":  "sha256:manifest",
				"lease_token":      "lease-token",
				"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
				"attempt":          1,
			},
		}
		data, _ := json.Marshal(job)
		_ = conn.Write(ctx, websocket.MessageText, data)
		// Hold the connection open until the test closes it.
		time.Sleep(150 * time.Millisecond)
	})

	c := newTestClient(t, srv, "enroll-1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Dial(ctx); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	job, dir, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if dir != DirectiveProcess {
		t.Fatalf("dir=%s want process", dir)
	}
	if job == nil || job.JobID != "job-1" || job.LeaseToken != "lease-token" {
		t.Fatalf("job=%+v", job)
	}
}

func TestClient_AckResultRoundTripsAsFrames(t *testing.T) {
	var (
		ackCh    = make(chan map[string]any, 1)
		resultCh = make(chan map[string]any, 1)
	)
	srv := newHelperWSTestServer(t, func(ctx context.Context, conn *websocket.Conn, _ *helperWSTestServer) {
		// Push one job + read 2 frames (ack, result).
		job := map[string]any{
			"type": "job",
			"job": map[string]any{
				"job_id":           "job-1",
				"enrollment_id":    "enroll-1",
				"job_type":         "openclaw.configure_agent",
				"schema_version":   1,
				"payload":          map[string]any{},
				"manifest_digest":  "sha256:manifest",
				"lease_token":      "lease-token",
				"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
				"attempt":          1,
			},
		}
		data, _ := json.Marshal(job)
		_ = conn.Write(ctx, websocket.MessageText, data)

		for i := 0; i < 2; i++ {
			_, buf, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			if err := json.Unmarshal(buf, &m); err != nil {
				continue
			}
			switch m["type"] {
			case "ack":
				ackCh <- m
			case "result":
				resultCh <- m
			}
		}
		time.Sleep(50 * time.Millisecond)
	})

	c := newTestClient(t, srv, "enroll-1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Dial(ctx); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if _, _, err := c.Receive(ctx); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if _, err := c.Ack(ctx, "enroll-1", "job-1", "lease-token"); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if _, err := c.Result(ctx, "enroll-1", "job-1", ResultRequest{
		LeaseToken:    "lease-token",
		Status:        "succeeded",
		ResultSummary: ResultSummary{AuditRefs: []string{"audit-1"}},
	}); err != nil {
		t.Fatalf("Result: %v", err)
	}

	select {
	case ack := <-ackCh:
		if ack["job_id"] != "job-1" || ack["lease_token"] != "lease-token" {
			t.Fatalf("ack frame bad: %v", ack)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ack frame timeout")
	}
	select {
	case res := <-resultCh:
		if res["job_id"] != "job-1" || res["status"] != "succeeded" {
			t.Fatalf("result frame bad: %v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("result frame timeout")
	}
}

func TestClient_DirectiveStopRevoked(t *testing.T) {
	srv := newHelperWSTestServer(t, func(ctx context.Context, conn *websocket.Conn, _ *helperWSTestServer) {
		data, _ := json.Marshal(map[string]any{"type": "directive", "code": "revoked"})
		_ = conn.Write(ctx, websocket.MessageText, data)
		time.Sleep(50 * time.Millisecond)
	})

	c := newTestClient(t, srv, "enroll-1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Dial(ctx); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	_, dir, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if dir != DirectiveStopRevoked {
		t.Fatalf("directive=%s want stop_revoked", dir)
	}
}

func TestClient_RejectsUnauthorizedHandshake(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := NewClient(
		PreparedConfig{Enabled: true, ServerOrigin: srv.URL},
		StaticCredentialSource{Credential: "bad", HelperDeviceID: "device-1"},
		WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetEnrollmentID("enroll-1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Dial(ctx); err == nil {
		t.Fatal("Dial should fail on 401")
	}
}

func TestClient_RejectsUnsafeIdentifiers(t *testing.T) {
	srv := newHelperWSTestServer(t, func(ctx context.Context, conn *websocket.Conn, _ *helperWSTestServer) {
		time.Sleep(20 * time.Millisecond)
	})
	c := newTestClient(t, srv, "enroll-1")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := c.Dial(ctx); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	for _, id := range []string{"https://evil.example/x", "../other", "job/with/slash"} {
		if _, err := c.Ack(ctx, id, "job-1", "lease-token"); err == nil {
			t.Fatalf("Ack accepted unsafe id %q", id)
		}
	}
}

func TestClient_PingPongHeartbeat(t *testing.T) {
	var pingCount int
	var mu sync.Mutex
	srv := newHelperWSTestServerCustom(t, &http.Header{}, func(ctx context.Context, conn *websocket.Conn) {
		// coder/websocket auto-pong response is internal — we just
		// hold the connection open and count via OnPing on server.
		_, _, _ = conn.Read(ctx)
	}, func() {
		mu.Lock()
		pingCount++
		mu.Unlock()
	})

	c := newTestClient(t, srv, "enroll-1", WithPingInterval(40*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := c.Dial(ctx); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// Wait for at least one ping to arrive.
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := pingCount
		mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no ping observed within 800ms")
}

// newHelperWSTestServerCustom is the ping-test variant that exposes
// the OnPingReceived hook.
func newHelperWSTestServerCustom(t *testing.T, _ *http.Header, onConn func(ctx context.Context, conn *websocket.Conn), onPing func()) *helperWSTestServer {
	t.Helper()
	s := &helperWSTestServer{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{HelperWSSubprotocol},
			InsecureSkipVerify: true,
			OnPingReceived: func(ctx context.Context, payload []byte) bool {
				if onPing != nil {
					onPing()
				}
				return true
			},
		})
		if err != nil {
			return
		}
		ctx := r.Context()
		if onConn != nil {
			onConn(ctx, conn)
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func TestClient_ReconnectsAfterServerClose(t *testing.T) {
	var dialCount int
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		dialCount++
		first := dialCount == 1
		mu.Unlock()
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{HelperWSSubprotocol},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		if first {
			_ = conn.Close(websocket.StatusGoingAway, "shutdown")
			return
		}
		// Second connection — push a job.
		data, _ := json.Marshal(map[string]any{
			"type": "job",
			"job": map[string]any{
				"job_id":           "job-2",
				"enrollment_id":    "enroll-1",
				"job_type":         "openclaw.configure_agent",
				"schema_version":   1,
				"payload":          map[string]any{},
				"manifest_digest":  "sha256:m",
				"lease_token":      "lease-2",
				"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
				"attempt":          1,
			},
		})
		_ = conn.Write(r.Context(), websocket.MessageText, data)
		time.Sleep(150 * time.Millisecond)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := NewClient(
		PreparedConfig{Enabled: true, ServerOrigin: srv.URL},
		StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"},
		WithHTTPClient(srv.Client()),
		WithReconnectBackoff(20*time.Millisecond, 100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetEnrollmentID("enroll-1")

	gotJob := make(chan *LeasedJob, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		_ = c.RunWithReconnect(ctx, func(_ context.Context, j *LeasedJob) {
			select {
			case gotJob <- j:
			default:
			}
		}, nil)
	}()
	select {
	case j := <-gotJob:
		if j.JobID != "job-2" {
			t.Fatalf("job_id=%s want job-2", j.JobID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected reconnect-then-job, timed out")
	}
}

// TestClient_PrereqAcceptsWSS asserts the prereq validator accepts
// wss:// (the production daemon target). Backed by the deriveWSOrigin
// install-time path.
func TestClient_PrereqAcceptsWSS(t *testing.T) {
	for _, origin := range []string{"wss://example.com", "https://example.com"} {
		prep, err := ValidateAndPrepare(PrereqConfig{
			ServerOrigin:    origin,
			AllowedOrigins:  origin,
			QueueStateDir:   t.TempDir(),
			StatusStateDir:  t.TempDir(),
			AuditHandoffDir: t.TempDir(),
		}, ValidationOptions{
			AllowedStateRoots: []string{"/"},
		})
		if err != nil {
			t.Fatalf("ValidateAndPrepare(%q): %v", origin, err)
		}
		if !prep.Enabled {
			t.Fatalf("Enabled=false for %q", origin)
		}
		if !strings.HasPrefix(prep.ServerOrigin, "wss://") && !strings.HasPrefix(prep.ServerOrigin, "https://") {
			t.Fatalf("ServerOrigin=%q", prep.ServerOrigin)
		}
	}
}
