// Package api_test — cv_12_search_namespace_test.go: CV-12.1 unit verifying
// the existing message-search endpoint works on artifact: namespace channels
// (CV-5 #530 namespace 单源 + messages.go::handleSearchMessages 既有 ACL).
//
// Stance pin (cv-12-spec.md §0 设计 ①):
//   - 0 server production code change
//   - existing GET /api/v1/channels/{channelId}/messages/search?q= covers
//     artifact-comment search byte-identical because comments are messages
//     (CV-5 collapse) and the search endpoint does not branch on
//     content_type.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestCV_SearchInArtifactNamespace pins 设计 ①: seed 3 messages with
// content_type='artifact_comment' in a private channel (acting as the
// artifact: namespace channel), search for a unique substring, expect
// the existing search endpoint to return the matching row — proves
// 0-server-code path works for artifact-comment search.
func TestCV_SearchInArtifactNamespace(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Create a private channel that simulates the artifact: namespace
	// channel (CV-5 设计 ①: name `artifact:<id>`, type 'artifact', private).
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := &store.Channel{
		Name:       "artifact:cv12-test-art",
		Visibility: "private",
		CreatedBy:  owner.ID,
		Type:       "artifact",
		Position:   "0|aaaaaa",
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Seed 3 comment-typed messages.
	bodies := []string{
		"first review note about lock TTL",
		"this contains the needle keyword for search",
		"third comment unrelated",
	}
	for _, b := range bodies {
		msg := &store.Message{
			ChannelID:   ch.ID,
			SenderID:    owner.ID,
			Content:     b,
			ContentType: "artifact_comment",
			OrgID:       owner.OrgID,
		}
		if err := s.CreateMessage(msg); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Search for "needle" — existing endpoint must return the 1 matching row.
	resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+ch.ID+"/messages/search?q=needle", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: %d %v", resp.StatusCode, data)
	}
	msgs, _ := data["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(msgs))
	}
	row := msgs[0].(map[string]any)
	if row["content"] != "this contains the needle keyword for search" {
		t.Errorf("hit body mismatch: %v", row["content"])
	}

	// Search for "absent" — 0 results.
	resp2, data2 := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+ch.ID+"/messages/search?q=absent-xyz", tok, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("empty search: %d %v", resp2.StatusCode, data2)
	}
	msgs2, _ := data2["messages"].([]any)
	if len(msgs2) != 0 {
		t.Errorf("expected 0 hits for absent term, got %d", len(msgs2))
	}
}
