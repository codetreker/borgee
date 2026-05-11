// Package api_test — artifact_comments_test.go: CV-5 acceptance tests
// (canvas-vision §0 L24 字面 "Linear issue + comment").
//
// 设计约束 pins exercised (cv-5-spec.md §0):
//   - 设计第 1 条 comment 走 messages 表的单一来源 — POST 真创建 message row + virtual
//     `artifact:<id>` channel; 不开 artifact_comments 表.
//   - 设计第 2 条 frame envelope cursor 走 hub.cursors 共序 + body_preview 80 rune cap.
//   - 设计第 3 条 agent thinking subject 必带 — 5-pattern 约束第 4 处链.
//   - 设计第 4 条 cross-channel reject — 非 host channel member → 403
//     `comment.cross_channel_reject` (REG-INV-002 fail-closed).
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// cv5Setup builds server + creates artifact in `general`. Returns
// (url, ownerTok, store, channelID, artifactID).
func cv5Setup(t *testing.T) (string, string, *store.Store, string, string) {
	t.Helper()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "head body",
	})
	return ts.URL, ownerTok, s, chID, art["id"].(string)
}

// TestArtifactComments_HumanCreate_OK pins 设计第 1 条: human owner POSTs
// comment → 201, response carries comment id + sender_role='human' +
// channel_id under `artifact:` namespace.
func TestArtifactComments_HumanCreate_OK(t *testing.T) {
	t.Parallel()
	url, tok, s, _, artID := cv5Setup(t)

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok, map[string]any{
		"body": "looks great, ship it",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create comment: %d %v", resp.StatusCode, data)
	}
	if data["sender_role"] != "human" {
		t.Errorf("sender_role: got %v want human", data["sender_role"])
	}
	if data["body"] != "looks great, ship it" {
		t.Errorf("body roundtrip lost: %v", data["body"])
	}
	if id, _ := data["id"].(string); id == "" {
		t.Error("id missing in response")
	}
	// 设计第 1 条 — message row landed in messages table with content_type 'artifact_comment'.
	chID, _ := data["channel_id"].(string)
	var count int64
	s.DB().Raw(`SELECT COUNT(*) FROM messages WHERE channel_id = ? AND content_type = 'artifact_comment'`, chID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 message row in artifact: namespace channel, got %d", count)
	}
	// channel name must be `artifact:<artID>` (设计第 1 条 grep 检查项).
	var name string
	s.DB().Raw(`SELECT name FROM channels WHERE id = ?`, chID).Scan(&name)
	if want := "artifact:" + artID; name != want {
		t.Errorf("channel name 字节级一致 锁失败: got %q want %q", name, want)
	}
}

// TestArtifactComments_AgentThinkingSubject_Reject pins 设计第 3 条 5-pattern
// reverse-grep 第 4 处链. Each sub-case body matches exactly one pattern;
// server rejects with 400 + code 字节级一致.
func TestArtifactComments_AgentThinkingSubject_Reject(t *testing.T) {
	t.Parallel()
	url, _, s, chID, artID := cv5Setup(t)
	agentTok := seedAgentInChannel(t, s, url, chID, "agent-cv5@test.com", "AgentTinker")

	cases := []struct {
		name string
		body string
	}{
		{"empty", "   "},                     // pattern 5: subject="" 空
		{"thinking_suffix", "agent thinking"}, // pattern 1: trailing "thinking"
		{"defaultSubject", "defaultSubject placeholder leak"},
		{"fallbackSubject", "wrapped fallbackSubject token"},
		{"ai_is_thinking", "AI is thinking..."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", agentTok, map[string]any{
				"body": c.body,
			})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("agent thinking-subject body accepted: %d %v", resp.StatusCode, data)
			}
			if data["code"] != "comment.thinking_subject_required" {
				t.Errorf("error code 字节级一致 锁失败: got %v want comment.thinking_subject_required", data["code"])
			}
		})
	}
}

// TestArtifactComments_AgentValidSubject_OK pins 设计第 3 条 反向: agent body
// 带具体 subject (无 5-pattern hit) → 201 success.
func TestArtifactComments_AgentValidSubject_OK(t *testing.T) {
	t.Parallel()
	url, _, s, chID, artID := cv5Setup(t)
	agentTok := seedAgentInChannel(t, s, url, chID, "agent-ok@test.com", "AgentReady")

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", agentTok, map[string]any{
		"body": "I propose tightening section 2 about lock TTLs.",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("valid agent comment rejected: %d %v", resp.StatusCode, data)
	}
	if data["sender_role"] != "agent" {
		t.Errorf("sender_role: got %v want agent", data["sender_role"])
	}
}

// TestArtifactComments_CrossChannelReject pins 设计第 4 条 + REG-INV-002:
// non-member of host channel → 403 `comment.cross_channel_reject`.
func TestArtifactComments_CrossChannelReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	adminTok := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	_, ch := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", memberTok, map[string]string{
		"name": "private-cv5", "visibility": "private",
	})
	chID := ch["channel"].(map[string]any)["id"].(string)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", memberTok, map[string]any{
		"title": "P", "body": "x",
	})
	artID := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+artID+"/comments", adminTok, map[string]any{
		"body": "drive-by comment from non-member",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-channel non-member POST not 403: got %d %v", resp.StatusCode, data)
	}
	if data["code"] != "comment.cross_channel_reject" {
		t.Errorf("code 字节级一致 锁失败: got %v want comment.cross_channel_reject", data["code"])
	}
}

// TestArtifactComments_TargetNotFound pins 错误码: artifact_id 不存在 → 404
// `comment.target_artifact_not_found` (跟 DM-2.2 mention.target_not_in_channel 同模式).
func TestArtifactComments_TargetNotFound(t *testing.T) {
	t.Parallel()
	url, tok, _, _, _ := cv5Setup(t)
	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/nope-uuid/comments", tok, map[string]any{
		"body": "hello",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing artifact: %d %v", resp.StatusCode, data)
	}
	if data["code"] != "comment.target_artifact_not_found" {
		t.Errorf("code 字节级一致 锁失败: got %v", data["code"])
	}
}

// TestArtifactComments_BodyPreviewCap80Rune pins 设计第 2 条 隐私 §13 cap —
// 长 body 创建 OK, 服务端截断 body_preview 在推送路径; round-trip body
// 仍完整保留 (full body 走授权 channel-member 拉路径).
func TestArtifactComments_BodyPreviewCap80Rune(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv5Setup(t)
	long := strings.Repeat("界", 200)
	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok, map[string]any{
		"body": long,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("long body rejected: %d %v", resp.StatusCode, data)
	}
	if got, _ := data["body"].(string); got != long {
		t.Error("full body roundtrip lost (full body authorised path)")
	}
}

// TestArtifactComments_ListRoundTrip pins 设计第 1 条 + GET endpoint.
func TestArtifactComments_ListRoundTrip(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv5Setup(t)
	for _, body := range []string{"first", "second", "third"} {
		resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok, map[string]any{"body": body})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create: %d", resp.StatusCode)
		}
	}
	resp, list := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/comments", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d %v", resp.StatusCode, list)
	}
	rows, _ := list["comments"].([]any)
	if len(rows) != 3 {
		t.Errorf("list len: got %d want 3", len(rows))
	}
}
