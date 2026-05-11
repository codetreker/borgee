// Package api_test — chn_13_search_test.go: CHN-13 server search filter
// + grep 检查 (CHN-13 仅 server LIKE filter + client SPA; 0 schema 改).
//
// 覆盖的检查点:
//   REG-CHN13-001 TestChn13search_NoSchemaChange (filepath.Walk migrations/)
//   REG-CHN13-002 TestCHN_ListChannelsWithQuery (q="" 与既有响应一致
//                  + q="match" 子串过滤)
//   REG-CHN13-003 TestCHN_QueryCaseInsensitive + QuerySubstringMatch
//   REG-CHN13-004 TestCHN_NoSearchQueue (AST 对齐检查第 21 处)
//   REG-CHN13-005 TestCHN_NoAdminSearchPath (admin 权限不挂)
package api_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-CHN13-001 — 0 schema 改 (grep 检查 migrations/chn_13_*).
func TestChn13search_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chn_13_") {
			t.Errorf("CHN-13 设计 ①检查失败 — found schema migration %q (must be 0 schema, 复用 channels 既有表)", e.Name())
		}
	}
}

// REG-CHN13-002 — GET /api/v1/channels?q= happy + 空 q 与既有响应一致.
func TestCHN_ListChannelsWithQuery(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")

	// Seed 3 channels: alpha / beta / gamma.
	for _, name := range []string{"alpha-search", "beta-search", "gamma-search"} {
		ch := &store.Channel{
			Name: name, Type: "channel", Visibility: "public",
			CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
			OrgID: owner.OrgID,
		}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
			t.Fatalf("add member %s: %v", name, err)
		}
	}

	// Empty q — full list (跟既有 path 响应一致).
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("empty q: got %d", resp.StatusCode)
	}
	all, _ := body["channels"].([]any)
	if len(all) < 3 {
		t.Errorf("empty q expected ≥3 channels, got %d", len(all))
	}

	// q=alpha — only alpha-search.
	resp, body = testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels?q=alpha", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("q=alpha: got %d", resp.StatusCode)
	}
	chs, _ := body["channels"].([]any)
	if len(chs) != 1 {
		t.Errorf("q=alpha expected 1 channel, got %d", len(chs))
	}
	if len(chs) > 0 {
		c, _ := chs[0].(map[string]any)
		if name, _ := c["name"].(string); name != "alpha-search" {
			t.Errorf("q=alpha got name=%q", name)
		}
	}
}

// REG-CHN13-003 — q LIKE COLLATE NOCASE 大小写不敏感 + 子串.
func TestCHN_QueryCaseInsensitive(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")

	ch := &store.Channel{
		Name: "MixedCase-Test", Type: "channel", Visibility: "public",
		CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
		OrgID: owner.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Lower-case query should match upper-case channel name.
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels?q="+url.QueryEscape("mixedcase"), ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("case-insensitive: got %d", resp.StatusCode)
	}
	chs, _ := body["channels"].([]any)
	if len(chs) < 1 {
		t.Errorf("case-insensitive expected ≥1 match, got %d", len(chs))
	}
}

// REG-CHN13-003b — 子串匹配 (中间字符).
func TestCHN_QuerySubstringMatch(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")

	ch := &store.Channel{
		Name: "abc-middle-xyz", Type: "channel", Visibility: "public",
		CreatedBy: owner.ID, Position: store.GenerateInitialRank(),
		OrgID: owner.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/channels?q=middle", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("substring: got %d", resp.StatusCode)
	}
	chs, _ := body["channels"].([]any)
	if len(chs) < 1 {
		t.Errorf("substring expected ≥1 match, got %d", len(chs))
	}
}

// REG-CHN13-004 — AST 对齐检查第 21 处 forbidden 3 token.
func TestCHN_NoSearchQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingSearch",
		"searchQueue",
		"deadLetterSearch",
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
				t.Errorf("AST 对齐检查第 21 处失败 — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-CHN13-005 — admin 权限不挂 search ?q= path (ADM-0 §1.3 限制;
// search 是 user-rail filter, admin /admin-api/v1/channels 既有列表已含
// 全 org, 不需要另外 search).
func TestCHN_NoAdminSearchPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("[^"]*admin-api/v[0-9]+/[^"]*\?q=`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			fb, _ := os.ReadFile(p)
			if loc := pat.FindIndex(fb); loc != nil {
				t.Errorf("CHN-13 admin 权限检查失败 — admin-rail search path in %s: %q",
					p, fb[loc[0]:loc[1]])
			}
			return nil
		})
	}
}
