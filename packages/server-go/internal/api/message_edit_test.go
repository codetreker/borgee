// Package api_test — DM-4.1 server PATCH endpoint tests.
//
// 设计反查 (跟 dm-4-stance-checklist.md §1+§3+§4):
//   设计第 1 条 RT-3 既有 fan-out 复用 — events INSERT op="edit" 真写入
//   设计第 3 条 thinking 5-pattern 约束延伸第 3 处 — dm_4*.go grep 检查 0 hit
//   设计第 4 条 DM-only path — channel.Type != "dm" reject 403
//   设计第 5 条 owner-only ACL — sender != caller → 403
//
// 跨 milestone 字节级一致: events 表 op="edit" 复用既有 message_edited
// kind (跟 messages.go::handleUpdateMessage 同源, 不另起 dictionary);
// useDMSync (DM-3 #508) 客户端订阅 channel events backfill 自动多端 derive.

package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// dm4SetupOwnerAndDM seeds owner + agent + DM channel + one initial
// message from owner. Returns ownerToken, dmChannelID, messageID.
func dm4SetupOwnerAndDM(t *testing.T, ts *httptest.Server, s *store.Store) (string, string, string) {
	t.Helper()
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")

	agentEmail := "agent-dm4@test.com"
	agentRole := "agent"
	agent := &store.User{
		DisplayName: "AgentDM4",
		Role:        agentRole,
		Email:       &agentEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	dm := &store.Channel{
		Name:       "dm-owner-agentdm4",
		Visibility: "private",
		CreatedBy:  owner.ID,
		Type:       "dm",
		Position:   store.GenerateInitialRank(),
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(dm); err != nil {
		t.Fatalf("create dm: %v", err)
	}
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dm.ID, UserID: owner.ID})
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dm.ID, UserID: agent.ID})

	resp, body := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/channels/"+dm.ID+"/messages", ownerToken,
		map[string]any{"content": "original content", "content_type": "text"})
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("seed POST /messages: status %d body %v", resp.StatusCode, body)
	}
	msg, ok := body["message"].(map[string]any)
	if !ok {
		t.Fatalf("seed response missing message: %v", body)
	}
	messageID, _ := msg["id"].(string)
	if messageID == "" {
		t.Fatalf("seed messageID empty: %v", msg)
	}
	return ownerToken, dm.ID, messageID
}

// TestDM_HappyPath — acceptance §1.1 owner edits own message in DM,
// returns 200 + content updated.
func TestDM_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	resp, body := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, ownerToken,
		map[string]any{"content": "edited content via DM-4"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH happy: status %d body %v", resp.StatusCode, body)
	}
	msg, ok := body["message"].(map[string]any)
	if !ok {
		t.Fatalf("response missing message: %v", body)
	}
	if got, want := msg["content"], "edited content via DM-4"; got != want {
		t.Errorf("content: got %q, want %q", got, want)
	}
}

// TestDM_HappyPath_IdempotentSameContent — DM-7 后续 (G4.audit row #1):
// 反向断言 same-content PATCH 不追加 edit_history (保 idempotent).
// Phase 4 batch1 audit §2.1 mismatch closure.
func TestDM_HappyPath_IdempotentSameContent(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	// 1st PATCH (real edit: original → "same content") → appends 1 history row.
	for i := 0; i < 3; i++ {
		resp, _ := testutil.JSON(t, "PATCH",
			ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, ownerToken,
			map[string]any{"content": "same content"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH iter %d: status %d", i, resp.StatusCode)
		}
	}
	var msg store.Message
	if err := s.DB().Where("id = ?", messageID).First(&msg).Error; err != nil {
		t.Fatalf("reload msg: %v", err)
	}
	if msg.EditHistory == nil {
		t.Fatal("edit_history nil after 1st PATCH (expected 1 entry)")
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(*msg.EditHistory), &arr); err != nil {
		t.Fatalf("parse edit_history: %v", err)
	}
	// 1st PATCH appended 1 row (original → "same content"); 2nd/3rd PATCH
	// idempotent (same content) — must NOT append.
	if len(arr) != 1 {
		t.Errorf("edit_history length: got %d, want 1 (idempotent same-content PATCH)", len(arr))
	}
}

// TestDM_NonOwnerRejected — acceptance §1.1 设计第 5 条 owner-only ACL.
// member@test.com is added as DM member to keep channel ACL satisfied
// but is not the original sender, so PATCH must 403.
func TestDM_NonOwnerRejected(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	_, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	member, _ := s.GetUserByEmail("member@test.com")
	if member == nil {
		t.Fatalf("member@test.com seed missing")
	}
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dmID, UserID: member.ID})
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, memberToken,
		map[string]any{"content": "not allowed"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner: got status %d, want 403", resp.StatusCode)
	}
}

