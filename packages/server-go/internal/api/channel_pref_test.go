// Package api_test — chn_8_notif_pref_test.go: CHN-8 notification pref
// REST + 0 schema + bitmap 三向锁 + admin god-mode 不挂 + AST 锁链 #13
// + 不 drop messages.
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

// REG-CHN8-001 — 0 schema 改反向断言.
func TestChannelPref_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`(?i)chn_8_\d+|chn8_\d+_notif`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if pat.MatchString(filepath.Base(p)) {
			t.Errorf("CHN-8 设计 ① broken — new schema migration file %s", p)
		}
		return nil
	})
	pat2 := regexp.MustCompile(`(?i)ALTER TABLE user_channel_layout ADD COLUMN.*notif`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		if pat2.Find(body) != nil {
			t.Errorf("CHN-8 设计 ① broken — notif column ALTER in %s", p)
		}
		return nil
	})
}

func setPrefHelper(t *testing.T, baseURL, token, channelID, pref string) (int, map[string]any) {
	t.Helper()
	resp, body := testutil.JSON(t, http.MethodPut,
		baseURL+"/api/v1/channels/"+channelID+"/notification-pref", token,
		map[string]any{"pref": pref})
	return resp.StatusCode, body
}

// REG-CHN8-002a — set pref `all` (collapsed bits 2-3 == 0).
func TestCHN_SetPref_All(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "pref-all", "public")
	chID := ch["id"].(string)

	status, body := setPrefHelper(t, ts.URL, ownerToken, chID, "all")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", status, body)
	}
	if body["pref"] != "all" {
		t.Errorf("pref: got %v, want all", body["pref"])
	}
	pref, err := s.GetNotifPrefForUser(owner.ID, chID, int64(api.NotifPrefShift), int64(api.NotifPrefMask))
	if err != nil {
		t.Fatalf("GetNotifPrefForUser: %v", err)
	}
	if pref != int64(api.NotifPrefAll) {
		t.Errorf("pref store: got %d, want %d", pref, api.NotifPrefAll)
	}
}

// REG-CHN8-002b — set pref `mention`.
func TestCHN_SetPref_Mention(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "pref-mention", "public")
	chID := ch["id"].(string)

	status, _ := setPrefHelper(t, ts.URL, ownerToken, chID, "mention")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	pref, _ := s.GetNotifPrefForUser(owner.ID, chID, int64(api.NotifPrefShift), int64(api.NotifPrefMask))
	if pref != int64(api.NotifPrefMention) {
		t.Errorf("pref: got %d, want %d", pref, api.NotifPrefMention)
	}
}

// REG-CHN8-002c — set pref `none`.
func TestCHN_SetPref_None(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "pref-none", "public")
	chID := ch["id"].(string)

	status, _ := setPrefHelper(t, ts.URL, ownerToken, chID, "none")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	pref, _ := s.GetNotifPrefForUser(owner.ID, chID, int64(api.NotifPrefShift), int64(api.NotifPrefMask))
	if pref != int64(api.NotifPrefNone) {
		t.Errorf("pref: got %d, want %d", pref, api.NotifPrefNone)
	}
}

// REG-CHN8-002d — spec 外值 → 400 invalid_value.
func TestCHN_SetPref_RejectsInvalidValue(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "pref-bad", "public")
	chID := ch["id"].(string)

	for _, bad := range []string{"xxx", "ALL", "Mention", ""} {
		status, body := setPrefHelper(t, ts.URL, ownerToken, chID, bad)
		if status != http.StatusBadRequest {
			t.Errorf("pref=%q: got %d, want 400", bad, status)
		}
		if got, _ := body["code"].(string); got != "notification_pref.invalid_value" {
			t.Errorf("pref=%q code: got %v, want notification_pref.invalid_value", bad, body["code"])
		}
	}
}

// REG-CHN8-002e — non-member rejected 403.
func TestCHN_SetPref_NonMemberRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "private-pref", "private")
	chID := ch["id"].(string)
	status, _ := setPrefHelper(t, ts.URL, memberToken, chID, "mention")
	if status != http.StatusForbidden {
		t.Errorf("non-member pref: got %d, want 403", status)
	}
}

