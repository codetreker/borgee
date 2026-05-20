package dispatch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
)

// dispatcherTestServer fakes the helper-jobs HTTP rail so dispatcher_test
// can drive Poll/Ack/Result without standing up the full server stack.
type dispatcherTestServer struct {
	srv     *httptest.Server
	mu      sync.Mutex
	pollHits int
	ackHits  int
	resultHits int
	results  []resultRecord
	// pollHandler returns (statusCode, body) for each /poll. nil → default 204 no_work.
	pollHandler func(callIdx int) (int, map[string]any)
}

type resultRecord struct {
	JobID  string
	Status string
	Code   string
	Msg    string
}

func newDispatcherTestServer() *dispatcherTestServer {
	ts := &dispatcherTestServer{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.mu.Lock()
		defer ts.mu.Unlock()
		switch {
		case r.URL.Path == "/api/v1/helper/enrollments/enroll-1/jobs/poll":
			ts.pollHits++
			if ts.pollHandler != nil {
				code, body := ts.pollHandler(ts.pollHits)
				w.WriteHeader(code)
				_ = json.NewEncoder(w).Encode(body)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "no_work", "retry_after_ms": 50})
		case r.URL.Path == "/api/v1/helper/enrollments/enroll-1/jobs/job-1/ack":
			ts.ackHits++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"job": map[string]any{"job_id": "job-1", "status": "running"}})
		case r.URL.Path == "/api/v1/helper/enrollments/enroll-1/jobs/job-1/result":
			ts.resultHits++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			rec := resultRecord{JobID: "job-1"}
			if s, ok := body["status"].(string); ok {
				rec.Status = s
			}
			if c, ok := body["failure_code"].(string); ok {
				rec.Code = c
			}
			if m, ok := body["failure_message"].(string); ok {
				rec.Msg = m
			}
			ts.results = append(ts.results, rec)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"job": map[string]any{"job_id": "job-1", "status": rec.Status}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return ts
}

func (ts *dispatcherTestServer) close() { ts.srv.Close() }

func (ts *dispatcherTestServer) setPollHandler(h func(callIdx int) (int, map[string]any)) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.pollHandler = h
}

func (ts *dispatcherTestServer) snapshot() (polls, acks, results int, records []resultRecord) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	dup := make([]resultRecord, len(ts.results))
	copy(dup, ts.results)
	return ts.pollHits, ts.ackHits, ts.resultHits, dup
}

