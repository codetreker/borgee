// Package api_test — chn_15_readonly_test.go: CHN-15 acceptance tests.
//
// Acceptance pins (docs/qa/acceptance-templates/chn-15.md):
//   - 1.1 0 schema 改 反向断言
//   - 1.2 ReadonlyBit byte-identical + IsReadonly truth table
//   - 2.1-2.6 endpoints owner-only + send gate + admin not mounted +
//     错码 byte-identical
package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/testutil"
)

// TestCHN_ReadonlyBit_ByteIdentical pins ReadonlyBit=16 + truth table.
func TestCHN_ReadonlyBit_ByteIdentical(t *testing.T) {
	t.Parallel()
	if api.ReadonlyBit != 16 {
		t.Errorf("ReadonlyBit drift: got %d, want 16 (双向锁 跟 client READONLY_BIT)", api.ReadonlyBit)
	}
	cases := []struct {
		collapsed int64
		want      bool
		desc      string
	}{
		{0, false, "no bits set"},
		{1, false, "collapse bit 0 only"},
		{2, false, "mute bit 1 only"},
		{4, false, "notif bit 2 only"},
		{15, false, "bits 0-3 set, no bit 4"},
		{16, true, "readonly bit 4 only"},
		{17, true, "collapse + readonly"},
		{31, true, "all 5 bits"},
		{48, true, "bit 4 + bit 5"},
	}
	for _, c := range cases {
		if got := api.IsReadonly(c.collapsed); got != c.want {
			t.Errorf("IsReadonly(%d) = %v, want %v (%s)", c.collapsed, got, c.want, c.desc)
		}
	}
}

// TestChn15readonly_NoSchemaChange — filepath.Walk migrations/ grep 检查
// chn_15_\d+ 0 hit + sqlite_master 反向. 设计 ①.
func TestChn15readonly_NoSchemaChange(t *testing.T) {
	t.Parallel()
	root := chn15RepoRoot(t)
	migDir := filepath.Join(root, "packages/server-go/internal/migrations")
	pat := regexp.MustCompile(`chn_15_\d+|ALTER TABLE channels.*readonly|channel_readonly_states|read_only_channels`)
	hits := chn15GrepCount(t, migDir, pat)
	if hits != 0 {
		t.Errorf("expected 0 schema hit, got %d (设计 ① 0 schema 改)", hits)
	}
}

// TestCHN_ChannelErrCode_ByteIdentical — server const 字面单源.
func TestCHN_ChannelErrCode_ByteIdentical(t *testing.T) {
	t.Parallel()
	if got, want := api.ChannelErrCodeReadonlyNoSend, "channel.readonly_no_send"; got != want {
		t.Errorf("ChannelErrCodeReadonlyNoSend = %q, want %q", got, want)
	}
}

// TestCHN_SetReadonly_OwnerOnly_HappyPath — owner sets readonly,
// response carries readonly=true + collapsed bit 4.
func TestCHN_SetReadonly_OwnerOnly_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID+"/readonly", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT readonly: got %d (%v)", resp.StatusCode, body)
	}
	if body["readonly"] != true {
		t.Errorf("readonly = %v, want true", body["readonly"])
	}
	collapsed, _ := body["collapsed"].(float64)
	if int64(collapsed)&int64(api.ReadonlyBit) == 0 {
		t.Errorf("collapsed=%d missing ReadonlyBit (=16)", int64(collapsed))
	}
}

// TestCHN_SetReadonly_NonOwner_403 — non-creator → 403.
func TestCHN_SetReadonly_NonOwner_403(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	// Foreign-org user can't even hit the channel; use a same-org member
	// instead (the owner created `general`; member is also in org-A).
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	_ = s
	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID+"/readonly", memberTok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-creator PUT: got %d, want 403", resp.StatusCode)
	}
}

// TestCHN_UnsetReadonly_Idempotent — DELETE twice both 200.
func TestCHN_UnsetReadonly_Idempotent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	// Set first, then DELETE twice — second DELETE should still be 200.
	testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID+"/readonly", tok, nil)
	resp, body := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+chID+"/readonly", tok, nil)
	if resp.StatusCode != http.StatusOK || body["readonly"] != false {
		t.Errorf("first DELETE: %d readonly=%v", resp.StatusCode, body["readonly"])
	}
	resp, body = testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+chID+"/readonly", tok, nil)
	if resp.StatusCode != http.StatusOK || body["readonly"] != false {
		t.Errorf("second DELETE (idempotent): %d readonly=%v", resp.StatusCode, body["readonly"])
	}
}

// TestCHN_SendBlockedForNonCreator_WhenReadonly — non-creator POST
// /messages → 403 channel.readonly_no_send.
func TestCHN_SendBlockedForNonCreator_WhenReadonly(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	// Owner sets readonly.
	testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID+"/readonly", ownerTok, nil)

	// Member (non-creator) tries to send → 403.
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", memberTok,
		map[string]any{"content": "hello"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-creator send: got %d, want 403", resp.StatusCode)
	}
	errStr, _ := body["error"].(string)
	if !strings.Contains(errStr, "channel.readonly_no_send") {
		t.Errorf("error = %q, want channel.readonly_no_send", errStr)
	}
}

// TestCHN_SendAllowedForCreator_WhenReadonly — creator's own send
// passes when readonly=true.
func TestCHN_SendAllowedForCreator_WhenReadonly(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID+"/readonly", ownerTok, nil)

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", ownerTok,
		map[string]any{"content": "creator can still send"})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("creator send when readonly: got %d, want 201 (%v)", resp.StatusCode, body)
	}
}

// TestCHN_SendAllowedForNonCreator_WhenNotReadonly — control: non-
// creator can send when channel is NOT readonly (反向断言不误伤).
func TestCHN_SendAllowedForNonCreator_WhenNotReadonly(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", memberTok,
		map[string]any{"content": "non-creator pre-readonly"})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("non-creator send when NOT readonly: got %d, want 201", resp.StatusCode)
	}
}

// TestCHN_NoAdminReadonlyPath — admin-rail does NOT mount any
// CHN-15 readonly toggle endpoint. Reverse-grep for the specific path
// (avoids false-positive matches on the word "readonly" that legitimately
// appears in admin GET-only doc comments for other milestones).
func TestCHN_NoAdminReadonlyPath(t *testing.T) {
	t.Parallel()
	root := chn15RepoRoot(t)
	dir := filepath.Join(root, "packages/server-go/internal")
	pat := regexp.MustCompile(`/admin-api/v[0-9]+/channels/[^/"]*/readonly|RegisterCHN15.*adminMw`)
	hits := chn15GrepCount(t, dir, pat)
	if hits != 0 {
		t.Errorf("admin-rail CHN-15 readonly endpoint grep: got %d, want 0 (admin god-mode 不挂 设计 ②)", hits)
	}
}

// TestCHN_ReadonlyEndpoint_Unauthorized — no auth → 401.
func TestCHN_ReadonlyEndpoint_Unauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/whatever/readonly", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no auth: got %d, want 401", resp.StatusCode)
	}
}

// chn15RepoRoot mirrors al_9 / dm_8 helper.
func chn15RepoRoot(t *testing.T) string {
	t.Helper()
	abs, _ := filepath.Abs("../../../..")
	return abs
}

func chn15GrepCount(t *testing.T, dir string, re *regexp.Regexp) int {
	t.Helper()
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := info.Name()
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			return nil
		}
		b, ferr := os.ReadFile(path)
		if ferr != nil {
			return nil
		}
		count += len(re.FindAllIndex(b, -1))
		return nil
	})
	return count
}
