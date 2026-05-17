// Package api_test — cv_15_comment_edit_history_test.go: CV-15 acceptance.

package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// cv15SeedArtifactComment posts an artifact comment message and returns
// (channelID, messageID). Comment is sent by `owner@test.com`.
func cv15SeedArtifactComment(t *testing.T, tsURL, ownerTok string) (string, string) {
	t.Helper()
	chID := cv12General(t, tsURL, ownerTok)
	resp, body := testutil.JSON(t, "POST", tsURL+"/api/v1/channels/"+chID+"/messages", ownerTok,
		map[string]any{"content": "first version", "content_type": "artifact_comment"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed comment: %d %v", resp.StatusCode, body)
	}
	msg, _ := body["message"].(map[string]any)
	id, _ := msg["id"].(string)
	if id == "" {
		t.Fatalf("no message id: %v", body)
	}
	return chID, id
}

// TestCV_ErrCode_ExactLiterals verifies the 3 const literals.
func TestCV_ErrCode_ExactLiterals(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"NotArtifactComment": "comment.not_artifact_comment",
		"NotOwner":           "comment.not_owner",
		"MessageNotFound":    "comment.message_not_found",
	}
	got := map[string]string{
		"NotArtifactComment": api.CommentEditHistoryErrCodeNotArtifactComment,
		"NotOwner":           api.CommentEditHistoryErrCodeNotOwner,
		"MessageNotFound":    api.CommentEditHistoryErrCodeMessageNotFound,
	}
	for k, v := range cases {
		if got[k] != v {
			t.Errorf("CommentEditHistoryErrCode%s = %q, want %q", k, got[k], v)
		}
	}
}

// TestCV_GetUserHistory_HappyPath — sender owner-only happy.
func TestCV_GetUserHistory_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID, msgID := cv15SeedArtifactComment(t, ts.URL, tok)

	// Edit the comment via existing PUT to populate edit_history.
	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msgID, tok,
		map[string]any{"content": "edited version"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT edit: %d", resp.StatusCode)
	}

	// GET edit history — should contain 1 entry (old: "first version").
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/"+chID+"/messages/"+msgID+"/comment-edit-history", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET history: %d", resp.StatusCode)
	}
	hist, _ := body["history"].([]any)
	if len(hist) < 1 {
		t.Errorf("history len=%d, want ≥1", len(hist))
	}
	_ = s
}

// TestCV_NonSenderRejected — non-sender → 403.
func TestCV_NonSenderRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID, msgID := cv15SeedArtifactComment(t, ts.URL, ownerTok)

	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/"+chID+"/messages/"+msgID+"/comment-edit-history", memberTok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-sender: got %d, want 403", resp.StatusCode)
	}
	errStr, _ := body["error"].(string)
	if !strings.Contains(errStr, "comment.not_owner") {
		t.Errorf("error = %q, want comment.not_owner", errStr)
	}
}

// TestCV_NonArtifactCommentRejects404 — text message → 404
// `comment.not_artifact_comment`.
func TestCV_NonArtifactCommentRejects404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	// Post a normal text message (not artifact_comment).
	_, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", tok,
		map[string]any{"content": "plain text"})
	msg, _ := body["message"].(map[string]any)
	msgID, _ := msg["id"].(string)

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/"+chID+"/messages/"+msgID+"/comment-edit-history", tok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("non-artifact_comment: got %d, want 404", resp.StatusCode)
	}
	errStr, _ := body["error"].(string)
	if !strings.Contains(errStr, "comment.not_artifact_comment") {
		t.Errorf("error = %q, want comment.not_artifact_comment", errStr)
	}
}

// TestCV_EmptyHistory_ReturnsArray — empty edit_history → returns
// `history: []` (not nil).
func TestCV_EmptyHistory_ReturnsArray(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID, msgID := cv15SeedArtifactComment(t, ts.URL, tok)

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/"+chID+"/messages/"+msgID+"/comment-edit-history", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d", resp.StatusCode)
	}
	hist, ok := body["history"].([]any)
	if !ok {
		t.Fatalf("history not array: %v", body["history"])
	}
	if len(hist) != 0 {
		t.Errorf("empty edit_history: got len=%d, want 0", len(hist))
	}
}

// TestCV_Unauthorized401 — no auth → 401.
func TestCV_Unauthorized401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/whatever/messages/whatever/comment-edit-history", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no auth: got %d, want 401", resp.StatusCode)
	}
}

// TestCV_MessageNotFound404 — missing message → 404 comment.message_not_found.
func TestCV_MessageNotFound404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/channels/whatever/messages/non-existent-msg/comment-edit-history", tok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing msg: got %d, want 404", resp.StatusCode)
	}
	errStr, _ := body["error"].(string)
	if !strings.Contains(errStr, "comment.message_not_found") {
		t.Errorf("error = %q, want comment.message_not_found", errStr)
	}
}

// TestCV_GetAdminHistory_HappyPath — admin readonly happy.
func TestCV_GetAdminHistory_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	_, msgID := cv15SeedArtifactComment(t, ts.URL, ownerTok)

	adminTok := testutil.LoginAsAdmin(t, ts.URL)
	req, _ := http.NewRequest("GET", ts.URL+"/admin-api/v1/messages/"+msgID+"/comment-edit-history", nil)
	req.AddCookie(&http.Cookie{Name: "borgee_admin_session", Value: adminTok})
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("admin GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin GET: got %d, want 200", resp.StatusCode)
	}
}

// Sanity — store unused import suppress.
var _ = store.User{}