// REG-CHN8-003 — NotifPref consts byte-identical 三向锁 + GetNotifPref 谓词.
func TestCHN_NotifPrefConsts_ByteIdentical(t *testing.T) {
	t.Parallel()
	if api.NotifPrefShift != 2 {
		t.Errorf("NotifPrefShift drift: got %d, want 2", api.NotifPrefShift)
	}
	if api.NotifPrefMask != 3 {
		t.Errorf("NotifPrefMask drift: got %d, want 3", api.NotifPrefMask)
	}
	if api.NotifPrefAll != 0 || api.NotifPrefMention != 1 || api.NotifPrefNone != 2 {
		t.Errorf("NotifPref consts drift: %d/%d/%d, want 0/1/2",
			api.NotifPrefAll, api.NotifPrefMention, api.NotifPrefNone)
	}
	// Bitmap predicate: bits 2-3 isolation.
	if api.GetNotifPref(0) != int64(api.NotifPrefAll) {
		t.Error("GetNotifPref(0) != All")
	}
	if api.GetNotifPref(4) != int64(api.NotifPrefMention) {
		t.Error("GetNotifPref(4) != Mention (bits 2-3 = 01)")
	}
	if api.GetNotifPref(8) != int64(api.NotifPrefNone) {
		t.Error("GetNotifPref(8) != None (bits 2-3 = 10)")
	}
}

// REG-CHN8-004 — admin god-mode 不挂 反向断言.
func TestCHN_NoAdminNotifPrefPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*notification`)
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
				t.Errorf("CHN-8 设计 ② broken — admin notification path in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
	pat2 := regexp.MustCompile(`(?i)func.*[Aa]dmin\w*[Nn]otif[Pp]ref\b`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := filepath.Base(p)
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || !strings.HasPrefix(base, "admin") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat2.FindIndex(body); loc != nil {
				t.Errorf("CHN-8 设计 ② broken — admin notif handler in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN8-005 — bitmap isolation: 改 pref 不动 collapsed bit 0 (CHN-3
// 折叠) — round-trip 验证位互不干扰.
func TestCHN_BitmapIsolation_PreservesOtherBits(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "bit-iso", "public")
	chID := ch["id"].(string)

	// Step 1: set bit 0 via PUT /me/layout (CHN-3 collapsed = 1).
	resp, _ := testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/me/layout",
		ownerToken, map[string]any{
			"layout": []map[string]any{
				{"channel_id": chID, "collapsed": 1, "position": 0.0},
			},
		})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("layout PUT: %d", resp.StatusCode)
	}

	// Step 2: set notif pref = mention (bits 2-3 = 01).
	if status, _ := setPrefHelper(t, ts.URL, ownerToken, chID, "mention"); status != http.StatusOK {
		t.Fatalf("set pref: %d", status)
	}

	// collapsed should now be 1 | (1<<2) = 5 (bit 0 + bit 2 set).
	collapsed, err := s.GetCollapsedForUser(owner.ID, chID)
	if err != nil {
		t.Fatalf("GetCollapsedForUser: %v", err)
	}
	if collapsed&1 == 0 {
		t.Errorf("CHN-3 collapsed bit 0 lost: collapsed=%d", collapsed)
	}
	if api.GetNotifPref(collapsed) != int64(api.NotifPrefMention) {
		t.Errorf("notif pref drift: got %d, want %d", api.GetNotifPref(collapsed), api.NotifPrefMention)
	}

	// Step 3: change pref to none — bit 0 still preserved.
	setPrefHelper(t, ts.URL, ownerToken, chID, "none")
	collapsed, _ = s.GetCollapsedForUser(owner.ID, chID)
	if collapsed&1 == 0 {
		t.Errorf("CHN-3 collapsed bit 0 lost after second pref change: collapsed=%d", collapsed)
	}
	if api.GetNotifPref(collapsed) != int64(api.NotifPrefNone) {
		t.Errorf("notif pref: got %d, want None", api.GetNotifPref(collapsed))
	}
}

// REG-CHN8-006a — notif pref 不 drop messages 反向断言.
func TestCHN_NotifPrefDoesNotDropMessages(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "ws"), filepath.Join("..", "api")}
	pat := regexp.MustCompile(`(?i)notif_pref\s*[\.\s\w]*\b(skip|drop)\s*\b.*\b(broadcast|fanout|message|frame)`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat.FindIndex(body); loc != nil {
				t.Errorf("CHN-8 设计 ③ broken — notif pref drops messages in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN8-006b — AST 锁链延伸第 13 处.
func TestCHN_NoNotifPrefQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingNotifPref",
		"notifPrefQueue",
		"deadLetterNotifPref",
	}
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "push")}
	for _, dir := range dirs {
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
					t.Errorf("AST 锁链延伸第 13 处 broken — token %q in %s", tok, p)
				}
			}
			return nil
		})
	}
}

// REG-CHN8-cov — 401 unauthorized branch.
func TestCHN_SetPref_Unauthorized401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "auth-pref", "public")
	chID := ch["id"].(string)
	status, _ := setPrefHelper(t, ts.URL, "", chID, "all")
	if status != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", status)
	}
}

// REG-CHN8-cov — 404 channel not found.
func TestCHN_SetPref_NotFound404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	status, _ := setPrefHelper(t, ts.URL, ownerToken,
		"00000000-0000-0000-0000-000000000000", "all")
	if status != http.StatusNotFound {
		t.Errorf("not-found: got %d, want 404", status)
	}
}
