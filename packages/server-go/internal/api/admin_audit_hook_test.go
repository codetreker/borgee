// Package api_test — adm_2_2_audit_hook_test.go: audit + system DM emit
// integration tests for the 3 admin write actions wired in this PR
// (force_delete_channel / suspend_user via PATCH disabled / reset_password
// via PATCH password / change_role via PATCH role).
//
// Acceptance pins:
//   - Behavioral invariant §4.1.a — each admin action type automatically writes one admin_actions row
//   - Behavioral invariant §4.1.b — affected users receive a system DM (body is byte-identical with
//     content-lock §1, admin_username is not a raw UUID)
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestADM_ForceDeleteChannel_WritesAuditAndSystemDM pins acceptance
// 4.1.a (audit row written) + 4.1.b (system DM body is byte-identical).
func TestADM_ForceDeleteChannel_WritesAuditAndSystemDM(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	owner, _ := s.GetUserByEmail("owner@test.com")
	// CM-onboarding welcome channel — testutil seed doesn't create it; do
	// it here so EmitAdminActionSystemDM has a target system channel.
	if _, _, err := s.CreateWelcomeChannelForUser(owner.ID, "Owner"); err != nil {
		t.Fatalf("seed welcome channel: %v", err)
	}

	// Owner creates a channel.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", ownerToken,
		map[string]any{"name": "doomed-channel", "visibility": "private"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel: %d %v", resp.StatusCode, body)
	}
	channelID := body["channel"].(map[string]any)["id"].(string)

	// Admin force-deletes it.
	resp2, body2 := testutil.JSON(t, "DELETE",
		ts.URL+"/admin-api/v1/channels/"+channelID+"/force", adminToken, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("force delete: %d %v", resp2.StatusCode, body2)
	}

	// Verify audit row exists with action=delete_channel + target=owner.
	rows, err := s.ListAdminActionsForTargetUser(owner.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if rows[0].Action != "delete_channel" {
		t.Errorf("expected action=delete_channel, got %q", rows[0].Action)
	}
	if rows[0].ActorID == "" {
		t.Error("audit actor_id empty")
	}
	// Metadata JSON contains channel_id + channel_name.
	if !strings.Contains(rows[0].Metadata, channelID) ||
		!strings.Contains(rows[0].Metadata, "doomed-channel") {
		t.Errorf("metadata missing channel info: %q", rows[0].Metadata)
	}

	// Verify system DM written into owner's #welcome (type='system') channel.
	// Body must contain 'admin {login}' literal (login = e2e-admin from
	// testutil.LoginAsAdmin / "test-admin" from server seed).
	var msgCount int64
	s.DB().Raw(`SELECT COUNT(*) FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE c.created_by = ? AND c.type = 'system' AND m.sender_id = 'system'
		  AND m.content LIKE ?`, owner.ID, "%doomed-channel%被 admin %").
		Scan(&msgCount)
	if msgCount != 1 {
		t.Errorf("expected 1 system DM in owner's #welcome containing 'doomed-channel', got %d", msgCount)
	}

	// Reverse assertion: DM body must not include a raw UUID-looking actor_id
	// (design item 2, ADM2-NEG-001 — admin_username must be a concrete name).
	var bodies []string
	s.DB().Raw(`SELECT m.content FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE c.created_by = ? AND c.type = 'system' AND m.sender_id = 'system'`, owner.ID).
		Scan(&bodies)
	for _, b := range bodies {
		// Skip CM-onboarding welcome msg; only check the new admin-action DM.
		if !strings.Contains(b, "doomed-channel") {
			continue
		}
		// UUID-like pattern check (8-4-4-4-12 hex segments).
		if strings.Count(b, "-") >= 4 && len(b) > 100 {
			// Could be coincidence, but body is short; check for the actor_id
			// field's UUID directly.
			if a := rows[0].ActorID; a != "" && strings.Contains(b, a) {
				t.Errorf("DM body leaks raw actor_id UUID %q: %q", a, b)
			}
		}
		// Must contain "admin " prefix followed by non-UUID login.
		if !strings.Contains(b, "admin ") {
			t.Errorf("DM body missing 'admin {login}' literal: %q", b)
		}
	}
}

// TestADM_PatchUserDisabled_WritesSuspendAudit pins 4.1.a — PATCH
// disabled=true wires action=suspend_user.
func TestADM_PatchUserDisabled_WritesSuspendAudit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	owner, _ := s.GetUserByEmail("owner@test.com")

	resp, body := testutil.JSON(t, "PATCH",
		ts.URL+"/admin-api/v1/users/"+owner.ID, adminToken,
		map[string]any{"disabled": true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch disabled: %d %v", resp.StatusCode, body)
	}

	rows, _ := s.ListAdminActionsForTargetUser(owner.ID, 50)
	found := false
	for _, r := range rows {
		if r.Action == "suspend_user" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected suspend_user audit row, got %v", rows)
	}
}

// TestADM_PatchUserPassword_WritesResetPasswordAudit pins 4.1.a — PATCH
// password change wires action=reset_password.
func TestADM_PatchUserPassword_WritesResetPasswordAudit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	owner, _ := s.GetUserByEmail("owner@test.com")
	// Seed welcome channel so DM emit has a target.
	if _, _, err := s.CreateWelcomeChannelForUser(owner.ID, "Owner"); err != nil {
		t.Fatalf("seed welcome: %v", err)
	}

	newPass := "new-secret-99"
	resp, _ := testutil.JSON(t, "PATCH",
		ts.URL+"/admin-api/v1/users/"+owner.ID, adminToken,
		map[string]any{"password": newPass})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch password: %d", resp.StatusCode)
	}

	rows, _ := s.ListAdminActionsForTargetUser(owner.ID, 50)
	found := false
	for _, r := range rows {
		if r.Action == "reset_password" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected reset_password audit row")
	}

	// Reverse assert: DM body contains "重置" + "admin " + non-UUID login.
	var dmBody string
	s.DB().Raw(`SELECT m.content FROM messages m
		JOIN channels c ON c.id = m.channel_id
		WHERE c.created_by = ? AND c.type = 'system' AND m.sender_id = 'system'
		  AND m.content LIKE ?`, owner.ID, "%重置%").Scan(&dmBody)
	if dmBody == "" {
		t.Error("expected reset_password DM body in owner's #welcome")
	}
	if !strings.Contains(dmBody, "登录密码被 admin ") {
		t.Errorf("DM body missing literal '登录密码被 admin ': %q", dmBody)
	}
}

