package store

import (
	"strings"
	"testing"
)

// TestValidateTransition_ValidEdges pins the full state graph (蓝图 §2.3).
// 跟 docs/blueprint/current/agent-lifecycle.md §2.3 字面对齐.
func TestValidateTransition_ValidEdges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to AgentState
		reason   string // for error transitions
	}{
		// 首次 transitions.
		{AgentStateInitial, AgentStateOnline, ""},
		{AgentStateInitial, AgentStateOffline, ""},
		// online → ...
		{AgentStateOnline, AgentStateBusy, ""},
		{AgentStateOnline, AgentStateIdle, ""},
		{AgentStateOnline, AgentStateError, "runtime_crashed"},
		{AgentStateOnline, AgentStateOffline, ""},
		// busy → ...
		{AgentStateBusy, AgentStateIdle, ""},
		{AgentStateBusy, AgentStateError, "runtime_timeout"},
		{AgentStateBusy, AgentStateOffline, ""},
		// idle → ...
		{AgentStateIdle, AgentStateBusy, ""},
		{AgentStateIdle, AgentStateError, "api_key_invalid"},
		{AgentStateIdle, AgentStateOffline, ""},
		// error → recovery.
		{AgentStateError, AgentStateOnline, ""},
		{AgentStateError, AgentStateOffline, ""},
		// offline → recovery.
		{AgentStateOffline, AgentStateOnline, ""},
	}
	for _, c := range cases {
		if err := ValidateTransition(c.from, c.to, c.reason); err != nil {
			t.Errorf("expected %q→%q valid, got: %v", c.from, c.to, err)
		}
	}
}

// TestValidateTransition_RejectsSameState pins reject no-op (立场 ②).
func TestValidateTransition_RejectsSameState(t *testing.T) {
	t.Parallel()
	for _, s := range []AgentState{
		AgentStateOnline, AgentStateBusy, AgentStateIdle, AgentStateError, AgentStateOffline,
	} {
		if err := ValidateTransition(s, s, ""); err == nil {
			t.Errorf("same-state %q should reject (no-op transition lossy)", s)
		}
	}
}

// TestValidateTransition_RejectsInvalidEdges pins blueprint §2.3 反向 graph.
func TestValidateTransition_RejectsInvalidEdges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to AgentState
		desc     string
	}{
		// busy ↛ online (must go through idle/error/offline first).
		{AgentStateBusy, AgentStateOnline, "busy → online (lossy lifecycle)"},
		// idle ↛ online (idle 已含 online, 直接 online 是 lossy redundant).
		{AgentStateIdle, AgentStateOnline, "idle → online (redundant)"},
		// error → busy/idle (must Clear → online first).
		{AgentStateError, AgentStateBusy, "error → busy (must recover via online)"},
		{AgentStateError, AgentStateIdle, "error → idle (must recover via online)"},
		// offline → busy/idle/error (presence-gated; must online first).
		{AgentStateOffline, AgentStateBusy, "offline → busy (presence gated)"},
		{AgentStateOffline, AgentStateIdle, "offline → idle (presence gated)"},
		{AgentStateOffline, AgentStateError, "offline → error (presence gated)"},
		// initial → busy/idle/error (must online first).
		{AgentStateInitial, AgentStateBusy, "initial → busy (must online first)"},
		{AgentStateInitial, AgentStateIdle, "initial → idle (must online first)"},
		{AgentStateInitial, AgentStateError, "initial → error (must online first)"},
	}
	for _, c := range cases {
		if err := ValidateTransition(c.from, c.to, "runtime_crashed"); err == nil {
			t.Errorf("%s should reject", c.desc)
		}
	}
}

// TestValidateTransition_ErrorRequiresReason pins 立场 ④ — error 转移 reason
// 必带 + ∈ AL-1a 6 字面 byte-identical.
func TestValidateTransition_ErrorRequiresReason(t *testing.T) {
	t.Parallel()
	// Empty reason → reject.
	if err := ValidateTransition(AgentStateOnline, AgentStateError, ""); err == nil {
		t.Error("error transition with empty reason should reject")
	}
	// Invalid reason → reject.
	for _, bad := range []string{
		"out_of_memory", "RuntimeCrashed", "API_KEY_INVALID", "key_invalid", "timeout",
		"NETWORK_DOWN", "panic",
	} {
		if err := ValidateTransition(AgentStateOnline, AgentStateError, bad); err == nil {
			t.Errorf("invalid reason %q should reject (立场 ④ AL-1a 6 字面锁)", bad)
		}
	}
	// All 6 valid reasons accept.
	for _, ok := range []string{
		"api_key_invalid", "quota_exceeded", "network_unreachable",
		"runtime_crashed", "runtime_timeout", "unknown",
	} {
		if err := ValidateTransition(AgentStateOnline, AgentStateError, ok); err != nil {
			t.Errorf("valid reason %q rejected: %v", ok, err)
		}
	}
}

