// Package api_test — cv_8_thread_validator_test.go: CV-8 thread reply
// validator unit tests (5-pattern 第 6 处链 + 1-level depth gate +
// reply target type gate).
//
// 设计约束 pins (cv-8-spec.md §0):
//   - 设计第 3 条 agent reply on artifact_comment must pass 5-pattern (4 sub-case)
//   - 设计第 4 条 depth 1 强制 — reply on reply rejected
//   - 设计第 4 条 reply target must be 'artifact_comment'
//   - 设计第 2 条 human reply skips validator (sanity)
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// helper: post a message via REST as `tok`, returning message id.
func cv8PostMsg(t *testing.T, url, tok, chID string, body map[string]any) (int, map[string]any) {
	t.Helper()
	resp, data := testutil.JSON(t, "POST", url+"/api/v1/channels/"+chID+"/messages", tok, body)
	return resp.StatusCode, data
}

// TestCV_HumanReplyOnComment_OK pins 设计第 2 条 sanity: human reply on an
// artifact_comment-typed parent → 201, parent.reply_to_id linkage written.
func TestCV_HumanReplyOnComment_OK(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := mustGeneralChannelIDCV8(t, s)

	parentStatus, parent := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{
		"content": "head comment", "content_type": "artifact_comment",
	})
	if parentStatus != http.StatusCreated {
		t.Fatalf("parent post: %d %v", parentStatus, parent)
	}
	parentID := parent["message"].(map[string]any)["id"].(string)

	replyStatus, reply := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{
		"content": "reply body", "content_type": "artifact_comment", "reply_to_id": parentID,
	})
	if replyStatus != http.StatusCreated {
		t.Fatalf("reply post: %d %v", replyStatus, reply)
	}
	rmsg := reply["message"].(map[string]any)
	if got, _ := rmsg["reply_to_id"].(string); got != parentID {
		t.Errorf("reply_to_id linkage lost: got %v, want %s", rmsg["reply_to_id"], parentID)
	}
}

// TestCV_AgentReplyThinking_Reject pins 设计第 3 条 4-pattern reject 字节级一致
// CV-5/CV-7 errcode (`comment.thinking_subject_required`).
func TestCV_AgentReplyThinking_Reject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := mustGeneralChannelIDCV8(t, s)

	// Owner seeds parent comment.
	pStatus, p := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{
		"content": "parent", "content_type": "artifact_comment",
	})
	if pStatus != http.StatusCreated {
		t.Fatalf("parent: %d %v", pStatus, p)
	}
	parentID := p["message"].(map[string]any)["id"].(string)

	// Seed agent in same org + channel.
	agent := mustSeedAgentCV8(t, s, "cv8-agent@test.com", "AgentCV8")
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	agentTok := testutil.LoginAs(t, ts.URL, "cv8-agent@test.com", "password123")

	bodies := []string{
		"agent thinking",
		"defaultSubject placeholder leak",
		"wrapped fallbackSubject token",
		"AI is thinking...",
	}
	for _, b := range bodies {
		t.Run(b, func(t *testing.T) {
			st, d := cv8PostMsg(t, ts.URL, agentTok, chID, map[string]any{
				"content": b, "content_type": "artifact_comment", "reply_to_id": parentID,
			})
			if st != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (%v)", st, d)
			}
			if d["code"] != "comment.thinking_subject_required" {
				t.Errorf("errcode 字节级一致 锁失败: got %v want comment.thinking_subject_required", d["code"])
			}
		})
	}

	// Sanity: agent with concrete subject succeeds.
	ok, _ := cv8PostMsg(t, ts.URL, agentTok, chID, map[string]any{
		"content": "Section 2 tightening proposal v2.", "content_type": "artifact_comment", "reply_to_id": parentID,
	})
	if ok != http.StatusCreated {
		t.Fatalf("valid agent reply rejected: %d", ok)
	}
}

// TestCV_ReplyOnReply_Reject pins 设计第 4 条: depth 2 → 400 thread_depth_exceeded.
func TestCV_ReplyOnReply_Reject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := mustGeneralChannelIDCV8(t, s)

	_, p := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{"content": "head", "content_type": "artifact_comment"})
	parentID := p["message"].(map[string]any)["id"].(string)
	_, r1 := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{
		"content": "reply 1", "content_type": "artifact_comment", "reply_to_id": parentID,
	})
	r1ID := r1["message"].(map[string]any)["id"].(string)

	st, d := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{
		"content": "reply on reply", "content_type": "artifact_comment", "reply_to_id": r1ID,
	})
	if st != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%v)", st, d)
	}
	if d["code"] != "comment.thread_depth_exceeded" {
		t.Errorf("errcode 字节级一致 锁失败: got %v want comment.thread_depth_exceeded", d["code"])
	}
}

// TestCV_ReplyOnNonComment_Reject pins 设计第 4 条: reply target 必须 artifact_comment 类型.
func TestCV_ReplyOnNonComment_Reject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := mustGeneralChannelIDCV8(t, s)

	// Seed a plain text message (default content_type='text').
	_, plain := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{"content": "plain chat"})
	plainID := plain["message"].(map[string]any)["id"].(string)

	st, d := cv8PostMsg(t, ts.URL, tok, chID, map[string]any{
		"content": "reply on plain", "content_type": "artifact_comment", "reply_to_id": plainID,
	})
	if st != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%v)", st, d)
	}
	if d["code"] != "comment.reply_target_invalid" {
		t.Errorf("errcode 字节级一致 锁失败: got %v want comment.reply_target_invalid", d["code"])
	}
}

// ---------- helpers ----------

func mustGeneralChannelIDCV8(t *testing.T, s *store.Store) string {
	t.Helper()
	var chID string
	if err := s.DB().Raw(`SELECT id FROM channels WHERE name = 'general' LIMIT 1`).Scan(&chID).Error; err != nil || chID == "" {
		t.Fatalf("seed general channel: %v / %q", err, chID)
	}
	return chID
}

func mustSeedAgentCV8(t *testing.T, s *store.Store, email, displayName string) *store.User {
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
