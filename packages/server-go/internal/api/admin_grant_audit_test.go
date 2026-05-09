// Package api_test — admin_grant_audit_test.go: ADM-2-FOLLOWUP REG-010
// audit hook verification — POST /api/v1/me/impersonation-grant 后,
// admin_actions 表 contains row with action="impersonate.start" + actor=user.

package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// TestADM2FU_REG010_ImpersonateStartAuditHook 真测 grant 创建后 audit
// 写入 (REG-010 wire — handleCreateMyImpersonateGrant 加 InsertAdminAction).
func TestADM2FU_REG010_ImpersonateStartAuditHook(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	if owner == nil {
		t.Skip("missing owner fixture")
	}
	// Pre-condition: no admin_actions for owner.
	preActions, _ := s.ListAdminActionsForTargetUser(owner.ID, 100)
	preCount := len(preActions)

	// Trigger grant creation.
	r, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/me/impersonation-grant", tok, nil)
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("grant: %d body=%v", r.StatusCode, body)
	}

	// Post-condition: admin_actions has new row with action="impersonate.start"
	// + actor=owner.ID (业主自签 SSOT, 跟 spec 设计沿用).
	postActions, err := s.ListAdminActionsForTargetUser(owner.ID, 100)
	if err != nil {
		t.Fatalf("list admin_actions: %v", err)
	}
	if len(postActions) != preCount+1 {
		t.Errorf("expected +1 admin_actions row, got %d → %d",
			preCount, len(postActions))
	}
	found := false
	for _, a := range postActions {
		if a.Action == "start_impersonation" && a.ActorID == owner.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("REG-010 audit hook fired no row with action=start_impersonation + actor=%s", owner.ID)
	}
}
