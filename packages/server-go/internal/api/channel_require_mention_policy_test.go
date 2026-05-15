package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func TestChannelRequireMentionPolicyAPI(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner := mustUserByEmail(t, s, "owner@test.com")
	member := mustUserByEmail(t, s, "member@test.com")

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "policy-api-room", "public")
	chID := ch["id"].(string)
	agent := testutil.CreateAgent(t, ts.URL, ownerToken, "Policy API Agent")
	agentID := agent["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agentID}); err != nil {
		t.Fatalf("add agent member: %v", err)
	}

	resp, data := testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID+"/members/"+agentID+"/require-mention", ownerToken, map[string]string{"policy": "on"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manager set on: status %d body=%v", resp.StatusCode, data)
	}
	if data["require_mention_policy"] != "on" || data["effective_require_mention"] != true {
		t.Fatalf("set on response = %v, want policy on effective true", data)
	}
	resp, _ = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID+"/members/"+agentID+"/require-mention", ownerToken, map[string]string{"policy": "sometimes"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid policy status %d, want 400", resp.StatusCode)
	}

	// Default agents require mention globally; channel-level off would broaden
	// delivery and must fail without mutating the stored policy.
	resp, _ = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID+"/members/"+agentID+"/require-mention", ownerToken, map[string]string{"policy": "off"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("owner-ceiling off: status %d, want 400", resp.StatusCode)
	}
	state, err := s.GetChannelMemberRequireMentionState(chID, agentID)
	if err != nil {
		t.Fatalf("state after rejected off: %v", err)
	}
	if state.RequireMentionPolicy != "on" {
		t.Fatalf("rejected off mutated policy to %q, want on", state.RequireMentionPolicy)
	}

	if err := s.UpdateUser(agentID, map[string]any{"require_mention": false}); err != nil {
		t.Fatalf("set agent global require_mention false: %v", err)
	}
	resp, data = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID+"/members/"+agentID+"/require-mention", ownerToken, map[string]string{"policy": "off"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manager set off after opt-out: status %d body=%v", resp.StatusCode, data)
	}
	if data["require_mention_policy"] != "off" || data["effective_require_mention"] != false {
		t.Fatalf("set off response = %v, want policy off effective false", data)
	}

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+chID+"/members", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list members after policy update: status %d body=%v", resp.StatusCode, data)
	}
	members := data["members"].([]any)
	var agentMember map[string]any
	for _, raw := range members {
		m := raw.(map[string]any)
		if m["user_id"] == agentID {
			agentMember = m
			break
		}
	}
	if agentMember == nil {
		t.Fatalf("agent member not found in members response: %v", members)
	}
	if agentMember["require_mention_policy"] != "off" || agentMember["effective_require_mention"] != false {
		t.Fatalf("member policy state = %v, want off/effective false", agentMember)
	}

	if err := s.DB().Where("user_id = ?", member.ID).Delete(&store.UserPermission{}).Error; err != nil {
		t.Fatalf("remove member wildcard perms: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: member.ID}); err != nil {
		t.Fatalf("add human member: %v", err)
	}
	resp, _ = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID+"/members/"+agentID+"/require-mention", memberToken, map[string]string{"policy": "inherit"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-manager status %d, want 403", resp.StatusCode)
	}

	resp, _ = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID+"/members/"+owner.ID+"/require-mention", ownerToken, map[string]string{"policy": "on"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("human target status %d, want 400", resp.StatusCode)
	}
}

func TestChannelRequireMentionPolicyMessageIntegration(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner := mustUserByEmail(t, s, "owner@test.com")

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "policy-message-room", "public")
	chID := ch["id"].(string)
	agent := testutil.CreateAgent(t, ts.URL, ownerToken, "Policy Message Agent")
	agentID := agent["id"].(string)
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agentID}); err != nil {
		t.Fatalf("add agent member: %v", err)
	}

	testutil.PostMessage(t, ts.URL, ownerToken, chID, "plain message while inherit requires mention")
	if got := countSystemMessages(t, s); got != 0 {
		t.Fatalf("inherit/require mention generated %d implicit fallback messages, want 0", got)
	}

	if err := s.UpdateUser(agentID, map[string]any{"require_mention": false}); err != nil {
		t.Fatalf("set agent global require_mention false: %v", err)
	}
	if _, err := s.SetChannelMemberRequireMentionPolicy(chID, agentID, store.RequireMentionPolicyOff); err != nil {
		t.Fatalf("set channel policy off: %v", err)
	}
	msg := testutil.PostMessage(t, ts.URL, ownerToken, chID, "plain message after opt-out")
	if got := countSystemMessages(t, s); got != 1 {
		t.Fatalf("off policy generated %d fallback messages, want 1", got)
	}
	var mentionRows int64
	if err := s.DB().Raw(`SELECT COUNT(*) FROM message_mentions WHERE message_id = ? AND target_user_id = ?`, msg["id"], agentID).Scan(&mentionRows).Error; err != nil {
		t.Fatalf("count message_mentions: %v", err)
	}
	if mentionRows != 0 {
		t.Fatalf("implicit non-mention delivery wrote %d message_mentions rows, want 0", mentionRows)
	}

	if _, err := s.SetChannelMemberRequireMentionPolicy(chID, agentID, store.RequireMentionPolicyOn); err != nil {
		t.Fatalf("set channel policy on: %v", err)
	}
	explicit := testutil.PostMessage(t, ts.URL, ownerToken, chID, "explicit @"+agentID)
	if err := s.DB().Raw(`SELECT COUNT(*) FROM message_mentions WHERE message_id = ? AND target_user_id = ?`, explicit["id"], agentID).Scan(&mentionRows).Error; err != nil {
		t.Fatalf("count explicit message_mentions: %v", err)
	}
	if mentionRows != 1 {
		t.Fatalf("explicit mention rows = %d, want 1", mentionRows)
	}

	_ = owner
}

func mustUserByEmail(t *testing.T, s *store.Store, email string) *store.User {
	t.Helper()
	u, err := s.GetUserByEmail(email)
	if err != nil || u == nil {
		t.Fatalf("get user %s: %v", email, err)
	}
	return u
}

func countSystemMessages(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var count int64
	if err := s.DB().Raw(`SELECT COUNT(*) FROM messages WHERE sender_id = 'system'`).Scan(&count).Error; err != nil {
		t.Fatalf("count system messages: %v", err)
	}
	return count
}
