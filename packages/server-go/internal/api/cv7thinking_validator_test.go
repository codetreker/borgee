// Package api_test — cv_7_thinking_validator_test.go: CV-7 thinking-subject
// 5-pattern reject 单测 (PUT /api/v1/messages/{id} edit path, 5-pattern 第 5 处链).
//
// Stance pins (cv-7-spec.md §0):
//   - ③ agent edit on content_type=='artifact_comment' must re-pass 5-pattern
//   - ② human edit not subject to validator
//   - ① 0 new endpoint — uses existing PUT /api/v1/messages/{id}
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestCV_AgentEditArtifactComment_ThinkingReject pins 设计 ③ integration:
// agent (role=='agent') edits a message with content_type=='artifact_comment'
// to a 5-pattern body → 400 `comment.thinking_subject_required` byte-identical
// (跟 CV-5 #530 同字符串).
func TestCV_AgentEditArtifactComment_ThinkingReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	chID := mustGeneralChannelID(t, s)

	agent := mustSeedAgentCV7(t, s, "cv7-agent@test.com", "AgentCV7")
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent to channel: %v", err)
	}
	agentTok := testutil.LoginAs(t, ts.URL, "cv7-agent@test.com", "password123")

	// Agent posts a valid comment-shaped message (content_type='artifact_comment').
	msg := &store.Message{
		ChannelID:   chID,
		SenderID:    agent.ID,
		Content:     "I propose tightening section 2 about lock TTLs.",
		ContentType: "artifact_comment",
		OrgID:       agent.OrgID,
	}
	if err := s.CreateMessage(msg); err != nil {
		t.Fatalf("seed msg: %v", err)
	}

	// Note: pattern 5 (empty/whitespace) is rejected earlier by the
	// existing "Content is required" guard (messages.go:313); the 4
	// non-empty patterns alone are the byte-identical lock-chain set
	// reachable through PUT (the empty case is a guard upstream).
	bodies := []string{
		"agent thinking",
		"defaultSubject placeholder leak",
		"wrapped fallbackSubject token",
		"AI is thinking...",
	}
	for _, b := range bodies {
		t.Run(b, func(t *testing.T) {
			resp, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msg.ID, agentTok, map[string]any{
				"content": b,
			})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 for body %q: got %d (%v)", b, resp.StatusCode, data)
			}
			if data["code"] != "comment.thinking_subject_required" {
				t.Errorf("error code byte-identical 锁失败: got %v want comment.thinking_subject_required", data["code"])
			}
		})
	}

	// Sanity: agent with concrete subject succeeds.
	ok, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msg.ID, agentTok, map[string]any{
		"content": "Section 2 tightening proposal v2.",
	})
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("valid agent edit rejected: %d", ok.StatusCode)
	}

	// 反约束: 非 artifact_comment 类型的 message 不走此 validator.
	textMsg := &store.Message{
		ChannelID:   chID,
		SenderID:    agent.ID,
		Content:     "plain chat",
		ContentType: "text",
		OrgID:       agent.OrgID,
	}
	if err := s.CreateMessage(textMsg); err != nil {
		t.Fatalf("seed text msg: %v", err)
	}
	resp2, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+textMsg.ID, agentTok, map[string]any{
		"content": "AI is thinking...",
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("non-comment-typed agent edit incorrectly rejected: %d", resp2.StatusCode)
	}
}

// TestCV_HumanEditArtifactComment_AnyBodyOK pins 设计 ② sanity: human-sender
// comment edit is NOT subject to validator (any body OK).
func TestCV_HumanEditArtifactComment_AnyBodyOK(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := mustGeneralChannelID(t, s)

	owner, _ := s.GetUserByEmail("owner@test.com")
	msg := &store.Message{
		ChannelID:   chID,
		SenderID:    owner.ID,
		Content:     "looks great",
		ContentType: "artifact_comment",
		OrgID:       owner.OrgID,
	}
	if err := s.CreateMessage(msg); err != nil {
		t.Fatalf("seed msg: %v", err)
	}

	// Human can edit to any body, even one that looks like a 5-pattern hit.
	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msg.ID, ownerTok, map[string]any{
		"content": "AI is thinking...",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("human edit rejected: %d", resp.StatusCode)
	}
}

// ---------- helpers ----------

func mustGeneralChannelID(t *testing.T, s *store.Store) string {
	t.Helper()
	var chID string
	if err := s.DB().Raw(`SELECT id FROM channels WHERE name = 'general' LIMIT 1`).Scan(&chID).Error; err != nil || chID == "" {
		t.Fatalf("seed general channel: %v / %q", err, chID)
	}
	return chID
}

func mustSeedAgentCV7(t *testing.T, s *store.Store, email, displayName string) *store.User {
	t.Helper()
	hash := mustHash(t, "password123")
	emailLocal := email
	agent := &store.User{
		DisplayName:  displayName,
		Role:         "agent",
		Email:        &emailLocal,
		PasswordHash: hash,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	owner, _ := s.GetUserByEmail("owner@test.com")
	if err := s.UpdateUser(agent.ID, map[string]any{"org_id": owner.OrgID}); err != nil {
		t.Fatalf("set agent org: %v", err)
	}
	if err := s.GrantDefaultPermissions(agent.ID, "member"); err != nil {
		t.Fatalf("grant agent perms: %v", err)
	}
	agent.OrgID = owner.OrgID
	return agent
}
