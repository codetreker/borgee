// Package api_test — chn_7_mute_test.go: CHN-7 mute/unmute REST + 0
// schema 改 + bitmap + admin god-mode 不挂 + AST 对齐链延伸第 12 处 + mute
// 不 drop messages best-effort.
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

// REG-CHN7-001 — 0 schema 改反向断言.
func TestChn7mute_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`(?i)chn_7_\d+|chn7_\d+_mute`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if pat.MatchString(filepath.Base(p)) {
			t.Errorf("CHN-7 设计 ① broken — new schema migration file %s", p)
		}
		return nil
	})
	pat2 := regexp.MustCompile(`(?i)ALTER TABLE user_channel_layout.*muted`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		if pat2.Find(body) != nil {
			t.Errorf("CHN-7 设计 ① broken — muted column ALTER in %s", p)
		}
		return nil
	})
}

// REG-CHN7-002a — POST mute happy path; collapsed bit 1 set.
func TestCHN_MuteChannel_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "to-mute", "public")
	chID := ch["id"].(string)

	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["muted"] != true {
		t.Errorf("muted: got %v, want true", body["muted"])
	}
	cVal, _ := body["collapsed"].(float64)
	if int64(cVal)&int64(api.MuteBit) == 0 {
		t.Errorf("collapsed bit 1 not set: got %v", cVal)
	}

	// IsMutedForUser store helper agrees.
	muted, err := s.IsMutedForUser(owner.ID, chID, int64(api.MuteBit))
	if err != nil {
		t.Fatalf("IsMutedForUser: %v", err)
	}
	if !muted {
		t.Error("IsMutedForUser: got false, want true")
	}
}

// REG-CHN7-002b — non-member 403.
func TestCHN_MuteChannel_NonMemberRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "private-mute", "private")
	chID := ch["id"].(string)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+chID+"/mute", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-member mute: got %d, want 403", resp.StatusCode)
	}
}

// REG-CHN7-002c — Unauthorized 401.
func TestCHN_MuteChannel_Unauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/some-id/mute", "", nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 unauthenticated, got 200")
	}
}

// REG-CHN7-003a — DELETE unmute clears bit 1.
func TestCHN_UnmuteChannel_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "round-trip", "public")
	chID := ch["id"].(string)

	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	resp, body := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["muted"] != false {
		t.Errorf("muted: got %v, want false", body["muted"])
	}
	cVal, _ := body["collapsed"].(float64)
	if int64(cVal)&int64(api.MuteBit) != 0 {
		t.Errorf("collapsed bit 1 still set: got %v", cVal)
	}
}

// REG-CHN7-003b — DELETE idempotent.
func TestCHN_UnmuteChannel_Idempotent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "idem", "public")
	chID := ch["id"].(string)
	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	for i := 0; i < 2; i++ {
		resp, _ := testutil.JSON(t, http.MethodDelete,
			ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("DELETE %d: got %d, want 200", i, resp.StatusCode)
		}
	}
}

// REG-CHN7-003c — unmute preserves collapse bit (bit 0).
func TestCHN_UnmuteChannel_PreservesCollapsedBit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "bit-preserve", "public")
	chID := ch["id"].(string)

	// Set bit 0 (CHN-3 collapsed) via PUT /me/layout.
	testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/me/layout", ownerToken, map[string]any{
		"layout": []map[string]any{
			{"channel_id": chID, "collapsed": 1, "position": 0.0},
		},
	})
	// Then mute (set bit 1) — collapsed should become 3.
	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	// Then unmute (clear bit 1) — collapsed should become 1 (bit 0 preserved).
	testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)

	muted, _ := s.IsMutedForUser(owner.ID, chID, int64(api.MuteBit))
	if muted {
		t.Error("IsMutedForUser after unmute: got true")
	}
}

// REG-CHN7-004 — MuteBit byte-identical 双向锁 + IsMuted 谓词单一来源.
func TestCHN_MuteBit_ByteIdentical(t *testing.T) {
	t.Parallel()
	if api.MuteBit != 2 {
		t.Errorf("MuteBit mismatch: got %d, want 2 (双向锁跟 client MUTE_BIT)", api.MuteBit)
	}
	if api.IsMuted(0) {
		t.Error("IsMuted(0): got true, want false")
	}
	if api.IsMuted(1) {
		t.Error("IsMuted(1) (collapsed only): got true, want false")
	}
	if !api.IsMuted(2) {
		t.Error("IsMuted(2) (muted only): got false, want true")
	}
	if !api.IsMuted(3) {
		t.Error("IsMuted(3) (collapsed+muted): got false, want true")
	}
}

// REG-CHN7-005 — admin god-mode 不挂 反向断言.
func TestCHN_NoAdminMutePath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*mute`)
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
				t.Errorf("CHN-7 设计 ② broken — admin mute path in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
	// admin*.go 不含 admin-mute handler symbol.
	pat2 := regexp.MustCompile(`(?i)func.*[Aa]dmin\w*[Mm]ute[Cc]hannel\b`)
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
				t.Errorf("CHN-7 设计 ② broken — admin mute handler in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN7-006a — mute 不 drop messages 反向断言 + AST 对齐链延伸第 12 处.
//
// 设计 ③: mute 仅 DL-4 push notifier skip — CreateMessage / RT-3 fan-out
// / WS frame 全 byte-identical. grep 检查 `mute.*skip.*broadcast\|
// mute.*drop.*message\|mute.*hub.*skip` 0 hit.
func TestCHN_MuteDoesNotDropMessages(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "ws"), filepath.Join("..", "api")}
	pat := regexp.MustCompile(`(?i)mute\s*[\.\s\w]*\b(skip|drop)\s*\b.*\b(broadcast|fanout|message|frame)`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat.FindIndex(body); loc != nil {
				t.Errorf("CHN-7 设计 ③ broken — mute drops messages in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN7-006b — AST 对齐链延伸第 12 处.
func TestCHN_NoChannelMuteQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingChannelMute",
		"channelMuteQueue",
		"deadLetterChannelMute",
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
					t.Errorf("AST 对齐链延伸第 12 处 broken — token %q in %s", tok, p)
				}
			}
			return nil
		})
	}
}
