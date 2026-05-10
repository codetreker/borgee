// Package api_test — me_grants_test.go: BPP-3.2.2 5 unit tests
// (acceptance §2.1-§2.5).
//
// Pins:
//   2.1 owner-only ACL — non-owner 403, no agent 404
//   2.2 capability MUST be in AP-1 auth.Capabilities (14 项 const)
//   2.3 scope MUST ∈ v1 三层 ({*, channel:<id>, artifact:<id>})
//   2.4 action="grant" → real GrantPermission write; reject/snooze → no-op
//   2.5 反向约束 grep — admin god-mode 不挂 /me/grants endpoint + scope 脱节出
package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/auth"
	"borgee-server/internal/testutil"
)

// REG-BPP32-006 (acceptance §2.4 happy + content-lock §2 action enum) —
// owner POST /me/grants action=grant 落 user_permissions 行.
func TestBPP_PostGrant_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	owner, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32-grant@test.com")
	ownerTok := testutil.LoginAs(t, ts.URL, *owner.Email, "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", ownerTok, map[string]any{
		"agent_id":   agent.ID,
		"capability": auth.CommitArtifact,
		"scope":      "artifact:art-1",
		"request_id": "req-grant-1",
		"action":     api.MeGrantsActionGrant,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%v", resp.StatusCode, body)
	}
	if got, _ := body["granted"].(bool); !got {
		t.Errorf("body.granted = %v, want true", body["granted"])
	}
	// Verify user_permissions row landed.
	perms, err := s.ListUserPermissions(agent.ID)
	if err != nil {
		t.Fatalf("list perms: %v", err)
	}
	found := false
	for _, p := range perms {
		if p.Permission == auth.CommitArtifact && p.Scope == "artifact:art-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GrantPermission row missing from user_permissions for agent=%q", agent.ID)
	}
}

// REG-BPP32-007 (acceptance §2.1) — owner-only ACL: non-owner → 403.
func TestBPP_PostGrant_NonOwner403(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	_, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32-acl@test.com")

	// member@test.com is not the agent's owner.
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", memberTok, map[string]any{
		"agent_id":   agent.ID,
		"capability": auth.CommitArtifact,
		"scope":      "*",
		"request_id": "r1",
		"action":     api.MeGrantsActionGrant,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner expected 403, got %d body=%v", resp.StatusCode, body)
	}
	if code, _ := body["error_code"].(string); code != api.MeGrantsErrCodeNotOwner {
		t.Errorf("error_code = %q, want %q", code, api.MeGrantsErrCodeNotOwner)
	}
}

// REG-BPP32-008 (acceptance §2.2) — capability 必 ∈ AP-1 auth.Capabilities;
// 字典外值 reject + bpp.grant_capability_disallowed 错码.
func TestBPP_PostGrant_CapabilityWhitelistGuard(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	owner, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32-cap@test.com")
	ownerTok := testutil.LoginAs(t, ts.URL, *owner.Email, "password123")

	for _, bad := range []string{
		"artifact.edit_content", // AP-1 rework 脱节 trap
		"workspace.create",      // 蓝图举例字面, 不在 14 const
		"foo_bar",
	} {
		resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", ownerTok, map[string]any{
			"agent_id":   agent.ID,
			"capability": bad,
			"scope":      "*",
			"request_id": "r-" + bad,
			"action":     api.MeGrantsActionGrant,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("capability=%q expected 400, got %d", bad, resp.StatusCode)
		}
		if code, _ := body["error_code"].(string); code != api.CapabilityGrantErrCodeCapabilityDisallowed {
			t.Errorf("capability=%q error_code = %q, want %q", bad, code, api.CapabilityGrantErrCodeCapabilityDisallowed)
		}
	}
}

