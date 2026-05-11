// Package api_test — chn_9_visibility_test.go: CHN-9 channel privacy
// 三态 + 0 schema + 三向锁定 + admin 权限不挂 + creator_only leak 反断
// + AST 守护链延伸第 14 处.
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

// REG-CHN9-001 — 0 schema 改反向断言.
func TestChn9visibility_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	pat := regexp.MustCompile(`(?i)chn_9_\d+|chn9_\d+_visibility`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if pat.MatchString(filepath.Base(p)) {
			t.Errorf("CHN-9 设计第 1 条检查失败 — new schema migration file %s", p)
		}
		return nil
	})
	pat2 := regexp.MustCompile(`(?i)ALTER TABLE channels.*ADD COLUMN.*visibility|ALTER TABLE channels.*MODIFY.*visibility`)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		// Skip tests; we only care about production migration files.
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		if pat2.Find(body) != nil {
			t.Errorf("CHN-9 设计第 1 条检查失败 — visibility ALTER in %s", p)
		}
		return nil
	})
}

// REG-CHN9-002 — VisibilityConsts 字节级一致 三向锁定.
func TestCHN_VisibilityConsts_ByteIdentical(t *testing.T) {
	t.Parallel()
	if api.VisibilityCreatorOnly != "creator_only" {
		t.Errorf("VisibilityCreatorOnly 脱节: got %q", api.VisibilityCreatorOnly)
	}
	if api.VisibilityMembers != "private" {
		t.Errorf("VisibilityMembers 脱节: got %q", api.VisibilityMembers)
	}
	if api.VisibilityOrgPublic != "public" {
		t.Errorf("VisibilityOrgPublic 脱节: got %q", api.VisibilityOrgPublic)
	}
	if !api.IsValidVisibility("creator_only") {
		t.Error("IsValidVisibility(creator_only): got false")
	}
	if !api.IsValidVisibility("private") {
		t.Error("IsValidVisibility(private): got false")
	}
	if !api.IsValidVisibility("public") {
		t.Error("IsValidVisibility(public): got false")
	}
	for _, bad := range []string{"secret", "team", "Public", "", "Private"} {
		if api.IsValidVisibility(bad) {
			t.Errorf("IsValidVisibility(%q): got true, want false", bad)
		}
	}
	// VisibilityRejectMessage 单一来源 字节级一致.
	if api.VisibilityRejectMessage != "Visibility must be 'creator_only', 'private', or 'public'" {
		t.Errorf("VisibilityRejectMessage 脱节: got %q", api.VisibilityRejectMessage)
	}
}

// REG-CHN9-003a — PATCH visibility=creator_only happy path (owner).
func TestCHN_PatchVisibility_CreatorOnly_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "co-channel", "public")
	chID := ch["id"].(string)

	resp, body := testutil.JSON(t, http.MethodPut,
		ts.URL+"/api/v1/channels/"+chID, ownerToken,
		map[string]any{"visibility": "creator_only"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	updated, _ := body["channel"].(map[string]any)
	if updated["visibility"] != "creator_only" {
		t.Errorf("visibility: got %v, want creator_only", updated["visibility"])
	}
}

// REG-CHN9-003b — backcompat: existing public/private PATCH 仍 OK 字节级一致.
func TestCHN_PatchVisibility_BackcompatPublicPrivate(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "back-compat", "public")
	chID := ch["id"].(string)

	for _, vis := range []string{"private", "public"} {
		resp, body := testutil.JSON(t, http.MethodPut,
			ts.URL+"/api/v1/channels/"+chID, ownerToken,
			map[string]any{"visibility": vis})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("PATCH visibility=%s: got %d", vis, resp.StatusCode)
		}
		updated, _ := body["channel"].(map[string]any)
		if updated["visibility"] != vis {
			t.Errorf("visibility: got %v, want %s", updated["visibility"], vis)
		}
	}
}

// REG-CHN9-004 — PATCH spec 外值 → 400 字节级一致 reject message.
func TestCHN_PatchVisibility_RejectsInvalidValue(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "rej-vis", "public")
	chID := ch["id"].(string)

	for _, bad := range []string{"secret", "team", "Public", "Private"} {
		resp, body := testutil.JSON(t, http.MethodPut,
			ts.URL+"/api/v1/channels/"+chID, ownerToken,
			map[string]any{"visibility": bad})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("visibility=%q: got %d, want 400", bad, resp.StatusCode)
		}
		if got, _ := body["error"].(string); got != api.VisibilityRejectMessage {
			t.Errorf("visibility=%q msg: got %q, want %q", bad, got, api.VisibilityRejectMessage)
		}
	}
}

// REG-CHN9-005a — creator_only channel 不 leak 给 org peers.
func TestCHN_CreatorOnlyChannel_NotLeakedToOrgPeers(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Owner creates a creator_only channel.
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "creator-only-test", "public")
	chID := ch["id"].(string)
	testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+chID, ownerToken,
		map[string]any{"visibility": "creator_only"})

	// Other user (same org peer) should NOT see the channel via list.
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels", memberToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list channels: %d", resp.StatusCode)
	}
	channels, _ := body["channels"].([]any)
	for _, raw := range channels {
		c, _ := raw.(map[string]any)
		if c["id"] == chID {
			t.Errorf("CHN-9 设计第 3 条检查失败 — creator_only channel leaked to non-creator: %v", c)
		}
	}
}

// REG-CHN9-005b — ListChannelsWithUnread filter 字节级一致 不动.
//
// 反向断言 SQL `visibility = 'public'` 字面跟 CHN-1.2 既有同源 (creator_only
// 不入 org-public preview filter).
func TestCHN_ListChannelsFilter_ByteIdentical(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "store", "queries.go"))
	if err != nil {
		t.Fatalf("read queries.go: %v", err)
	}
	// Existing CHN-1.2 filter 字面 字节级一致 锁定 — grep 检查
	// `visibility = 'public'` 必须 ≥1 hit.
	pat := regexp.MustCompile(`visibility\s*=\s*'public'`)
	if pat.Find(body) == nil {
		t.Error("CHN-9 设计第 3 条检查失败 — ListChannelsWithUnread `visibility = 'public'` filter 字面消失")
	}
	// 反向断言: 不出现 `visibility = 'creator_only'` 在 SQL 显式 filter
	// (creator_only 走 IsChannelMember + creator-only ACL, 不走 SQL filter).
	pat2 := regexp.MustCompile(`visibility\s*=\s*'creator_only'`)
	if pat2.Find(body) != nil {
		t.Error("CHN-9 设计第 3 条检查失败 — creator_only 不应入 SQL filter (走 IsChannelMember ACL)")
	}
}

// REG-CHN9-006 — admin 权限不挂 visibility PATCH 反向断言.
func TestCHN_NoAdminVisibilityPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*visibility`)
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
				t.Errorf("CHN-9 设计第 3 条检查失败 — admin visibility path in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
	// admin*.go 不含 admin-visibility handler symbol.
	pat2 := regexp.MustCompile(`(?i)func.*[Aa]dmin\w*[Vv]isibility\b`)
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
				t.Errorf("CHN-9 设计第 3 条检查失败 — admin visibility handler in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN9-007 — AST 守护链延伸第 14 处.
func TestCHN_NoVisibilityQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingVisibility",
		"visibilityChangeQueue",
		"deadLetterVisibility",
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
				t.Errorf("AST 守护链延伸第 14 处检查失败 — token %q in %s", tok, p)
			}
		}
		return nil
	})
}
