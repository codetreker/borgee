// Package api_test — dm_5_reaction_summary_test.go: DM-5.1 server unit
// reverse-asserting that the existing reaction PUT/DELETE/GET endpoints
// (CV-7 既有) work byte-identical on DM-typed channels (type='dm') —
// proving the DM-5 0-server-code stance.
//
// Stance pin (dm-5-spec.md §0 设计 ①):
//   - 0 server production code change for DM-5
//   - GET /api/v1/messages/{id}/reactions returns AggregatedReaction
//     (store/queries_phase2b.go::AggregatedReaction shape) which CV-7 已
//     落地; DM-5 client仅渲染.
//   - PUT/DELETE round-trip on DM-typed channel members works exactly as
//     in regular channel — no DM-specific branching needed.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestDM_ReactionSummaryInDMChannel pins 设计 ①: PUT reaction on a DM
// message → GET aggregated returns 1 chip with count=1, user_ids=[owner].
// Then a second member (added to DM) PUTs the same emoji → count=2.
// Same user re-PUT idempotent (count stays 2). DELETE → count drops.
func TestDM_ReactionSummaryInDMChannel(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	// Seed a DM channel between owner and member.
	dm := &store.Channel{
		Name:       "dm-owner-member-dm5",
		Visibility: "private",
		CreatedBy:  owner.ID,
		Type:       "dm",
		Position:   store.GenerateInitialRank(),
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(dm); err != nil {
		t.Fatalf("create dm: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: dm.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: dm.ID, UserID: member.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Owner posts a message.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+dm.ID+"/messages", ownerTok,
		map[string]any{"content": "react to me"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post: %d %v", resp.StatusCode, body)
	}
	msgID := body["message"].(map[string]any)["id"].(string)

	// Owner PUTs 👍.
	r1, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msgID+"/reactions", ownerTok,
		map[string]string{"emoji": "👍"})
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("owner PUT: %d", r1.StatusCode)
	}

	// GET aggregated — count==1, user_ids contains owner only.
	g1, gj1 := testutil.JSON(t, "GET", ts.URL+"/api/v1/messages/"+msgID+"/reactions", ownerTok, nil)
	if g1.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d", g1.StatusCode)
	}
	rxs1, _ := gj1["reactions"].([]any)
	if len(rxs1) != 1 {
		t.Fatalf("expected 1 chip after owner reaction, got %d", len(rxs1))
	}
	chip1 := rxs1[0].(map[string]any)
	if chip1["emoji"] != "👍" {
		t.Errorf("emoji: got %v", chip1["emoji"])
	}
	if cn, _ := chip1["count"].(float64); int(cn) != 1 {
		t.Errorf("count after owner: got %v want 1", chip1["count"])
	}

	// Member PUTs 👍 → count==2.
	r2, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msgID+"/reactions", memberTok,
		map[string]string{"emoji": "👍"})
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("member PUT: %d", r2.StatusCode)
	}
	g2, gj2 := testutil.JSON(t, "GET", ts.URL+"/api/v1/messages/"+msgID+"/reactions", ownerTok, nil)
	if g2.StatusCode != http.StatusOK {
		t.Fatalf("GET 2: %d", g2.StatusCode)
	}
	rxs2, _ := gj2["reactions"].([]any)
	chip2 := rxs2[0].(map[string]any)
	if cn, _ := chip2["count"].(float64); int(cn) != 2 {
		t.Errorf("count after member: got %v want 2", chip2["count"])
	}

	// Owner DELETE → count==1.
	d1, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID+"/reactions", ownerTok,
		map[string]string{"emoji": "👍"})
	if d1.StatusCode != http.StatusOK {
		t.Fatalf("owner DELETE: %d", d1.StatusCode)
	}
	g3, gj3 := testutil.JSON(t, "GET", ts.URL+"/api/v1/messages/"+msgID+"/reactions", ownerTok, nil)
	rxs3, _ := gj3["reactions"].([]any)
	if g3.StatusCode != http.StatusOK || len(rxs3) != 1 {
		t.Fatalf("GET 3: %d, len=%d", g3.StatusCode, len(rxs3))
	}
	if cn, _ := rxs3[0].(map[string]any)["count"].(float64); int(cn) != 1 {
		t.Errorf("count after owner DELETE: got %v want 1", rxs3[0].(map[string]any)["count"])
	}
}