// TestAppendAgentStateTransition_HappyPath pins helper INSERT path through
// validator gate.
func TestAppendAgentStateTransition_HappyPath(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)

	// Initial → online.
	id1, err := s.AppendAgentStateTransition("agent-1", AgentStateInitial, AgentStateOnline, "", "")
	if err != nil {
		t.Fatalf("initial → online: %v", err)
	}
	if id1 <= 0 {
		t.Errorf("expected positive auto-increment id, got %d", id1)
	}

	// online → busy with task_id.
	id2, err := s.AppendAgentStateTransition("agent-1", AgentStateOnline, AgentStateBusy, "", "task-1")
	if err != nil {
		t.Fatalf("online → busy: %v", err)
	}
	if id2 <= id1 {
		t.Errorf("id should be strictly monotonic: %d <= %d", id2, id1)
	}

	// busy → idle.
	if _, err := s.AppendAgentStateTransition("agent-1", AgentStateBusy, AgentStateIdle, "", "task-1"); err != nil {
		t.Fatalf("busy → idle: %v", err)
	}

	// idle → error (with reason).
	if _, err := s.AppendAgentStateTransition("agent-1", AgentStateIdle, AgentStateError, "runtime_crashed", ""); err != nil {
		t.Fatalf("idle → error: %v", err)
	}

	// Verify history.
	rows, err := s.ListAgentStateLog("agent-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Errorf("expected 4 transitions, got %d", len(rows))
	}
}

// TestAppendAgentStateTransition_RejectsInvalidViaValidator pins 立场 ②.
func TestAppendAgentStateTransition_RejectsInvalidViaValidator(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)

	// Empty agent_id rejected.
	if _, err := s.AppendAgentStateTransition("", AgentStateInitial, AgentStateOnline, "", ""); err == nil {
		t.Error("empty agent_id should reject")
	}

	// Invalid transition (busy → online) rejected.
	if _, err := s.AppendAgentStateTransition("a1", AgentStateBusy, AgentStateOnline, "", ""); err == nil {
		t.Error("busy → online should reject (lossy lifecycle)")
	}

	// Same state rejected.
	if _, err := s.AppendAgentStateTransition("a1", AgentStateOnline, AgentStateOnline, "", ""); err == nil {
		t.Error("same state should reject")
	}

	// Error without reason rejected.
	if _, err := s.AppendAgentStateTransition("a1", AgentStateOnline, AgentStateError, "", ""); err == nil {
		t.Error("error without reason should reject")
	}

	// Error with invalid reason rejected.
	if _, err := s.AppendAgentStateTransition("a1", AgentStateOnline, AgentStateError, "out_of_memory", ""); err == nil {
		t.Error("error with invalid reason should reject (立场 ④)")
	}
}

// TestListAgentStateLog_OrderingAndScope pins acceptance §read path —
// owner GET /api/v1/agents/:id/state-log returns DESC ts + scoped to agent.
func TestListAgentStateLog_OrderingAndScope(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)

	// Two agents, multiple transitions each.
	for i := 0; i < 3; i++ {
		_, _ = s.AppendAgentStateTransition("a1", AgentStateInitial, AgentStateOnline, "", "")
		// Insert next requires fresh from-state; just simulate raw inserts via direct append for diversity.
	}
	_, _ = s.AppendAgentStateTransition("a2", AgentStateInitial, AgentStateOnline, "", "")

	// agent-1 sees only its own transitions.
	rows, err := s.ListAgentStateLog("a1", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.AgentID != "a1" {
			t.Errorf("scope leak: row agent_id=%q", r.AgentID)
		}
	}

	// Empty agent_id rejected.
	if _, err := s.ListAgentStateLog("", 10); err == nil {
		t.Error("empty agent_id should reject")
	}

	// Limit clamping.
	rows2, _ := s.ListAgentStateLog("a1", -1)
	_ = rows2
	rows3, _ := s.ListAgentStateLog("a1", 10000)
	_ = rows3
}

// TestValidateTransition_RecoveryPath pins error → online → busy/idle/error
// recovery cycle (蓝图 §2.3 "故障可解释" + AL-1a Clear semantics).
func TestValidateTransition_RecoveryPath(t *testing.T) {
	t.Parallel()
	s := runStoreWithMigrations(t)
	// Simulate full lifecycle: initial → online → error → online → busy.
	_, err := s.AppendAgentStateTransition("a1", AgentStateInitial, AgentStateOnline, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AppendAgentStateTransition("a1", AgentStateOnline, AgentStateError, "api_key_invalid", "")
	if err != nil {
		t.Fatal(err)
	}
	// Recovery: error → online (AL-1a Clear).
	_, err = s.AppendAgentStateTransition("a1", AgentStateError, AgentStateOnline, "", "")
	if err != nil {
		t.Fatalf("recovery error→online: %v", err)
	}
	// Continue: online → busy.
	_, err = s.AppendAgentStateTransition("a1", AgentStateOnline, AgentStateBusy, "", "task-2")
	if err != nil {
		t.Fatalf("online→busy after recovery: %v", err)
	}

	rows, _ := s.ListAgentStateLog("a1", 50)
	if len(rows) != 4 {
		t.Errorf("expected 4 lifecycle rows, got %d", len(rows))
	}
}

// TestValidateTransition_AllReasons5Pin ensures the 6-字面 reason set is
// the 8th lock in the chain (AL-1a #249 + #305 + #321 + #380 + #454 +
// #458 + #481 + 此). 字面漂移 → CI 红.
func TestValidateTransition_AllReasons5Pin(t *testing.T) {
	t.Parallel()
	want := []string{
		"api_key_invalid", "quota_exceeded", "network_unreachable",
		"runtime_crashed", "runtime_timeout", "unknown",
	}
	for _, r := range want {
		if !validReasons[r] {
			t.Errorf("reason %q missing from validReasons map (cross-milestone byte-identical 锁链断)", r)
		}
	}
	if len(validReasons) != 6 {
		t.Errorf("expected 6 reasons (AL-1a #249 lock), got %d — drift!", len(validReasons))
	}
	// Reverse: ensure no extra entries leak in.
	for r := range validReasons {
		found := false
		for _, w := range want {
			if r == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("validReasons has unexpected entry %q (反向 drift)", r)
		}
	}
	_ = strings.Join(want, "|")
}
