// Package api_test — AL-5 recovery endpoint owner-only ACL + 5-state graph
// gate tests (跟 AL-1 #492 state-log endpoint 测试同模式).

package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestAL_Recover_Owner_HappyPath 覆盖 acceptance §2.1 — owner POST recovers
// agent from error → online via AL-1 #492 single-gate helper, returns 200
// with reason carried forward (REFACTOR-REASONS #496 SSOT 同一设计).
func TestAL_Recover_Owner_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Create agent owned by owner.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken,
		map[string]any{"display_name": "test-agent-recover"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d %v", resp.StatusCode, body)
	}
	agent := body["agent"].(map[string]any)
	agentID := agent["id"].(string)

	// Seed state-log: initial → error with reason 'api_key_invalid'.
	if _, err := s.AppendAgentStateTransition(agentID,
		store.AgentStateInitial, store.AgentStateOnline, "", ""); err != nil {
		t.Fatalf("seed online: %v", err)
	}
	if _, err := s.AppendAgentStateTransition(agentID,
		store.AgentStateOnline, store.AgentStateError, "api_key_invalid", ""); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	// Owner POST /recover — should succeed.
	resp2, body2 := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/agents/"+agentID+"/recover", ownerToken,
		map[string]any{"request_id": "test-req-1"})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp2.StatusCode, body2)
	}
	if got := body2["state"]; got != "online" {
		t.Errorf("expected state=online, got %v", got)
	}
	if got := body2["reason"]; got != "api_key_invalid" {
		t.Errorf("expected reason=api_key_invalid (carried forward), got %v", got)
	}

	// Verify state-log row was appended (forward-only audit, AL-1 设计 ①).
	rows, _ := s.ListAgentStateLog(agentID, 10)
	if len(rows) < 3 {
		t.Errorf("expected ≥3 transitions (online + error + recover), got %d", len(rows))
	}
	if rows[0].FromState != "error" || rows[0].ToState != "online" {
		t.Errorf("most recent should be error→online, got %s→%s",
			rows[0].FromState, rows[0].ToState)
	}
}

// TestAL_Recover_NonOwnerRejected 验证设计 ② owner-only ACL — non-owner → 403.
func TestAL_Recover_NonOwnerRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Owner creates agent.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken,
		map[string]any{"display_name": "test-agent"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d", resp.StatusCode)
	}
	agentID := body["agent"].(map[string]any)["id"].(string)

	// member token (not owner) → 403.
	resp2, _ := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/agents/"+agentID+"/recover", memberToken, nil)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner expected 403, got %d", resp2.StatusCode)
	}
}

// TestAL_Recover_Unauthenticated401 验证 user-rail auth gate.
func TestAL_Recover_Unauthenticated401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/agents/some-id/recover", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAL_Recover_AgentNotFound 验证 404 path (跟 AL-1 #492 state-log endpoint 同).
func TestAL_Recover_AgentNotFound(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/agents/non-existent-uuid/recover", token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestAL_Recover_NotInErrorStateConflict 验证设计 ② state machine gate —
// agent must currently be in `error` state to recover; otherwise 409.
func TestAL_Recover_NotInErrorStateConflict(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken,
		map[string]any{"display_name": "test-agent-online"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d", resp.StatusCode)
	}
	agentID := body["agent"].(map[string]any)["id"].(string)

	// Seed: initial → online (NOT error).
	if _, err := s.AppendAgentStateTransition(agentID,
		store.AgentStateInitial, store.AgentStateOnline, "", ""); err != nil {
		t.Fatalf("seed online: %v", err)
	}

	// POST /recover — should reject with 409 (not in error state).
	resp2, _ := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/agents/"+agentID+"/recover", ownerToken, nil)
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("non-error state expected 409, got %d", resp2.StatusCode)
	}
}

// TestAL_Recover_NoStateLogConflict 验证 behavior — agent without any
// state-log history cannot recover (no error to recover from); 409.
func TestAL_Recover_NoStateLogConflict(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken,
		map[string]any{"display_name": "test-agent-fresh"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d", resp.StatusCode)
	}
	agentID := body["agent"].(map[string]any)["id"].(string)

	// No state-log seeded.
	resp2, _ := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/agents/"+agentID+"/recover", ownerToken, nil)
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("no history expected 409, got %d", resp2.StatusCode)
	}
}

// TestAL_Recover_AdminAPINotMounted 验证 ADM-0 §1.3 限制 — admin 权限
// 不挂业务 recovery 路径 (跟 AL-2a admin path 同一检查思路).
func TestAL_Recover_AdminAPINotMounted(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminTok := testutil.LoginAsAdmin(t, ts.URL)
	resp, _ := testutil.JSON(t, "POST",
		ts.URL+"/admin-api/v1/agents/some-id/recover", adminTok, nil)
	// Either 404 (route not mounted) or 401/403 — anything but 200 confirms
	// the admin rail does NOT mount this endpoint.
	if resp.StatusCode == http.StatusOK {
		t.Errorf("admin-api MUST NOT mount /recover (ADM-0 §1.3 限制): got 200")
	}
}
