// covbump v6 — aggressive cov bump targeting 50-65% admin/server handlers.
// Add real tests with mock store / test predicate / nil-safe behavior.
// Targets: HB-5 retention override + chn_7 mute toggle + preview/thumbnail
// + more impersonation branches + pwa manifest fallback.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-WORKTREE-cov-v6 — HB-5 heartbeat-retention override branches
// (mirror of AL-7 audit-retention/override 7-branch test).
func TestHB_RetentionOverride(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", adminToken, "garbage")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid json: got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", adminToken,
		map[string]any{"retention_days": 0})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("0 days: got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", adminToken,
		map[string]any{"retention_days": 9999})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf(">365: got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", adminToken,
		map[string]any{"retention_days": 30})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("happy 30d: got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", adminToken,
		map[string]any{"retention_days": 90, "target_user_id": "test-user"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("happy 90d w/target: got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", "",
		map[string]any{"retention_days": 30})
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("no auth: got %d", resp.StatusCode)
	}
}

// REG-WORKTREE-cov-v6 — CHN-7 mute toggle full branches.
func TestChannelMute(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// 401 no auth.
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/some-id/mute", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("mute 401: got %d", resp.StatusCode)
	}
	// 404 unknown channel.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/nonexistent/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("mute 404: got %d", resp.StatusCode)
	}
	// Create a channel + mute happy + unmute happy.
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := &store.Channel{
		Name: "mute-target", Type: "channel", Visibility: "public",
		CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
		OrgID: owner.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	// Mute happy.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+ch.ID+"/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("mute happy: got %d", resp.StatusCode)
	}
	// Unmute happy.
	resp, _ = testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/"+ch.ID+"/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unmute happy: got %d", resp.StatusCode)
	}
}

// REG-WORKTREE-cov-v6 — capability_grant store helpers.
func TestStoreChannelGroupOps(t *testing.T) {
	t.Parallel()
	_, s, _ := testutil.NewTestServer(t)
	owner, _ := s.GetUserByEmail("owner@test.com")

	// Create + Get + Update + List + Delete a channel group.
	g := &store.ChannelGroup{
		Name:      "test-group-v6",
		Position:  "n",
		CreatedBy: owner.ID,
	}
	if err := s.CreateChannelGroup(g); err != nil {
		t.Fatalf("create group: %v", err)
	}
	got, err := s.GetChannelGroup(g.ID)
	if err != nil || got == nil {
		t.Fatalf("get group: %v", err)
	}
	if err := s.UpdateChannelGroup(g.ID, "renamed-v6"); err != nil {
		t.Fatalf("update group: %v", err)
	}
	groups, err := s.ListChannelGroups()
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) < 1 {
		t.Errorf("expected ≥1 group")
	}
	// UngroupChannels — empty path (no channels in group).
	ids, err := s.UngroupChannels(g.ID)
	if err != nil {
		t.Fatalf("ungroup empty: %v", err)
	}
	_ = ids
	// Delete + idempotent.
	if err := s.DeleteChannelGroup(g.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
}

// REG-WORKTREE-cov-v6 — audit-log filter branches in handleAdminAuditLog.
// (User-rail /api/v1/me/admin-actions branches were removed in #975 with
// the routes.)
func TestAdminActionsExtra(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	// audit-log: actor_id + target_user_id + action filters all together.
	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?actor_id=admin&target_user_id=usr&action=disable",
		adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("audit-log filters: got %d", resp.StatusCode)
	}
	// audit-log: archived=active explicit.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?archived=active", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("archived=active: got %d", resp.StatusCode)
	}
}
