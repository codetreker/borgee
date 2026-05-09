// Package api_test — AL-1.4 state log endpoint owner-only ACL + scope tests.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestAL_GetStateLog_OwnerSeesAgentHistory pins acceptance §read path —
// owner GET returns DESC ts ordered transitions for own agent.
func TestAL_GetStateLog_OwnerSeesAgentHistory(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	owner, _ := s.GetUserByEmail("owner@test.com")
	// Create agent via REST so OwnerID = owner.ID + role='agent'.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken,
		map[string]any{"display_name": "test-agent"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d %v", resp.StatusCode, body)
	}
	agent := body["agent"].(map[string]any)
	agentID := agent["id"].(string)

	// Append 3 state transitions via store helper.
	for _, tr := range []struct {
		from, to store.AgentState
		reason   string
		taskID   string
	}{
		{store.AgentStateInitial, store.AgentStateOnline, "", ""},
		{store.AgentStateOnline, store.AgentStateBusy, "", "task-1"},
		{store.AgentStateBusy, store.AgentStateIdle, "", "task-1"},
	} {
		if _, err := s.AppendAgentStateTransition(agentID, tr.from, tr.to, tr.reason, tr.taskID); err != nil {
			t.Fatalf("append %v→%v: %v", tr.from, tr.to, err)
		}
	}

	// Owner GET sees 3 transitions.
	resp2, body2 := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agentID+"/state-log", ownerToken, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp2.StatusCode, body2)
	}
	transitions := body2["transitions"].([]any)
	if len(transitions) != 3 {
		t.Errorf("expected 3 transitions, got %d", len(transitions))
	}
	// Verify field shape on first row.
	row := transitions[0].(map[string]any)
	for _, key := range []string{"id", "from_state", "to_state", "reason", "task_id", "ts"} {
		if _, ok := row[key]; !ok {
			t.Errorf("row missing key %q", key)
		}
	}
	// Reverse: row should NOT have agent_id (already in URL path).
	if _, has := row["agent_id"]; has {
		t.Error("response row should not duplicate agent_id (in URL path)")
	}

	// 用 owner 不应触发 NotFound.
	_ = owner
}

// TestAL_GetStateLog_NonOwnerRejected pins 设计 ① owner-only ACL — non-owner
// → 403.
func TestAL_GetStateLog_NonOwnerRejected(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Owner creates an agent.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken,
		map[string]any{"display_name": "test-agent"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d", resp.StatusCode)
	}
	agentID := body["agent"].(map[string]any)["id"].(string)

	// member token (not owner) → 403.
	resp2, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agentID+"/state-log", memberToken, nil)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner expected 403, got %d", resp2.StatusCode)
	}
	_ = s
}

// TestAL_GetStateLog_UnauthenticatedReturns401 pins user-rail auth gate.
func TestAL_GetStateLog_UnauthenticatedReturns401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/some-id/state-log", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestAL_GetStateLog_AgentNotFound pins 404 path.
func TestAL_GetStateLog_AgentNotFound(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/non-existent-uuid/state-log", token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent agent, got %d", resp.StatusCode)
	}
}

// TestAL_GetStateLog_NonAgentRejected pins 设计 ① — calling state-log
// on a non-agent user (e.g., another human) → 404 (we treat as not-found
// rather than leak that the id is a user).
func TestAL_GetStateLog_NonAgentRejected(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	member, _ := s.GetUserByEmail("member@test.com")

	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+member.ID+"/state-log", token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-agent user, got %d", resp.StatusCode)
	}
}