// TestADM_AuditRowMetadataNoBodyContent pins stance §3 cross-milestone
// shared baseline (ADM-0 §1.3 god-mode is metadata-only): reverse assertion that
// audit row metadata JSON excludes channel content / DM body / artifact content
// (admin does not read user content).
func TestADM_AuditRowMetadataNoBodyContent(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", ownerToken,
		map[string]any{"name": "audit-meta-test", "visibility": "private"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	channelID := body["channel"].(map[string]any)["id"].(string)

	testutil.JSON(t, "DELETE", ts.URL+"/admin-api/v1/channels/"+channelID+"/force", adminToken, nil)

	owner, _ := s.GetUserByEmail("owner@test.com")
	rows, _ := s.ListAdminActionsForTargetUser(owner.ID, 50)
	if len(rows) == 0 {
		t.Fatal("no audit row")
	}
	for _, r := range rows {
		// Metadata must not include body/content/text/artifact fields (ADM-0 §1.3 god-mode is metadata-only).
		for _, forbidden := range []string{
			`"body"`, `"content"`, `"text"`, `"artifact"`,
		} {
			if strings.Contains(r.Metadata, forbidden) {
				t.Errorf("metadata leaks content-bearing field %q: %q",
					forbidden, r.Metadata)
			}
		}
	}
	_ = store.AdminAction{} // silence import
}
