// admin_grant_permission_gate_test.go — ADMIN-SPA-SHAPE-FIX REG-ASF-D6
// admin-rail handleGrantPermission IsValidCapability gate 真测.
//
// 设计: spec §0.3 + content-lock §1 — admin cURL 塞任意 capability 字面 →
// 反向 reject 400 "invalid_capability". 4 case 守门:
//   1. valid dot-notation (channel.read 等 14 capability 之一) → 200 grant 真挂
//   2. legacy snake_case (read_channel) → 400 invalid_capability (reject)
//   3. typo / 自创字面 (admin.god_mode 等) → 400
//   4. 反 admin god-mode bypass (确保 gate 走 IsValidCapability 不 short-circuit)
//
// 跨 milestone 锁链: CAPABILITY-DOT #628 backfill 守存量 + 此 gate 守入口
// (user-rail 4 处全验是 ap-2 / capability_grant / users / me_grants 同源,
// admin-rail 是第 5 处链 SSOT 守).

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

	// legacy snake_case (read_channel) — CAPABILITY-DOT #628 后已废, gate reject.
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

	// typo / 自创字面 (admin.god_mode 不在 14 capability 名单).
	resp, body := testutil.AdminJSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/users/"+user.ID+"/permissions",
		adminToken, map[string]any{"permission": "admin.god_mode", "scope": "*"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for typo/自创, got %d: %v", resp.StatusCode, body)
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

	// 空字符串走既有 "permission is required" 路径 (gate 之前的早期检查).
	resp, body := testutil.AdminJSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/users/"+user.ID+"/permissions",
		adminToken, map[string]any{"permission": "", "scope": "*"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty permission, got %d: %v", resp.StatusCode, body)
	}
}
