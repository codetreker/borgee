package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func TestEveryoneFanoutComputedServerSide(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner := mustUserByEmail(t, s, "owner@test.com")
	member := mustUserByEmail(t, s, "member@test.com")

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "everyone-api-room", "private")
	chID := ch["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: member.ID}); err != nil {
		t.Fatalf("add human member: %v", err)
	}
	agent := testutil.CreateAgent(t, ts.URL, ownerToken, "Everyone Agent")
	agentID := agent["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agentID}); err != nil {
		t.Fatalf("add agent member: %v", err)
	}
	foreign := testutil.SeedForeignOrgUser(t, s, "Everyone Stranger", "everyone-stranger@test.com")

	msg := testutil.PostMessage(t, ts.URL, ownerToken, chID, "hello @Everyone")
	msgID := msg["id"].(string)

	for _, targetID := range []string{member.ID, agentID} {
		if got := countMessageMentionRows(t, s, msgID, targetID); got != 1 {
			t.Fatalf("message_mentions rows for %s = %d, want 1", targetID, got)
		}
	}
	for _, nonTargetID := range []string{owner.ID, foreign.ID} {
		if got := countMessageMentionRows(t, s, msgID, nonTargetID); got != 0 {
			t.Fatalf("message_mentions rows for non-target %s = %d, want 0", nonTargetID, got)
		}
	}
}

func TestEveryoneFanoutRejectsClientSuppliedRecipientIDs(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	member := mustUserByEmail(t, s, "member@test.com")
	chID := testutil.GetGeneralChannelID(t, ts.URL, ownerToken)

	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/messages", ownerToken, map[string]any{
		"content":  "client tries to choose recipients",
		"mentions": []string{member.ID},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("client recipient ids status %d body=%v, want 400", resp.StatusCode, data)
	}
}

func TestEveryoneFanoutRateLimitAndAgentLoopGuard(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "everyone-rate-room", "public")
	chID := ch["id"].(string)
	agent := testutil.CreateAgent(t, ts.URL, ownerToken, "Everyone Loop Agent")
	agentID := agent["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agentID}); err != nil {
		t.Fatalf("add agent member: %v", err)
	}

	first := testutil.PostMessage(t, ts.URL, ownerToken, chID, "first @Everyone")
	if first["id"] == "" {
		t.Fatal("first @Everyone message missing id")
	}
	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/messages", ownerToken, map[string]any{
		"content": "second @Everyone in same window",
	})
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second @Everyone status %d body=%v, want 429", resp.StatusCode, data)
	}

	agentToken := agent["api_key"].(string)
	resp, data = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/messages", agentToken, map[string]any{
		"content": "agent loop @Everyone",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("agent @Everyone status %d body=%v, want 400", resp.StatusCode, data)
	}
}

func countMessageMentionRows(t *testing.T, s *store.Store, messageID, targetUserID string) int64 {
	t.Helper()
	var count int64
	if err := s.DB().Raw(`SELECT COUNT(*) FROM message_mentions WHERE message_id = ? AND target_user_id = ?`, messageID, targetUserID).Scan(&count).Error; err != nil {
		t.Fatalf("count message_mentions: %v", err)
	}
	return count
}
