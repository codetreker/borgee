// Package api_test — adm_2_2_endpoints_test.go: ADM-2.2 admin-rail audit log
// endpoint. User-rail (the three impersonation-grant GET/POST/DELETE routes
// + /api/v1/me/admin-actions list) was removed in #975 — those tests were
// deleted with the routes.
//
// 覆盖的验收项:
//   - §行为不变量 4.1.d — admin 之间互可见; user cookie 调 admin-api 401
//   - 设计 ⑤ forward-only — audit 不可改写 (CI grep 检查; 测试不直接打 SQL)
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// seedADM2 creates an admin_actions row directly via store helper for the
// given target user. Returns the id for assertion. Reused across cases to
// avoid repeating the wire-up.
func seedADM2(t *testing.T, s *store.Store, actorID, targetUserID, action string) string {
	t.Helper()
	id, err := s.InsertAdminAction(actorID, targetUserID, action, "")
	if err != nil {
		t.Fatalf("seed admin_action %s: %v", action, err)
	}
	return id
}

// TestADM_GetAdminAuditLog_FullVisibility 覆盖 acceptance 4.1.d — admin
// /admin-api/v1/audit-log 互可见 (所有 admin 行).
func TestADM_GetAdminAuditLog_FullVisibility(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	seedADM2(t, s, "admin-A", owner.ID, "delete_channel")
	seedADM2(t, s, "admin-B", member.ID, "suspend_user")

	resp, body := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit-log", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	actions := body["actions"].([]any)
	if len(actions) != 2 {
		t.Errorf("expected 2 rows (admin 互可见), got %d", len(actions))
	}
	// admin-rail must expose actor_id (互可见).
	for _, a := range actions {
		row := a.(map[string]any)
		if _, has := row["actor_id"]; !has {
			t.Error("admin-rail audit-log must expose actor_id (admin 互可见)")
		}
	}
}

// TestADM_GetAdminAuditLog_FilterByActor 覆盖 ?actor_id=foo filter
// (admin SPA UI 收敛, 不影响设计 ③ 互可见默认).
func TestADM_GetAdminAuditLog_FilterByActor(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	seedADM2(t, s, "admin-A", owner.ID, "delete_channel")
	seedADM2(t, s, "admin-B", owner.ID, "suspend_user")

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit-log?actor_id=admin-A", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	actions := body["actions"].([]any)
	if len(actions) != 1 {
		t.Errorf("filter actor=admin-A expected 1 row, got %d", len(actions))
	}
}

// TestADM_AdminAuditLog_UserCookieRejected 覆盖 REG-ADM0-002 共享底线 +
// 设计 ⑥ admin/user 二轨严格分离: user cookie 调 /admin-api/v1/audit-log → 401.
func TestADM_AdminAuditLog_UserCookieRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit-log", userToken, nil)
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("user cookie 调 /admin-api/v1/audit-log should reject 401/403, got %d", resp.StatusCode)
	}
}

// TestADM_AdminAuditLog_LimitParam covers parseLimit branches with valid
// integer + invalid input + clamp.
func TestADM_AdminAuditLog_LimitParam(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	for i := 0; i < 5; i++ {
		seedADM2(t, s, "admin-A", owner.ID, "delete_channel")
	}
	// limit=2 explicit.
	resp, body := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit-log?limit=2", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}
	if len(body["actions"].([]any)) != 2 {
		t.Errorf("limit=2 expected 2 rows, got %d", len(body["actions"].([]any)))
	}
	// limit invalid string → default 100; expect all 5.
	resp2, body2 := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit-log?limit=abc", adminToken, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatal(resp2.StatusCode)
	}
	if len(body2["actions"].([]any)) != 5 {
		t.Errorf("limit=abc default expected 5, got %d", len(body2["actions"].([]any)))
	}
	// limit > 500 → clamped.
	resp3, _ := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit-log?limit=999999", adminToken, nil)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("limit=999999 should clamp not error, got %d", resp3.StatusCode)
	}
	// limit=0 → default.
	resp4, _ := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/audit-log?limit=0", adminToken, nil)
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("limit=0 should default not error, got %d", resp4.StatusCode)
	}
}

// TestADM_AdminAuditLog_FilterByActionAndTarget covers ?action=
// + ?target_user_id= filters together.
func TestADM_AdminAuditLog_FilterByActionAndTarget(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	seedADM2(t, s, "admin-A", owner.ID, "delete_channel")
	seedADM2(t, s, "admin-A", member.ID, "delete_channel")
	seedADM2(t, s, "admin-A", owner.ID, "suspend_user")

	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/admin-api/v1/audit-log?action=delete_channel&target_user_id="+owner.ID,
		adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatal(resp.StatusCode)
	}
	rows := body["actions"].([]any)
	if len(rows) != 1 {
		t.Errorf("expected 1 row (delete_channel × owner), got %d", len(rows))
	}
}
