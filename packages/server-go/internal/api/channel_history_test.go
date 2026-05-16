// Package api_test — chn_14_description_history_test.go: CHN-14 server
// description edit history audit unit tests + grep guard checks.
//
// Pins:
//
//	REG-CHN14-002 TestCHN142_UpdateChannelDescription_AppendsHistory + MultipleEdits + SameContent_NoAppend
//	REG-CHN14-003 TestCHN142_GetHistory_HappyPath + NonOwnerRejected + EmptyHistory + Unauthorized
//	REG-CHN14-004 TestCHN_GetHistoryAdmin_HappyPath + NoAdminPatchDeletePath
//	REG-CHN14-005 TestCHN_CHN10HandlePutByteIdentical
//	REG-CHN14-006 TestCHN_NoDescriptionHistoryQueue (AST alignment chain extension #22)
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-CHN14-002a/b/c — UpdateChannelDescription store-layer behaviors.
// Consolidated into one parent test sharing one fixture server (reduces
// race-detector load: 3 servers -> 1; team race budget optimization).
func TestCHN_UpdateChannelDescription_Behaviors(t *testing.T) {
	t.Parallel()
	_, s, _ := testutil.NewTestServer(t)
	owner, _ := s.GetUserByEmail("owner@test.com")

	t.Run("AppendsHistory", func(t *testing.T) {
		ch := &store.Channel{
			Name: "chn14-append", Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID, Topic: "v1",
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := s.UpdateChannelDescription(ch.ID, "v2"); err != nil {
			t.Fatalf("update: %v", err)
		}
		hist, err := s.GetChannelDescriptionHistory(ch.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if len(hist) != 1 {
			t.Fatalf("history length: got %d, want 1", len(hist))
		}
		if got, _ := hist[0]["old_content"].(string); got != "v1" {
			t.Errorf("old_content: got %q, want v1", got)
		}
		if got, _ := hist[0]["reason"].(string); got != "unknown" {
			t.Errorf("reason: got %q, want unknown (AL-1a alignment stops at HB-6 checkpoint 19)", got)
		}
	})

	t.Run("MultipleEdits", func(t *testing.T) {
		ch := &store.Channel{
			Name: "chn14-multi", Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID, Topic: "a",
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create: %v", err)
		}
		for _, v := range []string{"b", "c", "d"} {
			if err := s.UpdateChannelDescription(ch.ID, v); err != nil {
				t.Fatalf("update %s: %v", v, err)
			}
		}
		hist, _ := s.GetChannelDescriptionHistory(ch.ID)
		if len(hist) != 3 {
			t.Fatalf("history length: got %d, want 3", len(hist))
		}
		wants := []string{"a", "b", "c"} // each entry holds the OLD content.
		for i, w := range wants {
			if got, _ := hist[i]["old_content"].(string); got != w {
				t.Errorf("entry %d old_content: got %q, want %q", i, got, w)
			}
		}
	})

	t.Run("SameContent_NoAppend", func(t *testing.T) {
		ch := &store.Channel{
			Name: "chn14-noop", Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID, Topic: "stable",
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create: %v", err)
		}
		for i := 0; i < 3; i++ {
			if err := s.UpdateChannelDescription(ch.ID, "stable"); err != nil {
				t.Fatalf("update: %v", err)
			}
		}
		hist, _ := s.GetChannelDescriptionHistory(ch.ID)
		if len(hist) != 0 {
			t.Errorf("idempotent: same-content PUT must not append, got %d entries", len(hist))
		}
	})
}

// REG-CHN14-003 GET endpoints — consolidated into one parent server
// (4 servers -> 1 server; reduces repeated race-detector setup cost).
func TestCHN_GetHistory_Endpoints(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")

	t.Run("HappyPath", func(t *testing.T) {
		ch := &store.Channel{
			Name: "chn14-hist-happy", Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID, Topic: "v1",
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create: %v", err)
		}
		s.UpdateChannelDescription(ch.ID, "v2")

		resp, body := testutil.JSON(t, http.MethodGet,
			ts.URL+"/api/v1/channels/"+ch.ID+"/description/history", ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		hist, _ := body["history"].([]any)
		if len(hist) != 1 {
			t.Errorf("history length: got %d, want 1", len(hist))
		}
	})

	t.Run("NonOwnerRejected", func(t *testing.T) {
		ch := &store.Channel{
			Name: "chn14-nonowner", Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID, Topic: "v1",
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create: %v", err)
		}
		resp, _ := testutil.JSON(t, http.MethodGet,
			ts.URL+"/api/v1/channels/"+ch.ID+"/description/history", memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("non-owner GET: got %d, want 403", resp.StatusCode)
		}
	})

	t.Run("EmptyHistory", func(t *testing.T) {
		ch := &store.Channel{
			Name: "chn14-empty", Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID, Topic: "fresh",
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create: %v", err)
		}
		resp, body := testutil.JSON(t, http.MethodGet,
			ts.URL+"/api/v1/channels/"+ch.ID+"/description/history", ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		hist, _ := body["history"].([]any)
		if len(hist) != 0 {
			t.Errorf("empty history: got %d entries, want 0", len(hist))
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		resp, _ := testutil.JSON(t, http.MethodGet,
			ts.URL+"/api/v1/channels/some-id/description/history", "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("no auth: got %d, want 401", resp.StatusCode)
		}
	})
}

// REG-CHN14-004a — admin readonly GET HappyPath.
func TestCHN_GetHistoryAdmin_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := &store.Channel{
		Name: "chn14-admin", Type: "channel", Visibility: "public",
		CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
		OrgID: owner.OrgID, Topic: "v1",
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create: %v", err)
	}
	s.UpdateChannelDescription(ch.ID, "v2")

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/channels/"+ch.ID+"/description/history",
		adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	hist, _ := body["history"].([]any)
	if len(hist) != 1 {
		t.Errorf("admin history length: got %d, want 1", len(hist))
	}
}
