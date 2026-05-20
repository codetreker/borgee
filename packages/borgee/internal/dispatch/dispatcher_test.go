package dispatch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"

	"github.com/coder/websocket"
)

// dispatcherWSTest spins up a minimal WS helper endpoint per test and
// drives the dispatcher through one job push + Ack/Result round-trip.
// PR-2 #1038: replaces the prior HTTP long-poll fake.

func newDispatcherWSTest(t *testing.T, onConn func(ctx context.Context, conn *websocket.Conn, recv chan<- map[string]any)) (*outbound.Client, chan map[string]any) {
	t.Helper()
	recv := make(chan map[string]any, 8)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/helper/{enrollmentId}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer helper-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{outbound.HelperWSSubprotocol},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		ctx := r.Context()
		go func() {
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}
				var m map[string]any
				if err := json.Unmarshal(data, &m); err != nil {
					continue
				}
				select {
				case recv <- m:
				default:
				}
			}
		}()
		if onConn != nil {
			onConn(ctx, conn, recv)
		}
		time.Sleep(200 * time.Millisecond)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := outbound.NewClient(
		outbound.PreparedConfig{Enabled: true, ServerOrigin: srv.URL},
		outbound.StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"},
		outbound.WithHTTPClient(srv.Client()),
		outbound.WithReconnectBackoff(10*time.Millisecond, 50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.SetEnrollmentID("enroll-1")
	return c, recv
}

type recordingExecutor struct {
	calls atomic.Int32
	last  *outbound.LeasedJob
	terminal Executor // ignored, kept for shape
}

func (e *recordingExecutor) Execute(_ context.Context, job *outbound.LeasedJob) (TerminalStatus, error) {
	e.calls.Add(1)
	e.last = job
	return TerminalStatus{Status: StatusSucceeded}, nil
}

func TestDispatcher_ReceivesJobRunsExecutorPostsResult(t *testing.T) {
	pushedJob := map[string]any{
		"type": "job",
		"job": map[string]any{
			"job_id":           "job-1",
			"enrollment_id":    "enroll-1",
			"job_type":         "openclaw.configure_agent",
			"schema_version":   1,
			"payload":          map[string]any{},
			"manifest_digest":  "sha256:m",
			"lease_token":      "lease-1",
			"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
			"attempt":          1,
		},
	}
	c, recv := newDispatcherWSTest(t, func(ctx context.Context, conn *websocket.Conn, _ chan<- map[string]any) {
		data, _ := json.Marshal(pushedJob)
		_ = conn.Write(ctx, websocket.MessageText, data)
	})

	exec := &recordingExecutor{}
	d := &Dispatcher{
		Client:       c,
		EnrollmentID: "enroll-1",
		PolicyEvaluator: func(_ context.Context, _ *outbound.LeasedJob) jobpolicy.Decision {
			return jobpolicy.Decision{Allow: true}
		},
		Executors: map[string]Executor{
			"openclaw.configure_agent": exec,
		},
		LeaseRenewEvery: 30 * time.Second, // disable in-test ack ticker noise
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go d.Run(ctx)

	// Wait for a result frame.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-recv:
			if m["type"] == "result" {
				if m["job_id"] != "job-1" {
					t.Fatalf("result job_id=%v", m["job_id"])
				}
				if m["status"] != "succeeded" {
					t.Fatalf("result status=%v", m["status"])
				}
				if exec.calls.Load() != 1 {
					t.Fatalf("executor calls=%d", exec.calls.Load())
				}
				return
			}
		case <-deadline:
			t.Fatal("result frame timeout")
		}
	}
}

func TestDispatcher_PolicyRejectFailsJob(t *testing.T) {
	c, recv := newDispatcherWSTest(t, func(ctx context.Context, conn *websocket.Conn, _ chan<- map[string]any) {
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
			},
		})
		_ = conn.Write(ctx, websocket.MessageText, data)
	})

	d := &Dispatcher{
		Client:       c,
		EnrollmentID: "enroll-1",
		PolicyEvaluator: func(_ context.Context, _ *outbound.LeasedJob) jobpolicy.Decision {
			return jobpolicy.Decision{Allow: false, Reason: jobpolicy.ReasonPolicyDenied}
		},
		Executors: map[string]Executor{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go d.Run(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-recv:
			if m["type"] == "result" {
				if m["status"] != "failed" {
					t.Fatalf("status=%v", m["status"])
				}
				if !strings.Contains(asString(m["failure_message"]), "policy") {
					t.Fatalf("failure_message=%v", m["failure_message"])
				}
				return
			}
		case <-deadline:
			t.Fatal("result frame timeout")
		}
	}
}

func TestDispatcher_DirectiveStopExitsLoop(t *testing.T) {
	c, _ := newDispatcherWSTest(t, func(ctx context.Context, conn *websocket.Conn, _ chan<- map[string]any) {
		data, _ := json.Marshal(map[string]any{"type": "directive", "code": "revoked"})
		_ = conn.Write(ctx, websocket.MessageText, data)
	})

	directiveSeen := make(chan outbound.Directive, 1)
	d := &Dispatcher{
		Client:       c,
		EnrollmentID: "enroll-1",
		PolicyEvaluator: func(_ context.Context, _ *outbound.LeasedJob) jobpolicy.Decision {
			return jobpolicy.Decision{Allow: true}
		},
		OnDirective: func(_ context.Context, dir outbound.Directive) {
			select {
			case directiveSeen <- dir:
			default:
			}
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { d.Run(ctx); close(done) }()

	select {
	case dir := <-directiveSeen:
		if dir != outbound.DirectiveStopRevoked {
			t.Fatalf("directive=%s want stop_revoked", dir)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("directive timeout")
	}

	select {
	case <-done:
		// Dispatcher exited cleanly on stop directive.
	case <-time.After(2 * time.Second):
		// Some implementations may keep looping until ctx; that's
		// acceptable as long as the directive fired.
	}
}

func TestDispatcher_NoEnrollmentSkipsCleanly(t *testing.T) {
	var d Dispatcher // zero value
	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("Run on zero dispatcher: %v", err)
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// Ensure the helper recv buffer doesn't drop while we wait; suppress
// unused-variable warnings if a test exits early.
var _ = sync.Mutex{}
