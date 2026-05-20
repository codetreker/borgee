package outbound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testServer wraps httptest with an atomic hit counter + a swappable handler.
type testServer struct {
	srv     *httptest.Server
	hits    atomic.Int64
	hitTime atomic.Int64 // unix nano of last hit, 0 if none
	mu      sync.Mutex
	status  int
	body    func(w http.ResponseWriter, r *http.Request)
}

func newTestServer(status int) *testServer {
	ts := &testServer{status: status}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.hits.Add(1)
		ts.hitTime.Store(time.Now().UnixNano())
		ts.mu.Lock()
		body := ts.body
		st := ts.status
		ts.mu.Unlock()
		if body != nil {
			body(w, r)
			return
		}
		w.WriteHeader(st)
	}))
	return ts
}

func (ts *testServer) setStatus(s int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.status = s
}

func (ts *testServer) close() { ts.srv.Close() }

func newHeartbeater(origin string) *Heartbeater {
	return &Heartbeater{
		Client:         &http.Client{Timeout: 2 * time.Second},
		ServerOrigin:   origin,
		EnrollmentID:   "enroll-1",
		HelperDeviceID: "device-1",
		Credential:     "helper-token",
		Interval:       50 * time.Millisecond,
		BackoffBase:    20 * time.Millisecond,
		BackoffCap:     80 * time.Millisecond,
		Logger:         func(format string, v ...any) {}, // silence in tests
	}
}

func TestHeartbeater_FiresImmediately(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK)
	defer ts.close()

	h := newHeartbeater(ts.srv.URL)
	h.Interval = 10 * time.Second // ensure subsequent fires don't pollute timing

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	go func() { _ = h.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 1 {
			elapsed := time.Since(start)
			if elapsed > 200*time.Millisecond {
				t.Fatalf("first heartbeat fired %s after start; want <= 200ms", elapsed)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("first heartbeat never fired within 500ms")
}

func TestHeartbeater_RepeatsOnInterval(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK)
	defer ts.close()

	h := newHeartbeater(ts.srv.URL) // 50ms interval, success branch only
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected >=3 heartbeats in 500ms, got %d", ts.hits.Load())
}

func TestHeartbeater_BackoffOnFailure(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusServiceUnavailable)
	defer ts.close()

	h := newHeartbeater(ts.srv.URL)
	h.Interval = 10 * time.Second // success path won't reach
	// BackoffBase=20ms, BackoffCap=80ms. Expected: hit, 20ms, hit, 40ms, hit, 80ms, hit, 80ms ...

	var hitTimes []time.Time
	var mu sync.Mutex
	ts.mu.Lock()
	ts.body = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitTimes = append(hitTimes, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	ts.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.Run(ctx) }()

	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(hitTimes)
		mu.Unlock()
		if n >= 4 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hitTimes) < 4 {
		t.Fatalf("want >=4 hits with growing backoff, got %d", len(hitTimes))
	}
	gap1 := hitTimes[1].Sub(hitTimes[0])
	gap2 := hitTimes[2].Sub(hitTimes[1])
	gap3 := hitTimes[3].Sub(hitTimes[2])
	if gap2 < gap1 {
		t.Fatalf("expected backoff to grow: gap1=%s gap2=%s", gap1, gap2)
	}
	if gap3 < gap2 {
		t.Fatalf("expected backoff to grow: gap2=%s gap3=%s", gap2, gap3)
	}
	// Cap at 80ms + scheduling slack; allow up to 200ms.
	if gap3 > 200*time.Millisecond {
		t.Fatalf("gap3=%s exceeds reasonable cap (BackoffCap=80ms + slack)", gap3)
	}
}

func TestHeartbeater_ResetsOnSuccess(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusServiceUnavailable)
	defer ts.close()

	h := newHeartbeater(ts.srv.URL)
	h.Interval = 200 * time.Millisecond // succeeded → interval path = 200ms

	var hitTimes []time.Time
	var mu sync.Mutex
	var counter atomic.Int64
	ts.mu.Lock()
	ts.body = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitTimes = append(hitTimes, time.Now())
		mu.Unlock()
		n := counter.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
	ts.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.Run(ctx) }()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(hitTimes)
		mu.Unlock()
		if n >= 4 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(hitTimes) < 4 {
		t.Fatalf("want >=4 hits (2 fail + ≥2 success), got %d", len(hitTimes))
	}
	// After 3rd hit (1-indexed) returns 200, next gap should be interval (200ms)
	// not backoff. Allow scheduling slack: 150ms..400ms.
	gapAfterSuccess := hitTimes[3].Sub(hitTimes[2])
	if gapAfterSuccess < 150*time.Millisecond || gapAfterSuccess > 400*time.Millisecond {
		t.Fatalf("gap after success=%s, want ~200ms interval (not backoff)", gapAfterSuccess)
	}
}

func TestHeartbeater_ContextCancel(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusOK)
	defer ts.close()

	h := newHeartbeater(ts.srv.URL)
	h.Interval = 10 * time.Second // park in select after first hit

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.Run(ctx) }()

	// Wait for first heartbeat to fire so we know Run is in the wait branch.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && ts.hits.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if ts.hits.Load() < 1 {
		t.Fatalf("first heartbeat never fired")
	}

	cancel()
	select {
	case <-done:
		return
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("Run did not return within 200ms of context cancel")
	}
}

func TestHeartbeater_401Continues(t *testing.T) {
	t.Parallel()
	ts := newTestServer(http.StatusUnauthorized)
	defer ts.close()

	h := newHeartbeater(ts.srv.URL)
	h.Interval = 10 * time.Second

	var logged atomic.Int64
	h.Logger = func(format string, v ...any) {
		logged.Add(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.Run(ctx) }()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts.hits.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if ts.hits.Load() < 2 {
		t.Fatalf("401 should not stop heartbeater; got %d hits", ts.hits.Load())
	}
	if logged.Load() < 1 {
		t.Fatalf("expected at least one log line on 401 failure")
	}
}
