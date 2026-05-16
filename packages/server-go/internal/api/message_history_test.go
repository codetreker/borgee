// Package api_test — dm_7_edit_history_test.go: DM-7 server tests for
// edit history (UpdateMessage 单一来源 + GET endpoints + admin readonly +
// AST 对齐链 #16 + reason byte-identical).
package api_test

import (
	"encoding/json"
	"net/http"
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

	// Edit message via store 单一来源 (DM-4 既有 path 不变).
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
		t.Errorf("reason: got %v, want 'unknown' (AL-1a 对齐链第 18 处)", arr[0]["reason"])
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
