// Package api_test — ap_1_2_artifacts_e2e_test.go: AP-1.2 end-to-end
// pinning ABAC capability gate on POST /api/v1/artifacts/{id}/commits.
//
// 蓝图: docs/blueprint/current/auth-permissions.md §1.2 三层 scope + §1.4 agent
// 严格. Spec: docs/implementation/modules/ap-1-spec.md (Phase 4 entry 8/8).
//
// Pins:
//   - REG-AP1-101: agent without commit_artifact grant → 403 + body BPP routing
//   - REG-AP1-102: agent with explicit (commit_artifact, artifact:<id>) → 200
//   - REG-AP1-103: agent with cross-artifact grant → 403 on target
//   - REG-AP1-104: human owner without explicit grant 仍 200 (wildcard, 立场 ④)
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func ap1SeedAgent(t *testing.T, s *store.Store, ts string, email, ownerEmail string, chID string) (string, string) {
	t.Helper()
	hash, _ := authHashHelper(t)
	agent := &store.User{
		DisplayName:  "AgentX",
		Role:         "agent",
		Email:        &email,
		PasswordHash: hash,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := s.UpdateUser(agent.ID, map[string]any{"org_id": mustOrgID(t, s, ownerEmail)}); err != nil {
		t.Fatalf("set agent org: %v", err)
	}
	if err := s.GrantDefaultPermissions(agent.ID, "agent"); err != nil {
		t.Fatalf("grant agent defaults: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent: %v", err)
	}
	tok := testutil.LoginAs(t, ts, email, "password123")
	return agent.ID, tok
}

func authHashHelper(t *testing.T) (string, error) {
	t.Helper()
	return mustHash(t, "password123"), nil
}

// REG-AP1-101 — agent without artifact-scope grant → 403 + body has
// required_capability + current_scope keys.
func TestAP_AgentNoGrant_403WithBPPRoutingHints(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	_, agentTok := ap1SeedAgent(t, s, ts.URL, "ap1-agent-101@test.com", "owner@test.com", chID)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "v1",
	})
	id := art["id"].(string)

	resp, parsed := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", agentTok, map[string]any{
		"expected_version": 1, "body": "v2-attempt",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 (agent no grant), got %d", resp.StatusCode)
	}
	if got, _ := parsed["required_capability"].(string); got != auth.CommitArtifact {
		t.Errorf("body.required_capability missing: %v", parsed)
	}
	if got, _ := parsed["current_scope"].(string); got != "artifact:"+id {
		t.Errorf("body.current_scope missing: %v", parsed)
	}
}

// REG-AP1-102 — agent with explicit grant for THIS artifact → 200.
func TestAP_AgentWithExplicitGrant_200(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	agentID, agentTok := ap1SeedAgent(t, s, ts.URL, "ap1-agent-102@test.com", "owner@test.com", chID)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "v1",
	})
	id := art["id"].(string)

	if err := s.GrantPermission(&store.UserPermission{
		UserID: agentID, Permission: auth.CommitArtifact, Scope: "artifact:" + id,
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", agentTok, map[string]any{
		"expected_version": 1, "body": "v2",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (agent with grant), got %d", resp.StatusCode)
	}
}

// REG-AP1-103 — agent with grant for art-other 仍 403 on art-target
// (cross-artifact strict立场 §1.4).
func TestAP_AgentCrossArtifactGrant_403(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	agentID, agentTok := ap1SeedAgent(t, s, ts.URL, "ap1-agent-103@test.com", "owner@test.com", chID)

	_, artOther := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Other", "body": "x",
	})
	otherID := artOther["id"].(string)
	_, artTarget := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Target", "body": "y",
	})
	targetID := artTarget["id"].(string)

	// Grant 限于 other.
	if err := s.GrantPermission(&store.UserPermission{
		UserID: agentID, Permission: auth.CommitArtifact, Scope: "artifact:" + otherID,
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	// Try to commit on target → 403.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+targetID+"/commits", agentTok, map[string]any{
		"expected_version": 1, "body": "v2-target",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 (cross-artifact), got %d", resp.StatusCode)
	}
}

// REG-AP1-104 — human owner without explicit per-artifact grant still
// passes via wildcard (*,*) — 立场 ④ 区分 agent/human.
func TestAP_HumanWildcardStillWorks_200(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "v1",
	})
	id := art["id"].(string)
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", ownerTok, map[string]any{
		"expected_version": 1, "body": "v2",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (human wildcard), got %d", resp.StatusCode)
	}
}
