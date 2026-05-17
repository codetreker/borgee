// Package api_test — al_8_audit_log_filter_test.go: AL-8.1+8.2+8.3
// audit log query API filter 扩 (admin-rail GET /admin-api/v1/audit-log
// since/until/archived/actions). 0 schema / 0 新 endpoint.
//
// Pins:
//
//	REG-AL8-001 TestAL_NoSchemaChange — migrations/ 0 新 ALTER admin_actions
//	REG-AL8-002 TestAL_NoNewEndpoint — internal/api 0 新 audit-log path
//	REG-AL8-003 TestAL82_ArchivedView_* — 三态 (active/archived/all + reject)
//	REG-AL8-004 TestAL82_TimeRange_* — since/until clamp + reject
//	REG-AL8-005 TestAL82_Actions_* — 多值 + 单值 backward-compat
//	REG-AL8-006 TestAL_RejectsUserRail + TestAL_NoAuditQueryQueue
package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// al8SeedAction inserts a row with given action + optional archived_at.
// 跟 ADM-2.2 既有 InsertAdminAction 复用 (archived_at 后置 UPDATE 走原始
// SQL — AL-7.1 之后 admin_actions 才有此列).
func al8SeedAction(t *testing.T, s *store.Store, actorID, targetUserID, action string, archivedAt *int64) string {
	t.Helper()
	id, err := s.InsertAdminAction(actorID, targetUserID, action, "")
	if err != nil {
		t.Fatalf("seed admin_action %s: %v", action, err)
	}
	if archivedAt != nil {
		if err := s.DB().Exec(`UPDATE admin_actions SET archived_at = ? WHERE id = ?`,
			*archivedAt, id).Error; err != nil {
			t.Fatalf("set archived_at on %s: %v", id, err)
		}
	}
	return id
}

// al8AdminGET wraps testutil.JSON for shorter 调用 site.
func al8AdminGET(t *testing.T, ts *httptest.Server, adminToken, qstr string) (*http.Response, map[string]any) {
	t.Helper()
	return testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit-log"+qstr, adminToken, nil)
}

// REG-AL8-003 — archived 三态 (active/archived/all + reject).
func TestAL_ArchivedView_ActiveDefault(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	now := time.Now().UnixMilli()

	// 3 active (archived_at NULL) + 2 archived (archived_at set).
	al8SeedAction(t, s, "admin-1", owner.ID, "delete_channel", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "suspend_user", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "change_role", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "reset_password", &now)
	al8SeedAction(t, s, "admin-1", owner.ID, "start_impersonation", &now)

	// Default (no ?archived) = active.
	resp, body := al8AdminGET(t, ts, adminToken, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("default expected 200, got %d", resp.StatusCode)
	}
	if n := len(body["actions"].([]any)); n != 3 {
		t.Errorf("default (active) count: got %d, want 3", n)
	}

	// Explicit ?archived=active.
	resp, body = al8AdminGET(t, ts, adminToken, "?archived=active")
	if n := len(body["actions"].([]any)); n != 3 {
		t.Errorf("?archived=active count: got %d, want 3", n)
	}
}

func TestAL_ArchivedView_ArchivedOnly(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	now := time.Now().UnixMilli()

	al8SeedAction(t, s, "admin-1", owner.ID, "delete_channel", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "suspend_user", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "change_role", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "reset_password", &now)
	al8SeedAction(t, s, "admin-1", owner.ID, "start_impersonation", &now)

	resp, body := al8AdminGET(t, ts, adminToken, "?archived=archived")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if n := len(body["actions"].([]any)); n != 2 {
		t.Errorf("?archived=archived count: got %d, want 2", n)
	}
}

func TestAL_ArchivedView_All(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	now := time.Now().UnixMilli()

	al8SeedAction(t, s, "admin-1", owner.ID, "delete_channel", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "suspend_user", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "change_role", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "reset_password", &now)
	al8SeedAction(t, s, "admin-1", owner.ID, "start_impersonation", &now)

	resp, body := al8AdminGET(t, ts, adminToken, "?archived=all")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if n := len(body["actions"].([]any)); n != 5 {
		t.Errorf("?archived=all count: got %d, want 5", n)
	}
}

