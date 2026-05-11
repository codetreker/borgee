// Package api_test — cv_15_comment_edit_history_test.go: CV-15 acceptance.

package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

// TestCanvasEditHistory_NoSchemaChange — 0 schema 改反向检查.
func TestCanvasEditHistory_NoSchemaChange(t *testing.T) {
	t.Parallel()
	root := cv15RepoRoot(t)
	migDir := filepath.Join(root, "packages/server-go/internal/migrations")
	pat := regexp.MustCompile(`cv_15_\d+|CREATE TABLE.*artifact_comments|artifact_comment_history|ALTER TABLE artifact_comments`)
	hits := cv15GrepCount(t, migDir, pat)
	if hits != 0 {
		t.Errorf("expected 0 schema hit, got %d (设计 ① 0 schema 改)", hits)
	}
}

// TestCV_ReusesMessagesEditHistory — 复用 messages.edit_history 列
// (DM-7.1 v=34 既有, 不重新加).
func TestCV_ReusesMessagesEditHistory(t *testing.T) {
	t.Parallel()
	// Verify DM-7.1 migration still defines edit_history on messages.
	root := cv15RepoRoot(t)
	dm71 := filepath.Join(root, "packages/server-go/internal/migrations/messages_edit_history.go")
	if _, err := os.Stat(dm71); err != nil {
		t.Fatalf("dm_7_1 migration missing: %v", err)
	}
	b, _ := os.ReadFile(dm71)
	if !strings.Contains(string(b), "ALTER TABLE messages ADD COLUMN edit_history TEXT") {
		t.Error("DM-7.1 messages.edit_history column not found — CV-15 reuses this column")
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

// TestCV_NoAdminPatchDeletePath — admin-rail does NOT mount any
// PATCH/DELETE/PUT for comment-edit-history. Source scan.
func TestCV_NoAdminPatchDeletePath(t *testing.T) {
	t.Parallel()
	root := cv15RepoRoot(t)
	dir := filepath.Join(root, "packages/server-go/internal")
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)\s+/admin-api/v[0-9]+/[^"]*comment-edit-history`)
	hits := cv15GrepCount(t, dir, pat)
	if hits != 0 {
		t.Errorf("admin-rail PATCH/DELETE/PUT comment-edit-history: got %d, want 0 (admin 权限不挂 设计 ②)", hits)
	}
}

// cv15RepoRoot mirrors al_9 / dm_8 / chn_15 helpers.
func cv15RepoRoot(t *testing.T) string {
	t.Helper()
	abs, _ := filepath.Abs("../../../..")
	return abs
}

func cv15GrepCount(t *testing.T, dir string, re *regexp.Regexp) int {
	t.Helper()
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := info.Name()
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			return nil
		}
		b, ferr := os.ReadFile(path)
		if ferr != nil {
			return nil
		}
		count += len(re.FindAllIndex(b, -1))
		return nil
	})
	return count
}

// Sanity — store unused import suppress.
var _ = store.User{}
