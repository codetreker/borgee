// Package api_test — artifact_comments_branches_test.go: CV-5 branch
// coverage supplement (handleCreateComment + handleListComments
// uncovered branches拉满 84% threshold).
//
// Stance pins (跟 spec §0):
//   ① uncovered error paths 也是设计组成部分 — fail-closed branches 必测
//   ② 0 production code 改 — 仅加 test, 真测既有分支
//   ③ 复用既有 cv5Setup helper, 不另起 fixture

package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// TestArtifactComments_Create_Unauthorized — POST 无 token → 401.
func TestArtifactComments_Create_Unauthorized(t *testing.T) {
	t.Parallel()
	url, _, _, _, artID := cv5Setup(t)
	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", "",
		map[string]any{"body": "x"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestArtifactComments_List_Unauthorized — GET 无 token → 401.
func TestArtifactComments_List_Unauthorized(t *testing.T) {
	t.Parallel()
	url, _, _, _, artID := cv5Setup(t)
	resp, _ := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/comments", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestArtifactComments_Create_TargetNotFound — artifactId path empty
// hits MissingArtifactId 400 branch via 404 (artifact not found).
func TestArtifactComments_Create_NonexistentArtifact(t *testing.T) {
	t.Parallel()
	url, tok, _, _, _ := cv5Setup(t)
	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/no-such-id/comments", tok,
		map[string]any{"body": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestArtifactComments_List_NonexistentArtifact — same 404 branch.
func TestArtifactComments_List_NonexistentArtifact(t *testing.T) {
	t.Parallel()
	url, tok, _, _, _ := cv5Setup(t)
	resp, _ := testutil.JSON(t, "GET", url+"/api/v1/artifacts/no-such-id/comments", tok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestArtifactComments_Create_BadJSON — malformed body → 400.
func TestArtifactComments_Create_BadJSON(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv5Setup(t)
	// readJSON rejects non-object scalar — send a string body.
	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok, "not-an-object")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestArtifactComments_Create_EmptyBody — body="   " trimmed empty → 400.
func TestArtifactComments_Create_EmptyBody(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv5Setup(t)
	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok,
		map[string]any{"body": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %v", resp.StatusCode, data)
	}
}

// TestArtifactComments_List_EmptyBeforeAnyCreate — GET before any
// comment exists → 200 with empty list (gorm.ErrRecordNotFound branch
// in handleListComments).
func TestArtifactComments_List_EmptyBeforeAnyCreate(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv5Setup(t)
	resp, data := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/comments", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	comments, ok := data["comments"].([]any)
	if !ok {
		t.Fatalf("comments not an array: %v", data["comments"])
	}
	if len(comments) != 0 {
		t.Fatalf("expected empty list, got %d items", len(comments))
	}
}

// TestArtifactComments_Create_TwiceReusesChannel — second POST on the
// same artifact reuses the existing artifact:* namespace channel
// (exercises the early-return branch in ensureArtifactChannel where
// the lookup find existing row).
func TestArtifactComments_Create_TwiceReusesChannel(t *testing.T) {
	t.Parallel()
	url, tok, s, _, artID := cv5Setup(t)

	r1, d1 := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok,
		map[string]any{"body": "first"})
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("first post: %d %v", r1.StatusCode, d1)
	}
	r2, d2 := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok,
		map[string]any{"body": "second"})
	if r2.StatusCode != http.StatusCreated {
		t.Fatalf("second post: %d %v", r2.StatusCode, d2)
	}
	// Both posts should have landed in the same artifact: channel.
	if d1["channel_id"] != d2["channel_id"] {
		t.Fatalf("channel reuse broken: %v vs %v", d1["channel_id"], d2["channel_id"])
	}
	var count int64
	s.DB().Raw(`SELECT COUNT(*) FROM channels WHERE name = ?`, "artifact:"+artID).Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 artifact: channel row, got %d", count)
	}
}

// senderRoleFor lookup in handleListComments where sender is an agent
// (separate code path from owner — populates role="agent" in response).
func TestArtifactComments_List_AfterCreate_AgentRole(t *testing.T) {
	t.Parallel()
	url, tok, s, chID, artID := cv5Setup(t)
	agentTok := seedAgentInChannel(t, s, url, chID, "agent-listrole@test.com", "AgentR")

	// Agent posts a valid comment.
	if r, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", agentTok,
		map[string]any{"body": "Concrete subject v3."}); r.StatusCode != http.StatusCreated {
		t.Fatalf("agent post failed: %d", r.StatusCode)
	}
	// Owner also posts.
	if r, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/comments", tok,
		map[string]any{"body": "owner says ship"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("owner post failed: %d", r.StatusCode)
	}

	resp, data := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/comments", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	comments, _ := data["comments"].([]any)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	// Sanity: at least one entry has sender_role == "agent".
	sawAgent := false
	for _, c := range comments {
		m, _ := c.(map[string]any)
		if r, _ := m["sender_role"].(string); r == "agent" {
			sawAgent = true
		}
	}
	if !sawAgent {
		t.Errorf("expected at least one agent role in list response: %v", comments)
	}
}
