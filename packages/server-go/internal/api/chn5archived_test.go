// Package api_test — chn_5_archived_test.go: CHN-5 channel archived UI
// list behavior, admin readonly behavior, and unarchive system DM acceptance.
//
// Pins:
//
//	REG-CHN5-001 TestChn5archived_NoSchemaChange — migrations/ 0 新文件
//	REG-CHN5-002 TestCHN52_ListMyArchived_* — owner-only GET 用户路由
//	REG-CHN5-003 TestCHN52_AdminListArchived_* — admin readonly
//	REG-CHN5-004 TestCHN52_UnarchiveFanouts* — unarchive system DM notification
//	REG-CHN5-005 TestCHN_NoAdminPatchPath — admin API has no PATCH path
//	REG-CHN5-006 TestCHN_NoChannelArchiveQueue — no archive queue tokens
package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-CHN5-002a — owner-only success case.
func TestCHN_ListMyArchived_Success(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// 3 channels — archive 2, leave 1 active.
	for i, name := range []string{"arch-1", "arch-2", "active-1"} {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken, name, "public")
		if i < 2 {
			testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+ch["id"].(string), ownerToken,
				map[string]any{"archived": true})
		}
	}

	resp, body := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/me/archived-channels", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	list, _ := body["channels"].([]any)
	if len(list) != 2 {
		t.Errorf("expected 2 archived, got %d", len(list))
	}
	for _, raw := range list {
		ch := raw.(map[string]any)
		if ch["archived_at"] == nil {
			t.Errorf("listed channel missing archived_at: %v", ch)
		}
	}
}

// REG-CHN5-002b — empty list when no archived.
func TestCHN_ListMyArchived_EmptyList(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, body := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/me/archived-channels", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	list, _ := body["channels"].([]any)
	if len(list) != 0 {
		t.Errorf("expected 0 archived, got %d", len(list))
	}
}

// REG-CHN5-002c — unauthorized rejected.
func TestCHN_ListMyArchived_Unauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/me/archived-channels", "", nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 unauthenticated, got 200")
	}
}

// REG-CHN5-003a — admin readonly success case.
func TestCHN_AdminListArchived_Success(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "admin-archived", "public")
	testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+ch["id"].(string), ownerToken,
		map[string]any{"archived": true})

	resp, body := testutil.JSON(t, http.MethodGet, ts.URL+"/admin-api/v1/channels/archived", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	list, _ := body["channels"].([]any)
	if len(list) < 1 {
		t.Errorf("admin should see archived, got %d", len(list))
	}
}

// REG-CHN5-003b — user cookie hits admin path → 401/403.
func TestCHN_AdminListArchived_RejectsUserRail(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, http.MethodGet, ts.URL+"/admin-api/v1/channels/archived", userToken, nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("user token should not pass admin authorization check, got 200")
	}
}

// REG-CHN5-004 — unarchive system DM notification keeps the existing
// content-lock §1 format (`channel #{name} 已被 {owner} 恢复于 {ts}`).
func TestCHN_UnarchiveSystemMessageNotification(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "round-trip", "public")
	chID := ch["id"].(string)
	chName := ch["name"].(string)

	// archive then unarchive.
	testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID, ownerToken,
		map[string]any{"archived": true})
	resp, data := testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID, ownerToken,
		map[string]any{"archived": false})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unarchive PATCH: %d %v", resp.StatusCode, data)
	}
	updated, _ := data["channel"].(map[string]any)
	if updated["archived_at"] != nil {
		t.Errorf("expected archived_at nil after unarchive, got %v", updated["archived_at"])
	}

	// Verify the unarchive system DM emitted with the expected text format.
	resp, msgs := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+chID+"/messages?limit=10", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list messages: %d", resp.StatusCode)
	}
	list, _ := msgs["messages"].([]any)
	wantPrefix := "channel #" + chName + " 已被 "
	wantInfix := " 恢复于 "
	found := false
	for _, raw := range list {
		m, _ := raw.(map[string]any)
		c, _ := m["content"].(string)
		if strings.HasPrefix(c, wantPrefix) && strings.Contains(c, wantInfix) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("CHN-5 设计第 3 条: unarchive system DM not found (text-lock prefix=%q infix=%q) in %v",
			wantPrefix, wantInfix, list)
	}
}

