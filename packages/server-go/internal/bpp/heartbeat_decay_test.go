// Package bpp — heartbeat_decay_test.go: HB-3 v2.1 + v2.2 unit tests.
package bpp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
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

// TestHB_NoSchemaChange — acceptance §1.2 设计 ① 反断.
//
// Reverse-grep production migrations + bpp/api packages for forbidden
// HB-3 v2 decay table / schema literals.
func TestHB_NoSchemaChange(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"heartbeat_decay_table",
		"hb3_decay_log",
		"stale_ratio_history",
		"CREATE TABLE heartbeat_decay",
		"ALTER TABLE heartbeat_decay",
	}
	dirs := []string{"../migrations", ".", "../api"}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbidden {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("HB-3 v2 规则 ① broken: forbidden decay table / schema literals: %v", hits)
	}
}

// TestHB_DecayState_LiteralSingleSource — acceptance §2.3 设计 ④.
//
// hardcode "fresh"/"stale"/"dead" literals in production *.go MUST
// only appear in heartbeat_decay.go (source of truth). envelope.go
// references "stale" as part of an unrelated heartbeat status enum
// (BPP-1 plugin status: online/working/offline + the legacy
// 'stale' label) so it's whitelisted explicitly.
func TestHB_DecayState_LiteralSingleSource(t *testing.T) {
	t.Parallel()
	whitelist := map[string]bool{
		"heartbeat_decay.go": true,
		"envelope.go":        true, // pre-existing "stale" status enum (unrelated)
	}
	literals := []string{`"fresh"`, `"stale"`, `"dead"`}
	dirs := []string{"."}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			if whitelist[e.Name()] {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range literals {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("HB-3 v2 规则 ④ broken: DecayState literals outside heartbeat_decay.go: %v", hits)
	}
}

// TestHB_NoStaleSidePath — acceptance §2.1 设计 ②.
// 反向 grep `RecordHeartbeatStale\|LifecycleAuditor.*Stale` 0 hit
// (复用 BPP-8 RecordHeartbeatTimeout, 不另开 Stale 旁路).
func TestHB_NoStaleSidePath(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"RecordHeartbeatStale",
		"LifecycleStaleEvent",
		"LifecycleActionStale",
	}
	dirs := []string{".", "../api"}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbidden {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("HB-3 v2 规则 ② broken: forbidden Stale side-path identifiers (复用 BPP-8 RecordHeartbeatTimeout): %v", hits)
	}
}

// TestHB_NoDecayQueueOrSchema — acceptance §3.1 设计 ⑤
// best-effort 延伸第 6 处.
func TestHB_NoDecayQueueOrSchema(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingDecayQueue",
		"decayRetryQueue",
		"deadLetterDecay",
		"hb3DecayQueue",
		"decayPersistTable",
	}
	dirs := []string{"."}
	fset := token.NewFileSet()
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				ident, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				for _, bad := range forbidden {
					if strings.Contains(ident.Name, bad) {
						hits = append(hits, path+":"+ident.Name)
					}
				}
				return true
			})
		}
	}
	if len(hits) > 0 {
		t.Errorf("HB-3 v2 规则 ⑤ broken: forbidden decay-queue identifiers (best-effort 延伸第 6 处, BPP-4/5/6/7/8 + HB-3 v2): %v", hits)
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

// TestHB_AdminGodModeNotMounted — acceptance §3.2.
func TestHB_AdminGodModeNotMounted(t *testing.T) {
	t.Parallel()
	dir := "../api"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	literals := []string{
		"admin/heartbeat-decay",
		"admin/heartbeat/decay",
		"AdminHeartbeatDecay",
		"AdminHB3",
	}
	hits := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "admin") {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(b)
		for _, bad := range literals {
			if strings.Contains(content, bad) {
				hits = append(hits, path+":"+bad)
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("HB-3 v2 规则 ③ broken: admin god-mode references heartbeat-decay (ADM-0 §1.3 红线): %v", hits)
	}
}