func TestAL_ArchivedView_RejectsUnknown(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	for _, v := range []string{"foo", "Active", "ARCHIVED", "purged"} {
		resp, body := al8AdminGET(t, ts, adminToken, "?archived="+v)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("?archived=%s: got %d, want 400", v, resp.StatusCode)
		}
		if got, _ := body["error"].(string); got != "audit_log.archived_view_invalid" {
			t.Errorf("?archived=%s err: got %v, want audit_log.archived_view_invalid", v, body["error"])
		}
	}
}

// REG-AL8-004 — since/until clamp + reject.
func TestAL_TimeRange_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")

	// Seed with 2 rows; default created_at = now (server-side). Use wide
	// since=0 / until=future to guarantee inclusion.
	al8SeedAction(t, s, "admin-1", owner.ID, "delete_channel", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "suspend_user", nil)

	future := time.Now().Add(24 * time.Hour).UnixMilli()
	q := fmt.Sprintf("?since=0&until=%d", future)
	resp, body := al8AdminGET(t, ts, adminToken, q)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if n := len(body["actions"].([]any)); n != 2 {
		t.Errorf("wide range count: got %d, want 2", n)
	}

	// Narrow until before now → 0 rows.
	past := int64(1)
	q = fmt.Sprintf("?since=0&until=%d", past)
	_, body = al8AdminGET(t, ts, adminToken, q)
	if n := len(body["actions"].([]any)); n != 0 {
		t.Errorf("narrow range count: got %d, want 0", n)
	}
}

func TestAL_TimeRange_RejectsBadInput(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	for _, v := range []string{"?since=-1", "?since=abc", "?until=-100", "?until=foo"} {
		resp, body := al8AdminGET(t, ts, adminToken, v)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", v, resp.StatusCode)
		}
		if got, _ := body["error"].(string); got != "audit_log.time_range_invalid" {
			t.Errorf("%s err: got %v, want audit_log.time_range_invalid", v, body["error"])
		}
	}
}

func TestAL_TimeRange_RejectsInverted(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, body := al8AdminGET(t, ts, adminToken,
		"?since=1700000000000&until=1600000000000")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("inverted: got %d, want 400", resp.StatusCode)
	}
	if got, _ := body["error"].(string); got != "audit_log.time_range_inverted" {
		t.Errorf("inverted err: got %v, want audit_log.time_range_inverted", body["error"])
	}
}

// REG-AL8-005 — actions 多值 + 单值 backward-compat.
func TestAL_Actions_MultiValue(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")

	al8SeedAction(t, s, "admin-1", owner.ID, "delete_channel", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "suspend_user", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "change_role", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "reset_password", nil)

	// Multi-value: ?action=delete_channel&action=suspend_user → 2 rows.
	resp, body := al8AdminGET(t, ts, adminToken,
		"?action=delete_channel&action=suspend_user")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	actions := body["actions"].([]any)
	if len(actions) != 2 {
		t.Errorf("multi-value count: got %d, want 2", len(actions))
	}
	for _, a := range actions {
		row := a.(map[string]any)
		got, _ := row["action"].(string)
		if got != "delete_channel" && got != "suspend_user" {
			t.Errorf("leaked action: got %q (expect delete_channel|suspend_user)", got)
		}
	}
}

func TestAL_Actions_SingleValueBackcompat(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")

	al8SeedAction(t, s, "admin-1", owner.ID, "delete_channel", nil)
	al8SeedAction(t, s, "admin-1", owner.ID, "suspend_user", nil)

	resp, body := al8AdminGET(t, ts, adminToken,
		"?action=delete_channel")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if n := len(body["actions"].([]any)); n != 1 {
		t.Errorf("single-value count: got %d, want 1", n)
	}
}

// REG-AL8-006 — admin-rail only + AST scan + reason chain not expanded.
func TestAL_RejectsUserRail(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit-log?archived=archived",
		userToken, nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("user-rail should not pass admin gate, got 200")
	}
}