// TestDM_NonDMReject — acceptance §1.1 设计第 4 条 DM-only path.
// PATCH /api/v1/channels/{publicChannelId}/messages/{id} → 403
// `dm.edit_only_in_dm`.
func TestDM_NonDMReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")

	pub := &store.Channel{
		Name:       "general-dm4",
		Visibility: "public",
		CreatedBy:  owner.ID,
		Type:       "channel",
		Position:   store.GenerateInitialRank(),
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(pub); err != nil {
		t.Fatalf("create public channel: %v", err)
	}
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: pub.ID, UserID: owner.ID})

	resp, body := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/channels/"+pub.ID+"/messages", ownerToken,
		map[string]any{"content": "in public", "content_type": "text"})
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("POST public: %d %v", resp.StatusCode, body)
	}
	msg, _ := body["message"].(map[string]any)
	messageID, _ := msg["id"].(string)

	resp2, body2 := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+pub.ID+"/messages/"+messageID, ownerToken,
		map[string]any{"content": "edited"})
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("PATCH on public channel: got status %d, want 403 (dm.edit_only_in_dm)", resp2.StatusCode)
	}
	if msg, ok := body2["error"].(string); !ok || !strings.Contains(msg, "dm.edit_only_in_dm") {
		t.Errorf("expected error sentinel `dm.edit_only_in_dm`, got %v", body2)
	}
}

// TestDM_Unauthorized401 — acceptance §1.1 unauth → 401.
func TestDM_Unauthorized401(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	_, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, "",
		map[string]any{"content": "x"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got status %d, want 401", resp.StatusCode)
	}
}

// TestDM_NotFound404 — acceptance §1.1 message id mismatch → 404.
func TestDM_NotFound404(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, dmID, _ := dm4SetupOwnerAndDM(t, ts, s)

	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/nonexistent-id", ownerToken,
		map[string]any{"content": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("nonexistent message: got status %d, want 404", resp.StatusCode)
	}
}

// TestDM_NoThinkingPatternInBody — acceptance §1.3 设计第 3 条
// thinking subject 5-pattern 约束延伸第 3 处 (RT-3 第 1 + DM-3
// 第 2 + DM-4 第 3). dm_4*.go grep 检查 5 字面 0 hit (production
// .go 不含 thinking 状态字面).
func TestDM_NoThinkingPatternInBody(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		`"processing"`,
		`"responding"`,
		`"thinking"`,
		`"analyzing"`,
		`"planning"`,
	}
	dir := "../api"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	hits := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "dm_4") {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(b)
		for _, bad := range forbidden {
			if strings.Contains(content, bad) {
				hits = append(hits, path+":"+bad)
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("DM-4 设计第 3 条 broken: thinking 5-pattern literal in dm_4*.go production: %v", hits)
	}
}

// patchRaw fires a PATCH with arbitrary raw body (bypasses JSON marshal so
// tests can exercise invalid-JSON branches in handleEdit).
func patchRaw(t *testing.T, url, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("PATCH", url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "borgee_token", Value: token})
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp
}