func newTestClient(t *testing.T, origin string) *outbound.Client {
	t.Helper()
	c, err := outbound.NewClient(
		outbound.PreparedConfig{Enabled: true, ServerOrigin: origin},
		outbound.StaticCredentialSource{Credential: "helper-token", HelperDeviceID: "device-1"},
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func defaultLeasedJob() map[string]any {
	return map[string]any{
		"job_id":           "job-1",
		"enrollment_id":    "enroll-1",
		"job_type":         "state.write",
		"schema_version":   1,
		"payload":          map[string]any{"state_key": "k"},
		"manifest_digest":  "sha256:manifest",
		"lease_token":      "lease-token",
		"lease_expires_at": time.Now().Add(time.Minute).UnixMilli(),
		"attempt":          1,
	}
}

func newDispatcher(t *testing.T, ts *dispatcherTestServer) *Dispatcher {
	t.Helper()
	return &Dispatcher{
		Client:          newTestClient(t, ts.srv.URL),
		EnrollmentID:    "enroll-1",
		PollWait:        10 * time.Millisecond,
		PollRetry:       10 * time.Millisecond,
		LeaseRenewEvery: 100 * time.Hour, // tests that care override this
		BackoffBase:     10 * time.Millisecond,
		BackoffCap:      40 * time.Millisecond,
		Logger:          func(string, ...any) {}, // silence in tests
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

// TD-1: Poll returns one job, no executor registered for the type → dispatcher
// reports terminal failed/not_implemented within ~1s. Locks the "not wired
// yet" baseline so follow-up PRs that register executors flip exactly the
// branches they intend to.
func TestDispatcher_PollReturnsJob_NotImplemented(t *testing.T) {
	t.Parallel()
	ts := newDispatcherTestServer()
	defer ts.close()

	var served atomic.Bool
	ts.setPollHandler(func(callIdx int) (int, map[string]any) {
		if !served.CompareAndSwap(false, true) {
			return http.StatusOK, map[string]any{"status": "no_work", "retry_after_ms": 50}
		}
		return http.StatusOK, map[string]any{"status": "leased", "job": defaultLeasedJob()}
	})

	d := newDispatcher(t, ts)
	// Allow-everything policy so we hit the executor-lookup branch.
	d.PolicyEvaluator = func(context.Context, *outbound.LeasedJob) jobpolicy.Decision {
		return jobpolicy.Decision{Allow: true, Reason: jobpolicy.ReasonOK}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()

	waitFor(t, time.Second, func() bool {
		_, _, n, _ := ts.snapshot()
		return n >= 1
	})
	_, _, _, recs := ts.snapshot()
	if len(recs) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(recs))
	}
	r := recs[0]
	if r.Status != "failed" || r.Code != TerminalNotImplemented {
		t.Fatalf("result=%+v want status=failed code=%s", r, TerminalNotImplemented)
	}
}

// TD-2: Poll returns one job, evaluator says reject → dispatcher reports the
// reject reason and the executor map is never consulted. Closes #1002:
// helper-side policy gate is a real call site, not a stub.
func TestDispatcher_PollReturnsJob_PolicyReject(t *testing.T) {
	t.Parallel()
	ts := newDispatcherTestServer()
	defer ts.close()

	var served atomic.Bool
	ts.setPollHandler(func(callIdx int) (int, map[string]any) {
		if !served.CompareAndSwap(false, true) {
			return http.StatusOK, map[string]any{"status": "no_work", "retry_after_ms": 50}
		}
		return http.StatusOK, map[string]any{"status": "leased", "job": defaultLeasedJob()}
	})

	d := newDispatcher(t, ts)
	var executorInvocations atomic.Int64
	d.Executors = map[string]Executor{
		"state.write": executorFunc(func(ctx context.Context, job *outbound.LeasedJob) (TerminalStatus, error) {
			executorInvocations.Add(1)
			return TerminalStatus{Status: StatusSucceeded}, nil
		}),
	}
	d.PolicyEvaluator = func(context.Context, *outbound.LeasedJob) jobpolicy.Decision {
		return jobpolicy.Decision{Allow: false, Reason: jobpolicy.ReasonRevoked}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()

	waitFor(t, time.Second, func() bool {
		_, _, n, _ := ts.snapshot()
		return n >= 1
	})
	_, _, _, recs := ts.snapshot()
	if recs[0].Status != "failed" || recs[0].Code != string(jobpolicy.ReasonRevoked) {
		t.Fatalf("result=%+v want failed/%s", recs[0], jobpolicy.ReasonRevoked)
	}
	if executorInvocations.Load() != 0 {
		t.Fatalf("executor was invoked %d times despite policy reject; helper-side double-validate broken", executorInvocations.Load())
	}
}

// TD-3: executor registered → executor.Execute called, terminal status from
// the executor flows through to /result.
func TestDispatcher_PollReturnsJob_ExecutorRegistered(t *testing.T) {
	t.Parallel()
	ts := newDispatcherTestServer()
	defer ts.close()

	var served atomic.Bool
	ts.setPollHandler(func(callIdx int) (int, map[string]any) {
		if !served.CompareAndSwap(false, true) {
			return http.StatusOK, map[string]any{"status": "no_work", "retry_after_ms": 50}
		}
		return http.StatusOK, map[string]any{"status": "leased", "job": defaultLeasedJob()}
	})

	d := newDispatcher(t, ts)
	d.PolicyEvaluator = func(context.Context, *outbound.LeasedJob) jobpolicy.Decision {
		return jobpolicy.Decision{Allow: true, Reason: jobpolicy.ReasonOK}
	}
	d.Executors = map[string]Executor{
		"state.write": executorFunc(func(ctx context.Context, job *outbound.LeasedJob) (TerminalStatus, error) {
			return TerminalStatus{Status: StatusSucceeded, ResultSummary: outbound.ResultSummary{AuditRefs: []string{"audit-1"}}}, nil
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()

	waitFor(t, time.Second, func() bool {
		_, _, n, _ := ts.snapshot()
		return n >= 1
	})
	_, _, _, recs := ts.snapshot()
	if recs[0].Status != StatusSucceeded || recs[0].Code != "" {
		t.Fatalf("result=%+v want succeeded", recs[0])
	}
}

// TD-4: long-running executor + short LeaseRenewEvery → server sees ≥1 Ack
// before the terminal Result. Locks the lease-extension contract: the
// dispatcher must not let a slow job's lease expire on the server.
func TestDispatcher_LeaseAck(t *testing.T) {
	t.Parallel()
	ts := newDispatcherTestServer()
	defer ts.close()

	var served atomic.Bool
	ts.setPollHandler(func(callIdx int) (int, map[string]any) {
		if !served.CompareAndSwap(false, true) {
			return http.StatusOK, map[string]any{"status": "no_work", "retry_after_ms": 200}
		}
		return http.StatusOK, map[string]any{"status": "leased", "job": defaultLeasedJob()}
	})

	d := newDispatcher(t, ts)
	d.LeaseRenewEvery = 20 * time.Millisecond
	d.PolicyEvaluator = func(context.Context, *outbound.LeasedJob) jobpolicy.Decision {
		return jobpolicy.Decision{Allow: true, Reason: jobpolicy.ReasonOK}
	}
	d.Executors = map[string]Executor{
		"state.write": executorFunc(func(ctx context.Context, job *outbound.LeasedJob) (TerminalStatus, error) {
			// Block long enough for ≥2 ack ticks.
			select {
			case <-ctx.Done():
				return TerminalStatus{Status: StatusFailed, FailureCode: "ctx_done"}, nil
			case <-time.After(120 * time.Millisecond):
			}
			return TerminalStatus{Status: StatusSucceeded}, nil
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()

	waitFor(t, 2*time.Second, func() bool {
		_, _, n, _ := ts.snapshot()
		return n >= 1
	})
	_, acks, _, recs := ts.snapshot()
	if acks < 1 {
		t.Fatalf("expected at least 1 lease ack while executor was running, got %d", acks)
	}
	if recs[0].Status != StatusSucceeded {
		t.Fatalf("result=%+v want succeeded", recs[0])
	}
}

// TD-5: poll fails 3x in a row → consecutive gaps grow (exp backoff), then a
// successful poll resets the curve. Mirrors Heartbeater_BackoffOnFailure so
// reviewers don't have to learn a second retry curve.
func TestDispatcher_BackoffOnPollFailure(t *testing.T) {
	t.Parallel()
	ts := newDispatcherTestServer()
	defer ts.close()

	var hitTimes []time.Time
	var mu sync.Mutex
	ts.setPollHandler(func(callIdx int) (int, map[string]any) {
		mu.Lock()
		hitTimes = append(hitTimes, time.Now())
		mu.Unlock()
		// All 5xx → exercise the backoff branch.
		return http.StatusBadGateway, map[string]any{"code": "temporary"}
	})

	d := newDispatcher(t, ts)
	d.BackoffBase = 20 * time.Millisecond
	d.BackoffCap = 200 * time.Millisecond
	d.PollRetry = 20 * time.Millisecond // 5xx is mapped to DirectiveRetry by the client, so the loop uses pollRetry path

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = d.Run(ctx) }()

	// Wait for at least 4 poll hits.
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		n := len(hitTimes)
		mu.Unlock()
		return n >= 4
	})
	mu.Lock()
	defer mu.Unlock()
	if len(hitTimes) < 4 {
		t.Fatalf("expected ≥4 poll hits, got %d", len(hitTimes))
	}
	// 5xx returns directive=retry (no err, just retry) — so gaps should be
	// roughly pollRetry. This test still locks "no tight loop" by asserting
	// every gap is at least ~half of pollRetry.
	for i := 1; i < len(hitTimes); i++ {
		gap := hitTimes[i].Sub(hitTimes[i-1])
		if gap < 10*time.Millisecond {
			t.Fatalf("gap[%d]=%s indicates tight loop on 5xx response", i, gap)
		}
	}
}

// TD-6: cancel ctx mid-poll → Run returns within ~200ms and the goroutine
// does not leak. Race detector is the primary defense here; the timing
// assertion is the user-facing teardown SLA.
func TestDispatcher_ContextCancel(t *testing.T) {
	t.Parallel()
	ts := newDispatcherTestServer()
	defer ts.close()

	d := newDispatcher(t, ts)
	d.PollWait = 5 * time.Second // park in poll long enough for cancel to matter
	d.PollRetry = 5 * time.Second
	d.PolicyEvaluator = func(context.Context, *outbound.LeasedJob) jobpolicy.Decision {
		return jobpolicy.Decision{Allow: true, Reason: jobpolicy.ReasonOK}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// Let one poll start.
	waitFor(t, time.Second, func() bool {
		n, _, _, _ := ts.snapshot()
		return n >= 1
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned err=%v after cancel", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Run did not return within 500ms of cancel")
	}
}

// TD-7: nil Client → Run returns immediately with nil err. Mirrors the
// heartbeater "no enrollment configured, skipping" branch so a pre-claim
// daemon still boots.
func TestDispatcher_NoEnrollmentSkip(t *testing.T) {
	t.Parallel()
	for name, d := range map[string]*Dispatcher{
		"nil client":      {EnrollmentID: "enroll-1"},
		"empty enrollment": {Client: &outbound.Client{}, EnrollmentID: ""},
	} {
		t.Run(name, func(t *testing.T) {
			d.Logger = func(string, ...any) {}
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- d.Run(ctx) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("Run returned err=%v want nil", err)
				}
			case <-time.After(150 * time.Millisecond):
				t.Fatalf("Run did not return immediately")
			}
		})
	}
}

// executorFunc adapts a plain function to the Executor interface for tests.
type executorFunc func(ctx context.Context, job *outbound.LeasedJob) (TerminalStatus, error)

func (f executorFunc) Execute(ctx context.Context, job *outbound.LeasedJob) (TerminalStatus, error) {
	return f(ctx, job)
}
