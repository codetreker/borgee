// Package api_test — adm_2_2_endpoints_test.go: ADM-2.2 user-rail audit list
// + impersonate grant CRUD + admin-rail audit log endpoints.
//
// 覆盖的验收项:
//   - §行为不变量 4.1.c — user 只见自己 (跨业主 inject 防线: ?target_user_id 忽略)
//   - §行为不变量 4.1.d — admin 之间互可见; user cookie 调 admin-api 401
//   - §impersonate 红横幅 4.2.a — GET / POST / DELETE /me/impersonation-grant
//     语义 + 24h cooldown reject duplicate
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

// TestADM_GetMyAdminActions_ScopedToTargetUser 覆盖 acceptance 4.1.c — user
// 调 GET /me/admin-actions 只见自己 (target_user_id == current).
func TestADM_GetMyAdminActions_ScopedToTargetUser(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	// 3 actions targeting owner, 2 targeting member.
	for i := 0; i < 3; i++ {
		seedADM2(t, s, "admin-1", owner.ID, "delete_channel")
	}
	for i := 0; i < 2; i++ {
		seedADM2(t, s, "admin-1", member.ID, "suspend_user")
	}

	// Owner GET sees own 3 actions only.
	resp, body := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/admin-actions", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	actions := body["actions"].([]any)
	if len(actions) != 3 {
		t.Errorf("expected 3 actions for owner, got %d", len(actions))
	}
	for _, a := range actions {
		row := a.(map[string]any)
		if row["target_user_id"] != owner.ID {
			t.Errorf("leaked: target_user_id=%v (expect %s)", row["target_user_id"], owner.ID)
		}
		// user-rail must NOT expose actor_id raw (sanitizeAdminAction
		// admin_view=false omits actor_id).
		if _, has := row["actor_id"]; has {
			t.Error("user-rail GET should not expose raw actor_id (反向检查 ADM2-NEG-001)")
		}
	}
}

// TestADM_GetMyAdminActions_IgnoresTargetUserIDInjection 覆盖 acceptance
// §行为不变量 4.1.c 反向: ?target_user_id=other 参数被忽略, 只见自己.
func TestADM_GetMyAdminActions_IgnoresTargetUserIDInjection(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	member, _ := s.GetUserByEmail("member@test.com")
	owner, _ := s.GetUserByEmail("owner@test.com")

	// member 收 1 行 audit; owner 收 0 行.
	seedADM2(t, s, "admin-1", member.ID, "suspend_user")

	// Owner inject ?target_user_id=member.ID — server 必须忽略, 返 owner 自己 (0 行).
	resp, body := testutil.JSON(t, "GET",
		ts.URL+"/api/v1/me/admin-actions?target_user_id="+member.ID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	actions := body["actions"].([]any)
	if len(actions) != 0 {
		t.Errorf("inject ?target_user_id should be ignored — owner expected 0, got %d", len(actions))
	}
	_ = owner
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

// TestADM_GetMyImpersonateGrant_NoneReturnsNullGrant 覆盖 acceptance
// §4.2.a — 无 grant GET 返 200 + grant=null (client BannerImpersonate 使用此
// 决定不渲染红横幅).
func TestADM_GetMyImpersonateGrant_NoneReturnsNullGrant(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["grant"] != nil {
		t.Errorf("expected grant=null, got %v", body["grant"])
	}
}

// TestADM_PostImpersonateGrant_24hExpiry 覆盖 acceptance §4.2.a — POST 创
// 24h grant, expires_at - granted_at = 24h ms.
func TestADM_PostImpersonateGrant_24hExpiry(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	g := body["grant"].(map[string]any)
	granted := int64(g["granted_at"].(float64))
	expires := int64(g["expires_at"].(float64))
	if expires-granted != 24*60*60*1000 {
		t.Errorf("expires - granted = %d ms, expected 24h (%d ms)",
			expires-granted, 24*60*60*1000)
	}
	if g["revoked_at"] != nil {
		t.Error("new grant revoked_at should be null")
	}
}

// TestADM_PostImpersonateGrant_RejectsActiveDuplicate 验证设计 ⑦ 业主
// cooldown — 24h 期内 grant 已存在 → 409 grant_already_active.
func TestADM_PostImpersonateGrant_RejectsActiveDuplicate(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first POST expected 201, got %d", resp.StatusCode)
	}
	resp2, body2 := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("duplicate POST expected 409, got %d: %v", resp2.StatusCode, body2)
	}
}

// TestADM_DeleteImpersonateGrant_RevokesActiveGrant 覆盖 acceptance §4.2.a
// 业主撤销路径.
func TestADM_DeleteImpersonateGrant_RevokesActiveGrant(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// First grant.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("grant: %d", resp.StatusCode)
	}
	// Then revoke.
	resp2, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("revoke expected 204, got %d", resp2.StatusCode)
	}
	// GET should now return null.
	resp3, body3 := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp3.StatusCode != http.StatusOK {
		t.Fatal("get after revoke")
	}
	if body3["grant"] != nil {
		t.Errorf("after revoke, grant should be null, got %v", body3["grant"])
	}
	// Re-grant after revoke succeeds (cooldown released).
	resp4, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp4.StatusCode != http.StatusCreated {
		t.Errorf("re-grant after revoke expected 201, got %d", resp4.StatusCode)
	}
}

// TestADM_AdminActions_UserUnauthenticatedReturns401 验证 user-rail auth
// gate — 无 cookie GET 返 401.
func TestADM_AdminActions_UserUnauthenticatedReturns401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/admin-actions", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestADM_ImpersonateGrant_UnauthenticatedRejected covers 401 paths for
// 3 impersonation-grant endpoints.
func TestADM_ImpersonateGrant_UnauthenticatedRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp1, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/impersonation-grant", "", nil)
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET unauth expected 401, got %d", resp1.StatusCode)
	}
	resp2, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/impersonation-grant", "", nil)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST unauth expected 401, got %d", resp2.StatusCode)
	}
	resp3, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/me/impersonation-grant", "", nil)
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("DELETE unauth expected 401, got %d", resp3.StatusCode)
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

// TestADM_RevokeMyImpersonate_StoreError covers handleRevokeMyImpersonateGrant
// 500 path — dropping impersonation_grants forces store error.
func TestADM_RevokeMyImpersonate_StoreError(t *testing.T) {
	ts, store, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	store.DB().Exec(`PRAGMA foreign_keys = OFF`)
	if err := store.DB().Exec(`DROP TABLE impersonation_grants`).Error; err != nil {
		t.Fatalf("drop impersonation_grants: %v", err)
	}

	resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/me/impersonation-grant", token, nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 on store error, got %d", resp.StatusCode)
	}
}
