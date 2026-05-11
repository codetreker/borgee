// Package api_test — chn_6_pin_test.go: CHN-6 channel pin/unpin REST
// endpoints + 0 schema 改 + owner-only ACL + admin god-mode 不挂 + AST
// 对齐链延伸第 11 处.
//
// Pins:
//   REG-CHN6-001 TestChn6pin_NoSchemaChange — migrations/ 0 新文件
//   REG-CHN6-002 TestCHN61_PinChannel_* — POST /pin owner-only
//   REG-CHN6-003 TestCHN61_UnpinChannel_* — DELETE /pin idempotent
//   REG-CHN6-004 TestCHN_PinThreshold_ByteIdentical — 双向锁
//   REG-CHN6-005 TestCHN_NoAdminPinPath — admin 不挂
//   REG-CHN6-006 TestCHN_NoChannelPinQueue — AST 对齐链
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

// REG-CHN6-001 — 0 schema 改反向断言: migrations/ 0 新 chn_6_* file.
func TestChn6pin_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`(?i)chn_6_\d+|chn6_\d+_pin`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if pat.MatchString(filepath.Base(p)) {
			t.Errorf("CHN-6 设计 ① broken — new schema migration file %s", p)
		}
		return nil
	})
	// Also reject ALTER TABLE user_channel_layout ADD COLUMN pinned* in
	// any production migration file.
	pat2 := regexp.MustCompile(`(?i)ALTER TABLE user_channel_layout ADD COLUMN.*pinned`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		if pat2.Find(body) != nil {
			t.Errorf("CHN-6 设计 ① broken — pinned column ALTER in %s", p)
		}
		return nil
	})
}

// REG-CHN6-002a — POST /pin happy path; position < 0 (pinned 字面约定).
func TestCHN_PinChannel_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "to-pin", "public")
	chID := ch["id"].(string)

	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+chID+"/pin", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["pinned"] != true {
		t.Errorf("pinned: got %v, want true", body["pinned"])
	}
	pos, _ := body["position"].(float64)
	if pos >= 0 {
		t.Errorf("position: got %v, want < 0 (pinned 字面约定)", pos)
	}
}

// REG-CHN6-002b — non-member rejected 403.
func TestCHN_PinChannel_NonMemberRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	// owner creates a private channel that member is NOT in.
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "private-x", "private")
	chID := ch["id"].(string)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+chID+"/pin", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-member pin: got %d, want 403", resp.StatusCode)
	}
}

// REG-CHN6-002c — Unauthorized 401.
func TestCHN_PinChannel_Unauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/some-id/pin", "", nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 unauthenticated, got 200")
	}
}

// REG-CHN6-003a — DELETE /pin happy path; position > 0 (unpinned).
func TestCHN_UnpinChannel_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "round-trip", "public")
	chID := ch["id"].(string)

	// pin then unpin.
	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/pin", ownerToken, nil)
	resp, body := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/"+chID+"/pin", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["pinned"] != false {
		t.Errorf("pinned: got %v, want false", body["pinned"])
	}
	pos, _ := body["position"].(float64)
	if pos <= 0 {
		t.Errorf("position: got %v, want > 0 (unpinned)", pos)
	}
}

// REG-CHN6-003b — DELETE idempotent (二次 DELETE 200).
func TestCHN_UnpinChannel_Idempotent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "idem", "public")
	chID := ch["id"].(string)
	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/pin", ownerToken, nil)
	for i := 0; i < 2; i++ {
		resp, _ := testutil.JSON(t, http.MethodDelete,
			ts.URL+"/api/v1/channels/"+chID+"/pin", ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("DELETE %d: got %d, want 200", i, resp.StatusCode)
		}
	}
}

// REG-CHN6-004 — PinThreshold byte-identical 双向锁 + IsPinned 谓词单一来源.
func TestCHN_PinThreshold_ByteIdentical(t *testing.T) {
	t.Parallel()
	if api.PinThreshold != 0.0 {
		t.Errorf("PinThreshold mismatch: got %v, want 0.0 (双向锁跟 client POSITION_PIN_THRESHOLD)", api.PinThreshold)
	}
	if !api.IsPinned(-1.0) {
		t.Error("IsPinned(-1.0): got false, want true")
	}
	if api.IsPinned(0.0) {
		t.Error("IsPinned(0.0): got true, want false")
	}
	if api.IsPinned(1.0) {
		t.Error("IsPinned(1.0): got true, want false")
	}
}

// REG-CHN6-005 — admin god-mode 不挂 反向断言.
//
// 双 pattern: (1) admin handler 不挂 pin path; (2) 任何 admin*.go 不
// 含 pin_channel/pin\b symbol.
func TestCHN_NoAdminPinPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*pin`)
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
				t.Errorf("CHN-6 设计 ② broken — admin pin path in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
	// admin*.go must not contain admin-pin handler symbol.
	pat2 := regexp.MustCompile(`(?i)func.*[Aa]dmin\w*[Pp]in[Cc]hannel\b|admin\.\w*[Pp]in[Cc]hannel`)
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
				t.Errorf("CHN-6 设计 ② broken — admin pin handler in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN6-006 — AST 对齐链延伸第 11 处.
func TestCHN_NoChannelPinQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingChannelPin",
		"channelPinQueue",
		"deadLetterChannelPin",
	}
	dir := filepath.Join("..", "api")
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
				t.Errorf("AST 对齐链延伸第 11 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN6-cov — pin 404 channel not found + DM reject + unpin 401/404/non-member.
func TestCHN_PinChannel_NotFound404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/00000000-0000-0000-0000-000000000000/pin",
		ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("pin 404: got %d, want 404", resp.StatusCode)
	}
}

func TestCHN_PinChannel_DMRejected(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+dm.ID+"/pin", ownerToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("DM pin: got %d, want 400", resp.StatusCode)
	}
	if got, _ := body["code"].(string); got != "layout.dm_not_grouped" {
		t.Errorf("DM pin code: got %v, want layout.dm_not_grouped", body["code"])
	}
}

func TestCHN_UnpinChannel_NotFound404(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/00000000-0000-0000-0000-000000000000/pin",
		ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unpin 404: got %d, want 404", resp.StatusCode)
	}
}

func TestCHN_UnpinChannel_Unauthorized401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/some-id/pin", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unpin unauth: got %d, want 401", resp.StatusCode)
	}
}

func TestCHN_UnpinChannel_DMRejected(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")
	dm, _ := s.CreateDmChannel(owner.ID, member.ID)
	resp, _ := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/"+dm.ID+"/pin", ownerToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("DM unpin: got %d, want 400", resp.StatusCode)
	}
}

func TestCHN_UnpinChannel_NonMemberRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "unpin-priv", "private")
	chID := ch["id"].(string)
	resp, _ := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/"+chID+"/pin", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-member unpin: got %d, want 403", resp.StatusCode)
	}
}