// TestDM_ChannelNotFound — handleEdit step 3: GetChannelByID error/nil
// returns 404. Path uses a synthetic channel id that does not exist.
func TestDM_ChannelNotFound(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/nonexistent-channel/messages/some-msg", ownerToken,
		map[string]any{"content": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("channel not found: got status %d, want 404", resp.StatusCode)
	}
}

// TestDM_InvalidJSON — handleEdit step 4: malformed JSON body → 400.
func TestDM_InvalidJSON(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	resp := patchRaw(t,
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, ownerToken,
		"{not valid json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid json: got status %d, want 400", resp.StatusCode)
	}
}

// TestDM_EmptyContent — handleEdit step 4 trim path: whitespace-only
// content trims to empty → 400.
func TestDM_EmptyContent(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	resp, body := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, ownerToken,
		map[string]any{"content": "   \t\n  "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty content: got status %d, want 400 (body=%v)", resp.StatusCode, body)
	}
}

// TestDM_MessageInOtherChannel — handleEdit step 5: message exists but
// belongs to a different channel id than the path → 404.
func TestDM_MessageInOtherChannel(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken, dmID, _ := dm4SetupOwnerAndDM(t, ts, s)
	owner, _ := s.GetUserByEmail("owner@test.com")

	// Create a second DM (between owner and a fresh agent) and post a
	// message there. PATCH against {dmID, messageID-of-other-dm} → 404.
	otherEmail := "agent-other@test.com"
	otherRole := "agent"
	other := &store.User{
		DisplayName: "AgentOther",
		Role:        otherRole,
		Email:       &otherEmail,
		OrgID:       owner.OrgID,
		OwnerID:     &owner.ID,
	}
	if err := s.CreateUser(other); err != nil {
		t.Fatalf("create other agent: %v", err)
	}
	dm2 := &store.Channel{
		Name:       "dm-owner-agentother",
		Visibility: "private",
		CreatedBy:  owner.ID,
		Type:       "dm",
		Position:   store.GenerateInitialRank(),
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(dm2); err != nil {
		t.Fatalf("create dm2: %v", err)
	}
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dm2.ID, UserID: owner.ID})
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: dm2.ID, UserID: other.ID})

	respPost, postBody := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/channels/"+dm2.ID+"/messages", ownerToken,
		map[string]any{"content": "in dm2", "content_type": "text"})
	if respPost.StatusCode != http.StatusCreated && respPost.StatusCode != http.StatusOK {
		t.Fatalf("seed dm2 message: %d %v", respPost.StatusCode, postBody)
	}
	dm2Msg, _ := postBody["message"].(map[string]any)
	dm2MessageID, _ := dm2Msg["id"].(string)

	// Now PATCH against the FIRST dm path with the SECOND dm's message id.
	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+dm2MessageID, ownerToken,
		map[string]any{"content": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-channel message id: got status %d, want 404", resp.StatusCode)
	}
}

// TestDM_CrossOrg403 — handleEdit step 6: REG-INV-002 fail-closed
// cross-org reject. Foreign-org caller logs in, finds DM in their own org
// is impossible — so we instead place a message owned by a foreign-org
// user in the owner DM via direct store insert (CrossOrg branch).
func TestDM_CrossOrg403(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	_, dmID, messageID := dm4SetupOwnerAndDM(t, ts, s)

	// Mutate the seed message's OrgID to a different org so the owner caller
	// triggers the CrossOrg(user.OrgID, existing.OrgID) branch (step 6,
	// before owner-only ACL at step 7).
	foreign := testutil.SeedForeignOrgUser(t, s, "Foreign", "foreign-dm4@test.com")
	if err := s.DB().Model(&store.Message{}).
		Where("id = ?", messageID).
		Update("org_id", foreign.OrgID).Error; err != nil {
		t.Fatalf("mutate message org_id: %v", err)
	}

	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/api/v1/channels/"+dmID+"/messages/"+messageID, ownerToken,
		map[string]any{"content": "cross-org attempt"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-org: got status %d, want 403", resp.StatusCode)
	}
}
