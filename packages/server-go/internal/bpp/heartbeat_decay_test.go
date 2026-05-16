// Package bpp — heartbeat_decay_test.go: HB-3 v2.1 + v2.2 unit tests.
package bpp

import (
	"sync"
	"testing"
	"time"
)

// TestHB_DeriveDecayState_Boundaries — acceptance §1.1.
// Boundaries: t=0/29/30/59/60 (seconds since lastHeartbeatAt) →
// fresh/fresh/stale/stale/dead. 30s and 60s are inclusive boundaries
// of fresh and stale respectively.
func TestHB_DeriveDecayState_Boundaries(t *testing.T) {
	t.Parallel()
	const lastHB = int64(1_000_000_000_000)
	cases := []struct {
		deltaMs int64
		want    DecayState
		desc    string
	}{
		{0, DecayStateFresh, "t=0 → fresh"},
		{29_000, DecayStateFresh, "t=29s → fresh"},
		{30_000, DecayStateFresh, "t=30s exact StaleThreshold → fresh (≤)"},
		{30_001, DecayStateStale, "t=30.001s → stale (boundary cross)"},
		{45_000, DecayStateStale, "t=45s → stale"},
		{59_000, DecayStateStale, "t=59s → stale"},
		{60_000, DecayStateStale, "t=60s exact DeadThreshold → stale (≤)"},
		{60_001, DecayStateDead, "t=60.001s → dead (boundary cross)"},
		{120_000, DecayStateDead, "t=120s → dead"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			now := lastHB + tc.deltaMs
			got := DeriveDecayState(now, lastHB)
			if got != tc.want {
				t.Errorf("delta=%dms: got %q, want %q", tc.deltaMs, got, tc.want)
			}
		})
	}
}

// TestHB_DeriveDecayState_NilSafe — acceptance §1.3.
// 0 / negative lastHeartbeatAt → dead (never alive).
// future-dated lastHeartbeatAt (now < last) → fresh (clamp delta=0).
func TestHB_DeriveDecayState_NilSafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		now, last int64
		want      DecayState
		desc      string
	}{
		{1_000, 0, DecayStateDead, "last=0 → dead"},
		{1_000, -5, DecayStateDead, "last<0 → dead"},
		{500, 1_000, DecayStateFresh, "future-dated last → fresh (clamp 0)"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if got := DeriveDecayState(tc.now, tc.last); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestHB_ConstThresholdsByteIdentical — acceptance §2.3 设计 ⑥.
// StaleThreshold byte-identical 跟 BPP-4 watchdog 30s + BPP-7 SDK
// HeartbeatInterval 30s 同源.
func TestHB_ConstThresholdsByteIdentical(t *testing.T) {
	t.Parallel()
	if StaleThreshold != 30*time.Second {
		t.Errorf("StaleThreshold mismatch: got %v, want 30s (BPP-4 watchdog + BPP-7 SDK 同源)", StaleThreshold)
	}
	if DeadThreshold != 60*time.Second {
		t.Errorf("DeadThreshold mismatch: got %v, want 60s", DeadThreshold)
	}
}

// ---- HB-3 v2.2 watchdog wire helper ----

// HeartbeatTimeoutAuditor — minimal interface this package needs from
// BPP-8 LifecycleAuditor. HB-3 v2 references only the heartbeat-timeout
// recording method to stay decoupled from BPP-8 PR ordering. BPP-8's
// AdminActionsLifecycleAuditor satisfies this interface.
type HeartbeatTimeoutAuditor interface {
	RecordHeartbeatTimeout(pluginID, agentID string)
}

// BucketAuditTrigger encapsulates the cross-bucket transition rule
// (设计 ⑦): only fire BPP-8 audit on cross-bucket transitions
// (fresh→stale / stale→dead / etc), same-bucket is no-op.
type BucketAuditTrigger struct {
	auditor HeartbeatTimeoutAuditor
}

// NewBucketAuditTrigger wires an auditor (BPP-8 LifecycleAuditor impl
// satisfies HeartbeatTimeoutAuditor). Nil auditor panics — boot bug.
func NewBucketAuditTrigger(auditor HeartbeatTimeoutAuditor) *BucketAuditTrigger {
	if auditor == nil {
		panic("bpp: NewBucketAuditTrigger auditor must not be nil")
	}
	return &BucketAuditTrigger{auditor: auditor}
}

// MaybeFire — fires RecordHeartbeatTimeout iff cross-bucket transition
// (设计 ⑦). Same-bucket is no-op.
func (b *BucketAuditTrigger) MaybeFire(from, to DecayState, pluginID, agentID string) {
	if !IsCrossBucketTransition(from, to) {
		return
	}
	b.auditor.RecordHeartbeatTimeout(pluginID, agentID)
}

// stubLifecycleAuditor for test verification of trigger calls.
type stubBucketAuditor struct {
	mu    sync.Mutex
	calls int
}

func (s *stubBucketAuditor) RecordHeartbeatTimeout(pluginID, agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
}

// TestHB_CrossBucket_TriggersAudit — acceptance §2.1.
func TestHB_CrossBucket_TriggersAudit(t *testing.T) {
	t.Parallel()
	st := &stubBucketAuditor{}
	tr := NewBucketAuditTrigger(st)
	tr.MaybeFire(DecayStateFresh, DecayStateStale, "p", "a")
	if st.calls != 1 {
		t.Errorf("cross-bucket fresh→stale: got %d calls, want 1", st.calls)
	}
}

// TestHB_SameBucket_NoAuditCall — acceptance §2.1 设计 ⑦.
func TestHB_SameBucket_NoAuditCall(t *testing.T) {
	t.Parallel()
	st := &stubBucketAuditor{}
	tr := NewBucketAuditTrigger(st)
	tr.MaybeFire(DecayStateFresh, DecayStateFresh, "p", "a")
	tr.MaybeFire(DecayStateStale, DecayStateStale, "p", "a")
	tr.MaybeFire(DecayStateDead, DecayStateDead, "p", "a")
	if st.calls != 0 {
		t.Errorf("same-bucket: got %d calls, want 0", st.calls)
	}
}

// TestHB_BucketAuditTrigger_NilSafeCtor — boot bug.
func TestHB_BucketAuditTrigger_NilSafeCtor(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("expected panic on nil auditor")
		}
	}()
	NewBucketAuditTrigger(nil)
}
