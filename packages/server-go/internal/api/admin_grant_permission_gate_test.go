// admin_grant_permission_gate_test.go — ADMIN-SPA-SHAPE-FIX REG-ASF-D6
// admin-rail handleGrantPermission IsValidCapability behavior test.
//
// Design: spec §0.3 + content-lock §1 — admin cURL with an arbitrary
// capability literal is rejected with 400 "invalid_capability". 4 validation cases:
//   1. valid dot-notation (channel.read 等 14 capability 之一) → 200 grant persisted
//   2. legacy snake_case (read_channel) → 400 invalid_capability (reject)
//   3. invalid custom literal (admin.god_mode 等) → 400
//   4. admin god-mode/bypass guard (permission check must use IsValidCapability)
//
// Cross-milestone coverage: CAPABILITY-DOT #628 protects backfilled data;
// this check protects the admin-rail entry point alongside user-rail validation
// in ap-2 / capability_grant / users / me_grants.

package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestADMSPASHAPE_REGASFD6_GrantPermission_ValidDot_200(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	user, _ := s.GetUserByEmail("member@test.com")
	if user == nil {
		t.Skip("missing fixture")
	}

	// dot-notation valid capability (CAPABILITY-DOT #628 14 const之一).
	resp, body := testutil.AdminJSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/users/"+user.ID+"/permissions",
		adminToken, map[string]any{"permission": "channel.read", "scope": "*"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for valid dot-notation, got %d: %v", resp.StatusCode, body)
	}
}

func TestADMSPASHAPE_REGASFD6_GrantPermission_LegacySnake_400(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	user, _ := s.GetUserByEmail("member@test.com")
	if user == nil {
		t.Skip("missing fixture")
	}

	// legacy snake_case (read_channel) — CAPABILITY-DOT #628 后已废, validation rejects it.
	resp, body := testutil.AdminJSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/users/"+user.ID+"/permissions",
		adminToken, map[string]any{"permission": "read_channel", "scope": "*"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for legacy snake_case, got %d: %v", resp.StatusCode, body)
	}
	if errMsg, _ := body["error"].(string); errMsg != "invalid_capability" {
		t.Errorf("expected error=invalid_capability, got %q", errMsg)
	}
}

func TestADMSPASHAPE_REGASFD6_GrantPermission_TypoFreestyle_400(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	user, _ := s.GetUserByEmail("member@test.com")
	if user == nil {
		t.Skip("missing fixture")
	}

	// invalid custom literal (admin.god_mode 不在 14 capability 名单).
	resp, body := testutil.AdminJSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/users/"+user.ID+"/permissions",
		adminToken, map[string]any{"permission": "admin.god_mode", "scope": "*"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid custom permission, got %d: %v", resp.StatusCode, body)
	}
}

func TestADMSPASHAPE_REGASFD6_GrantPermission_EmptyString_400(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	user, _ := s.GetUserByEmail("member@test.com")
	if user == nil {
		t.Skip("missing fixture")
	}

	// 空字符串走既有 "permission is required" 路径 (permission validation before capability lookup).
	resp, body := testutil.AdminJSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/users/"+user.ID+"/permissions",
		adminToken, map[string]any{"permission": "", "scope": "*"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty permission, got %d: %v", resp.StatusCode, body)
	}
}