// REG-CHN5-cov — admin list success case with seeded archived rows (covers
// handleAdminListArchivedChannels success path) + multi-archived listing.
func TestCHN_AdminListArchived_MultipleArchived(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	// Create + archive 2 channels via PUT with archived: true.
	for i := 0; i < 2; i++ {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken,
			fmt.Sprintf("adm-arch-%d", i), "public")
		chID := ch["id"].(string)
		testutil.JSON(t, http.MethodPut,
			ts.URL+"/api/v1/channels/"+chID, ownerToken,
			map[string]any{"archived": true})
	}
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/channels/archived", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin list: got %d", resp.StatusCode)
	}
	chs, _ := body["channels"].([]any)
	if len(chs) < 2 {
		t.Errorf("admin archived count: got %d, want >= 2", len(chs))
	}
}

// REG-CHN5-cov — list my archived after self-archive (covers full path).
func TestCHN_ListMyArchived_AfterArchive(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "my-arch-1", "public")
	chID := ch["id"].(string)
	testutil.JSON(t, http.MethodPut,
		ts.URL+"/api/v1/channels/"+chID, ownerToken,
		map[string]any{"archived": true})
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/me/archived-channels", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("my list: got %d", resp.StatusCode)
	}
	chs, _ := body["channels"].([]any)
	if len(chs) < 1 {
		t.Errorf("my archived count: got %d, want >= 1", len(chs))
	}
}

func itoaCHN5(i int) string {
	return fmt.Sprintf("%d", i)
}

var _ = itoaCHN5 // referenced by fmt.Sprintf usage above; keep compile-time reference

// REG-CHN5-cov — admin endpoint with no archived (covers 200 + empty list).
func TestCHN_AdminListArchived_NoArchived(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/channels/archived", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin list empty: got %d", resp.StatusCode)
	}
	chs, _ := body["channels"].([]any)
	if len(chs) != 0 {
		t.Errorf("admin archived empty: got %d, want 0", len(chs))
	}
}

// REG-CHN5-cov — direct admin user GET 401 (no admin token).
func TestCHN_AdminListArchived_NoToken(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/channels/archived", "", nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("admin no-token: got 200, expected non-200")
	}
}

// REG-CHN5-coverage — extra success repetitions to ensure coverage hits all
// reachable statements deterministically (race-detector flake mitigation).
func TestCHN_ListMyArchived_RepeatedSuccess(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	for i := 0; i < 3; i++ {
		ch := testutil.CreateChannel(t, ts.URL, ownerToken,
			fmt.Sprintf("rep-%d", i), "public")
		chID := ch["id"].(string)
		testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID, ownerToken,
			map[string]any{"archived": true})
	}
	for j := 0; j < 5; j++ {
		resp, body := testutil.JSON(t, http.MethodGet,
			ts.URL+"/api/v1/me/archived-channels", ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("iter %d: got %d", j, resp.StatusCode)
		}
		chs, _ := body["channels"].([]any)
		if len(chs) != 3 {
			t.Errorf("iter %d count: got %d, want 3", j, len(chs))
		}
	}
}

// TestCHN_ListMyArchived_StoreError covers the 500 error path —
// dropping the channels table makes the underlying SELECT fail, the
// handler logs + returns 500.
func TestCHN_ListMyArchived_StoreError(t *testing.T) {
	// 不能 t.Parallel — 我们破坏 store schema, 跟 NewTestServer 同 fresh DB.
	ts, store, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	store.DB().Exec(`PRAGMA foreign_keys = OFF`)
	if err := store.DB().Exec(`DROP TABLE channels`).Error; err != nil {
		t.Fatalf("drop channels: %v", err)
	}

	resp, _ := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/me/archived-channels", ownerToken, nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 on store error, got %d", resp.StatusCode)
	}
}

// TestCHN_AdminListArchived_StoreError covers admin handler 500 path
// (mirrors TestCHN_ListMyArchived_StoreError 模式).
func TestCHN_AdminListArchived_StoreError(t *testing.T) {
	ts, store, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	store.DB().Exec(`PRAGMA foreign_keys = OFF`)
	if err := store.DB().Exec(`DROP TABLE channels`).Error; err != nil {
		t.Fatalf("drop channels: %v", err)
	}

	resp, _ := testutil.JSON(t, http.MethodGet, ts.URL+"/admin-api/v1/channels/archived", adminToken, nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 on store error, got %d", resp.StatusCode)
	}
}
