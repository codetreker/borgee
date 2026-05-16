// Package api_test — chn_13_search_test.go: CHN-13 server search filter
// + grep checks (CHN-13 only adds the server LIKE filter and client SPA; no schema change).
//
// Covered checks:
//
//	REG-CHN13-001 TestChn13search_NoSchemaChange (filepath.Walk migrations/)
//	REG-CHN13-002 TestCHN_ListChannelsWithQuery (q="" matches existing response
//	               + q="match" substring filter)
//	REG-CHN13-003 TestCHN_QueryCaseInsensitive + QuerySubstringMatch
//	REG-CHN13-004 TestCHN_NoSearchQueue (AST alignment check)
//	REG-CHN13-005 TestCHN_NoAdminSearchPath (admin API must not mount search)
package api_test

import (
	"net/http"
	"net/url"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-CHN13-002 — GET /api/v1/channels?q= happy path + empty q matches existing response.
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

	// Empty q — full list, matching the existing path response.
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

// REG-CHN13-003 — q LIKE COLLATE NOCASE is case-insensitive + substring match.
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

// REG-CHN13-003b — substring match in the middle of the channel name.
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
