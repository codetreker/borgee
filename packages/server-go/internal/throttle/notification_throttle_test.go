// G2.3 five-test matrix from #229: T1 const pin, T2 first call, T3 within-
// window suppress, T4 cross-window reset, T5 two-dimensional key isolation.
// Use clock.Fake only; no wall-clock sleeps (G2.3 blocking constraint #4).
package throttle

import (
	"testing"
	"time"

	"borgee-server/internal/testutil/clock"
)

func TestThrottleWindow_Is5Minutes(t *testing.T) {
	if ThrottleWindow != 5*time.Minute {
		t.Fatalf("ThrottleWindow = %v, want 5m", ThrottleWindow)
	}
}

func TestAllow_FirstCall(t *testing.T) {
	tr := New(clock.NewFake(time.Time{}))
	if !tr.Allow("ch-1", "ag-1") {
		t.Fatal("first @ on cold key must Allow")
	}
}

func TestAllow_SuppressedWithinWindow(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	tr := New(fake)
	if !tr.Allow("ch-1", "ag-1") {
		t.Fatal("first @ should Allow")
	}
	fake.Advance(4*time.Minute + 59*time.Second)
	for i := 0; i < 6; i++ {
		if tr.Allow("ch-1", "ag-1") {
			t.Fatalf("call #%d within window must be suppressed", i+2)
		}
	}
}

func TestAllow_CrossWindowResets(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	tr := New(fake)
	if !tr.Allow("ch-1", "ag-1") {
		t.Fatal("first @ should Allow")
	}
	fake.Advance(ThrottleWindow) // boundary: >=, not <
	if !tr.Allow("ch-1", "ag-1") {
		t.Fatal("@ at ThrottleWindow boundary must Allow")
	}
	fake.Advance(time.Minute)
	if tr.Allow("ch-1", "ag-1") {
		t.Fatal("inside 2nd window must suppress")
	}
}

func TestAllow_TwoDimensionalIsolation(t *testing.T) {
	tr := New(clock.NewFake(time.Time{}))
	if !tr.Allow("ch-1", "ag-1") {
		t.Fatal("baseline (ch-1, ag-1) should Allow")
	}
	if !tr.Allow("ch-2", "ag-1") {
		t.Fatal("(ch-2, ag-1) must NOT inherit suppression")
	}
	if !tr.Allow("ch-1", "ag-2") {
		t.Fatal("(ch-1, ag-2) must NOT inherit suppression")
	}
	if tr.Allow("ch-1", "ag-1") || tr.Allow("ch-2", "ag-1") || tr.Allow("ch-1", "ag-2") {
		t.Fatal("each (ch, ag) keeps its own window suppressed")
	}
}
