// Package api_test — bpp_8_lifecycle_list_test.go: BPP-8.2 GET endpoint
// tests + AST scan AdminGodMode 反断.

package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestBPP_LifecycleList_HappyPath — acceptance §2.3.
func TestBPP_LifecycleList_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	agentEmail := "agent-bpp8@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentBPP8",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	// Seed 2 lifecycle rows — connect + cold_start.
	for _, action := range []string{"plugin_connect", "plugin_cold_start"} {
		if _, err := s.InsertAdminAction("system", agent.ID, action, `{"plugin_id":"p1"}`); err != nil {
			t.Fatalf("seed %s: %v", action, err)
		}
	}

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agent.ID+"/lifecycle", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("happy path: %d %v", resp.StatusCode, body)
	}
	events, ok := body["events"].([]any)
	if !ok || len(events) < 2 {
		t.Errorf("expected ≥2 events, got %v", body)
	}
}

// TestBPP_LifecycleList_CrossOwnerReject — acceptance §2.3.
func TestBPP_LifecycleList_CrossOwnerReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	owner, _ := s.GetUserByEmail("owner@test.com")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	agentEmail := "agent-bpp8b@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentBPP8b",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID, // owned by owner
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agent.ID+"/lifecycle", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-owner: got %d, want 403", resp.StatusCode)
	}
}

// TestBPP_LifecycleList_Unauthorized401 — acceptance §2.3.
func TestBPP_LifecycleList_Unauthorized401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/some-agent/lifecycle", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}
}

// TestBPP_LifecycleList_AgentNotFound404 — acceptance §2.3.
func TestBPP_LifecycleList_AgentNotFound404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/nonexistent-id/lifecycle", ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not found: got %d, want 404", resp.StatusCode)
	}
}

// TestBPP_LifecycleList_LimitClamp — limit query default/max bounds.
func TestBPP_LifecycleList_LimitClamp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want int
	}{
		{"", 100},
		{"0", 100},
		{"-5", 100},
		{"abc", 100},
		{"50", 50},
		{"500", 500},
		{"999", 500},
	}
	for _, tc := range cases {
		got := api.ClampBPP8LifecycleLimitForTest(tc.raw)
		if got != tc.want {
			t.Errorf("limit %q: got %d, want %d", tc.raw, got, tc.want)
		}
	}
}

// TestBPP_LifecycleList_NonPluginActionsExcluded — acceptance §3.2.
//
// Audit rows for the same agent_id with non-plugin_* action (e.g.
// 'permission_expired') must NOT appear in the lifecycle list response.
func TestBPP_LifecycleList_NonPluginActionsExcluded(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	agentEmail := "agent-bpp8c@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentBPP8c",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	// Seed 1 plugin_* + 1 permission_expired (must be filtered out).
	if _, err := s.InsertAdminAction("system", agent.ID, "plugin_connect", `{}`); err != nil {
		t.Fatalf("seed plugin_connect: %v", err)
	}
	if _, err := s.InsertAdminAction("system", agent.ID, "permission_expired", `{}`); err != nil {
		t.Fatalf("seed permission_expired: %v", err)
	}
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/agents/"+agent.ID+"/lifecycle", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("happy: %d %v", resp.StatusCode, body)
	}
	events, _ := body["events"].([]any)
	if len(events) != 1 {
		t.Errorf("expected 1 plugin_* event (permission_expired excluded), got %d: %v", len(events), events)
	}
}

// TestBPP_NoAdminLifecyclePath — acceptance §3.2 设计 ⑦ ADM-0 §1.3 红线.
func TestBPP_NoAdminLifecyclePath(t *testing.T) {
	t.Parallel()
	dir := "../api"
	// dir is relative to this test file location (internal/api/).
	if _, err := os.Stat(dir); err != nil {
		dir = "."
	}
	literals := []string{
		"admin/agents/lifecycle",
		"admin/plugins/lifecycle",
		"AdminPluginLifecycle",
		"AdminBPP8",
	}
	hits := []string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
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
		t.Errorf("BPP-8 设计 ⑦ broken — admin god-mode references plugin lifecycle (ADM-0 §1.3 红线): %v", hits)
	}
}
