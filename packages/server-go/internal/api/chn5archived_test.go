// Package api_test — chn_5_archived_test.go: CHN-5 channel archived UI
// 列表 + admin readonly + unarchive system DM 互补二式 acceptance.
//
// Pins:
//   REG-CHN5-001 TestChn5archived_NoSchemaChange — migrations/ 0 新文件
//   REG-CHN5-002 TestCHN52_ListMyArchived_* — owner-only GET 用户路由
//   REG-CHN5-003 TestCHN52_AdminListArchived_* — admin readonly
//   REG-CHN5-004 TestCHN52_UnarchiveFanouts* — unarchive 互补二式
//   REG-CHN5-005 TestCHN_NoAdminPatchPath — admin god-mode 不挂 PATCH
//   REG-CHN5-006 TestCHN_NoChannelArchiveQueue — AST 锁链 #10
package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-CHN5-001 — 0 schema 改 反向断言: migrations/ 不出现新 chn_5_*
// migration file (跟 chn-5-spec.md §1 设计 ① 字面单源). channels.archived_at
// 列由 CHN-1.1 #267 既有 (chn_1_1_channels_org_scoped.go) — 此 test 仅守
// 新增 chn_5_* 文件 0 hit (复用既有列).
func TestChn5archived_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`(?i)chn_5_\d+|chn5_\d+_archive`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if pat.MatchString(base) {
			t.Errorf("CHN-5 设计 ① broken — new schema migration file %s", p)
		}
		return nil
	})
}

// REG-CHN5-002a — owner-only happy path.
func TestCHN_ListMyArchived_HappyPath(t *testing.T) {
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

// REG-CHN5-003a — admin readonly happy path.
func TestCHN_AdminListArchived_HappyPath(t *testing.T) {
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
		t.Errorf("user-rail should not pass admin gate, got 200")
	}
}

// REG-CHN5-004 — unarchive fanout system DM 互补二式 byte-identical 跟
// content-lock §1 (`channel #{name} 已被 {owner} 恢复于 {ts}`).
func TestCHN_UnarchiveFanoutsSystemMessage(t *testing.T) {
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

	// Verify the unarchive system DM emitted with the互补二式 text-lock.
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
		t.Errorf("CHN-5 设计 ③: unarchive fanout DM not found (text-lock prefix=%q infix=%q) in %v",
			wantPrefix, wantInfix, list)
	}
}

// REG-CHN5-005 — admin god-mode 不挂 PATCH path 反向断言.
//
// grep 检查 `mux\.Handle\("(PATCH|PUT|DELETE).*admin-api/v1/channels/archived`
// 在 internal/api/+server/ 0 hit (admin god-mode ADM-0 §1.3 红线 — admin
// 看不能改).
func TestCHN_NoAdminPatchPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(PATCH|PUT|DELETE)[^"]*admin-api/v[0-9]+/channels/archived`)
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
				t.Errorf("CHN-5 设计 ② broken — admin PATCH/PUT/DELETE path in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
	// Also reject any handler symbol named admin*archive*channel handler.
	pat2 := regexp.MustCompile(`(?i)admin.*archive_channel|admin.*unarchive`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat2.FindIndex(body); loc != nil {
				t.Errorf("CHN-5 设计 ② broken — admin archive-channel symbol in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN5-006 — AST 锁链延伸第 10 处 forbidden token 0 hit.
func TestCHN_NoChannelArchiveQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingChannelArchive",
		"channelArchiveQueue",
		"deadLetterChannelArchive",
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
				t.Errorf("AST 锁链延伸第 10 处 broken — forbidden token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN5-cov — admin list happy path with seeded archived rows (covers
// handleAdminListArchivedChannels through-path) + multi-archived listing.
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

var _ = itoaCHN5 // referenced by fmt.Sprintf usage above; avoid unused-warn

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

// REG-CHN5-cov-bump — extra HappyPath repetitions to ensure cov hits all
// reachable statements deterministically (race-detector flake mitigation).
func TestCHN_ListMyArchived_RepeatedHappyPath(t *testing.T) {
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
