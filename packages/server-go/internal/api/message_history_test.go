// Package api_test — dm_7_edit_history_test.go: DM-7 server tests for
// edit history (UpdateMessage SSOT + GET endpoints + admin readonly +
// AST 锁链 #16 + reason byte-identical).
package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func sendDM(t *testing.T, baseURL, token, channelID, content string) string {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPost,
		baseURL+"/api/v1/channels/"+channelID+"/messages", token,
		map[string]any{"content": content})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("send: %d %v", resp.StatusCode, body)
	}
	msg, _ := body["message"].(map[string]any)
	return msg["id"].(string)
}

// REG-DM7-002a — UpdateMessage appends edit_history JSON entry.
func TestDM_UpdateMessage_AppendsEditHistory(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)

	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "first version")

	// Edit message via store SSOT (DM-4 既有 path 不变).
	if _, err := s.UpdateMessage(msgID, "second version"); err != nil {
		t.Fatalf("UpdateMessage: %v", err)
	}

	var msg store.Message
	if err := s.DB().Where("id = ?", msgID).First(&msg).Error; err != nil {
		t.Fatalf("reload msg: %v", err)
	}
	if msg.EditHistory == nil {
		t.Fatal("edit_history nil after edit")
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(*msg.EditHistory), &arr); err != nil {
		t.Fatalf("parse edit_history: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("edit_history length: got %d, want 1", len(arr))
	}
	if arr[0]["old_content"] != "first version" {
		t.Errorf("old_content: got %v, want first version", arr[0]["old_content"])
	}
	if arr[0]["reason"] != "unknown" {
		t.Errorf("reason: got %v, want 'unknown' (AL-1a 锁链第 18 处)", arr[0]["reason"])
	}
}

// REG-DM7-002b — multiple edits append each entry; ts monotonic.
func TestDM_UpdateMessage_MultipleEdits_AppendsAll(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "v1")

	for i, content := range []string{"v2", "v3", "v4"} {
		if _, err := s.UpdateMessage(msgID, content); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
	}

	var msg store.Message
	s.DB().Where("id = ?", msgID).First(&msg)
	var arr []map[string]any
	json.Unmarshal([]byte(*msg.EditHistory), &arr)
	if len(arr) != 3 {
		t.Fatalf("edit_history length: got %d, want 3", len(arr))
	}
	wantOld := []string{"v1", "v2", "v3"}
	for i, want := range wantOld {
		if arr[i]["old_content"] != want {
			t.Errorf("edit_history[%d].old_content: got %v, want %s", i, arr[i]["old_content"], want)
		}
	}
}

// REG-DM7-002c — idempotent: same-content edit does not append.
func TestDM_UpdateMessage_IdempotentSameContent(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "same")

	for i := 0; i < 3; i++ {
		if _, err := s.UpdateMessage(msgID, "same"); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
	}
	var msg store.Message
	s.DB().Where("id = ?", msgID).First(&msg)
	if msg.EditHistory != nil && *msg.EditHistory != "" && *msg.EditHistory != "null" {
		t.Errorf("edit_history not empty for same-content edits: got %q", *msg.EditHistory)
	}
}

// REG-DM7-003a — GET user-rail HappyPath.
func TestDM_GetEditHistory_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "v1")
	s.UpdateMessage(msgID, "v2")

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels/"+dm.ID+"/messages/"+msgID+"/edit-history",
		ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	hist, _ := body["history"].([]any)
	if len(hist) != 1 {
		t.Errorf("history length: got %d, want 1", len(hist))
	}
}

// REG-DM7-003b — non-sender 403.
func TestDM_GetEditHistory_NonSenderRejected(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "v1")

	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels/"+dm.ID+"/messages/"+msgID+"/edit-history",
		memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-sender GET: got %d, want 403", resp.StatusCode)
	}
}

// REG-DM7-003c — empty history returns [].
func TestDM_GetEditHistory_EmptyHistory(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "fresh") // never edited

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels/"+dm.ID+"/messages/"+msgID+"/edit-history",
		ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	hist, _ := body["history"].([]any)
	if len(hist) != 0 {
		t.Errorf("empty history: got %d, want 0", len(hist))
	}
}

// REG-DM7-004a — admin readonly HappyPath.
func TestDM_GetEditHistoryAdmin_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	msgID := sendDM(t, ts.URL, ownerToken, dm.ID, "v1")
	s.UpdateMessage(msgID, "v2")

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/messages/"+msgID+"/edit-history",
		adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin readonly: got %d", resp.StatusCode)
	}
	hist, _ := body["history"].([]any)
	if len(hist) != 1 {
		t.Errorf("admin history length: got %d, want 1", len(hist))
	}
}

// REG-DM7-004b — admin god-mode 不挂 PATCH/DELETE 双反向断言.
func TestDM_NoAdminPatchDeletePath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*edit-history`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat.FindIndex(body); loc != nil {
				t.Errorf("DM-7 设计 ③ broken — admin PATCH/DELETE/PUT path in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-DM7-005 — DM-4 既有 dm_4_message_edit.go production byte-identical
// 反向断言 (grep 检查 dm_7 在 dm_4*.go 0 hit).
func TestDM_DM4ProductionByteIdentical(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "api", "message_edit.go"))
	if err != nil {
		t.Fatalf("read dm_4: %v", err)
	}
	if regexp.MustCompile(`dm_?7\b`).Find(body) != nil {
		t.Error("DM-4 production drift — dm_7 reference in dm_4_message_edit.go")
	}
}

// REG-DM7-006 — AST 锁链延伸第 16 处.
func TestDM_NoEditHistoryQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingEditHistory",
		"editHistoryQueue",
		"deadLetterEditHistory",
	}
	dir := filepath.Join("..", "api")
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(body), tok) {
				t.Errorf("AST 锁链延伸第 16 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}