// REG-BPP32-009 (acceptance §2.3) — scope ∈ v1 三层; 脱节值 reject.
func TestBPP_PostGrant_ScopeWhitelistGuard(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	owner, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32-scope@test.com")
	ownerTok := testutil.LoginAs(t, ts.URL, *owner.Email, "password123")

	// Valid scopes.
	for _, ok := range []string{"*", "channel:c1", "artifact:art-9"} {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", ownerTok, map[string]any{
			"agent_id":   agent.ID,
			"capability": auth.CommitArtifact,
			"scope":      ok,
			"request_id": "r-ok-" + ok,
			"action":     api.MeGrantsActionGrant,
		})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("scope=%q expected 200, got %d", ok, resp.StatusCode)
		}
	}
	// Invalid scopes — 脱节 outside v1 三层.
	for _, bad := range []string{"workspace:w1", "org:o1", "channel:", "artifact:", ""} {
		resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", ownerTok, map[string]any{
			"agent_id":   agent.ID,
			"capability": auth.CommitArtifact,
			"scope":      bad,
			"request_id": "r-bad-" + bad,
			"action":     api.MeGrantsActionGrant,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("scope=%q expected 400, got %d body=%v", bad, resp.StatusCode, body)
		}
	}
}

// REG-BPP32-010 (acceptance §2.5 + 反向约束 spec §3 #5/#6/#7) —
// reject + snooze v1 仅 audit (不持久化反向 grant). admin god-mode 不挂.
func TestBPP_PostGrant_RejectSnoozeAuditOnly(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	owner, agent := bpp32SeedOwnerAndAgent(t, s, "owner-bpp32-rs@test.com")
	ownerTok := testutil.LoginAs(t, ts.URL, *owner.Email, "password123")

	for _, action := range []string{api.MeGrantsActionReject, api.MeGrantsActionSnooze} {
		resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", ownerTok, map[string]any{
			"agent_id":   agent.ID,
			"capability": auth.CommitArtifact,
			"scope":      "*",
			"request_id": "r-" + action,
			"action":     action,
		})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("action=%q expected 200 (audit-only), got %d", action, resp.StatusCode)
		}
		if got, _ := body["granted"].(bool); got {
			t.Errorf("action=%q body.granted = true, want false (v1 audit-only)", action)
		}
	}
	// 反向约束: reject/snooze 不写 user_permissions 行 (audit only).
	perms, _ := s.ListUserPermissions(agent.ID)
	for _, p := range perms {
		if p.Permission == auth.CommitArtifact {
			t.Errorf("reject/snooze must NOT write GrantPermission row, found: %+v", p)
		}
	}

	// Action 字典外值 reject (3-enum).
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/me/grants", ownerTok, map[string]any{
		"agent_id":   agent.ID,
		"capability": auth.CommitArtifact,
		"scope":      "*",
		"request_id": "r-bad",
		"action":     "approve", // 同义词脱节禁 (content-lock §3 反向约束)
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("action='approve' expected 400 (3-enum strict), got %d body=%v", resp.StatusCode, body)
	}
}

// REG-BPP32-011 (反向约束 spec §3 #5+#6) — admin path 不挂 /me/grants
// (admin god-mode 走 /admin-api 单独 mw); + cross-org grant grep 检查.
func TestBPP_ReverseGrep_NoAdminPathAndNoCrossOrgGrant(t *testing.T) {
	t.Parallel()
	apiDir := filepath.Join("..", "api")
	// 反向约束: admin handler / mw 不出现 grant endpoint
	bad1 := regexp.MustCompile(`admin.*\/me\/grants|admin-api.*\/grants`)
	// 反向约束: cross-org grant via Scope (workspace: / org: 脱节出 v1 三层)
	bad2 := regexp.MustCompile(`Scope:\s*"workspace:|Scope:\s*"org:`)
	hits := []string{}
	_ = filepath.Walk(apiDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if bad1.Find(body) != nil || bad2.Find(body) != nil {
			hits = append(hits, p)
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("反向约束 spec §3 #5+#7 broken — admin /me/grants OR scope 脱节出 v1 三层 hit at: %v", hits)
	}
}
