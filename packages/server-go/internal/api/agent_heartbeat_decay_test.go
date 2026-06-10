// Package api_test — agent_heartbeat_decay_test.go: agent heartbeat-decay
// view GET endpoint tests.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// seedAgentWithHeartbeat creates owner + agent and inserts an
// agent_runtimes row with the given lastHeartbeatAt (Unix ms). Returns
// (ownerToken, agentID).
func seedAgentWithHeartbeat(t *testing.T, ts *httptest.Server,
	s *store.Store, ageMs int64) (string, string) {
	t.Helper()
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	agentEmail := "agent-heartbeat-decay@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentHeartbeatDecay",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	now := time.Now().UnixMilli()
	heartbeat := now - ageMs
	if err := s.DB().Exec(`INSERT INTO agent_runtimes
		(agent_id, endpoint_url, process_kind, status, last_error_reason,
		 last_heartbeat_at, created_at, updated_at)
		VALUES (?, '', 'hermes', 'running', NULL, ?, ?, ?)`,
		agent.ID, heartbeat, now, now).Error; err != nil {
		t.Fatalf("seed agent_runtimes: %v", err)
	}
	return ownerToken, agent.ID
}

// TestAgentHeartbeatDecay_HappyPath — acceptance §2.2 (fresh).
func TestAgentHeartbeatDecay_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, agentID := seedAgentWithHeartbeat(t, ts, s, 5_000) // 5s ago → fresh

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agentID+"/heartbeat-decay", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("happy: %d %v", resp.StatusCode, body)
	}
	if body["state"] != "fresh" {
		t.Errorf("state: got %v, want fresh", body["state"])
	}
	if body["agent_id"] != agentID {
		t.Errorf("agent_id: got %v, want %s", body["agent_id"], agentID)
	}
}

// TestAgentHeartbeatDecay_StaleState — acceptance §2.2 (stale).
func TestAgentHeartbeatDecay_StaleState(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, agentID := seedAgentWithHeartbeat(t, ts, s, 45_000) // 45s ago → stale

	_, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agentID+"/heartbeat-decay", ownerToken, nil)
	if body["state"] != "stale" {
		t.Errorf("state: got %v, want stale", body["state"])
	}
}

// TestAgentHeartbeatDecay_DeadState — acceptance §2.2 (dead).
func TestAgentHeartbeatDecay_DeadState(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, agentID := seedAgentWithHeartbeat(t, ts, s, 120_000) // 120s ago → dead

	_, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agentID+"/heartbeat-decay", ownerToken, nil)
	if body["state"] != "dead" {
		t.Errorf("state: got %v, want dead", body["state"])
	}
}

// TestAgentHeartbeatDecay_CrossOwnerReject — acceptance §2.2.
func TestAgentHeartbeatDecay_CrossOwnerReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	_, agentID := seedAgentWithHeartbeat(t, ts, s, 5_000)
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agentID+"/heartbeat-decay", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-owner: got %d, want 403", resp.StatusCode)
	}
}

// TestAgentHeartbeatDecay_Unauthorized401 — acceptance §2.2.
func TestAgentHeartbeatDecay_Unauthorized401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/some-id/heartbeat-decay", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}
}

// TestAgentHeartbeatDecay_AgentNotFound404 — acceptance §2.2.
func TestAgentHeartbeatDecay_AgentNotFound404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/nonexistent/heartbeat-decay", ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not found: got %d, want 404", resp.StatusCode)
	}
}

// TestAgentHeartbeatDecay_NoRuntimeRowYieldsDead — acceptance §1.3 nil-safe.
// agent without agent_runtimes row → dead state (DeriveDecayState
// last=0 nil-safe behavior).
func TestAgentHeartbeatDecay_NoRuntimeRowYieldsDead(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	agentEmail := "agent-heartbeat-decay-no-rt@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentHeartbeatDecayNoRT",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	// Note: NO agent_runtimes row inserted.
	_, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agent.ID+"/heartbeat-decay", ownerToken, nil)
	if body["state"] != "dead" {
		t.Errorf("no runtime row: got %v, want dead (DeriveDecayState last=0 → dead)", body["state"])
	}
}
