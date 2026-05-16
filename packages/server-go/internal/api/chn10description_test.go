// Package api_test — chn_10_description_test.go: CHN-10 owner-only PUT
// /api/v1/channels/{channelId}/description endpoint + 约束守门.
//
// Pins:
//
//	REG-CHN10-001 TestChn10description_NoSchemaChange (filepath.Walk migrations/)
//	REG-CHN10-002 TestCHN_PutDescription_OwnerHappyPath
//	              + _NonOwnerRejected + _Unauthorized401
//	REG-CHN10-003 TestCHN_PutDescription_LengthCap500
//	REG-CHN10-004 TestCHN_TopicPathByteIdentical (grep 检查 dm_10/chn_10
//	              字面 在 channels.go::handleSetTopic block 0 hit)
//	REG-CHN10-005 TestCHN_NoAdminDescriptionPath
//	REG-CHN10-006 TestCHN_NoDescriptionQueue
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// chnHelper — minimal channel + owner setup. Returns ownerToken, owner,
// non-owner-member, and a channel they both belong to.
type chn10Setup struct {
	ts          *httptestServerSurrogate // type alias-like
	store       *store.Store
	ownerToken  string
	memberToken string
	owner       *store.User
	member      *store.User
	channelID   string
}

// httptestServerSurrogate aliases httptest.Server fields used here. We
// don't import httptest directly because testutil.NewTestServer returns
// *httptest.Server typed.
type httptestServerSurrogate = struct {
	URL string
}

func setupCHN10(t *testing.T) (string, string, string, string, *store.Store) {
	t.Helper()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	// Create channel via POST /api/v1/channels (owner becomes creator).
	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels", ownerToken,
		map[string]any{"name": "chn10-test", "visibility": "public"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel: %d %v", resp.StatusCode, body)
	}
	ch, _ := body["channel"].(map[string]any)
	channelID, _ := ch["id"].(string)
	if channelID == "" {
		t.Fatalf("channel.id missing in response: %v", body)
	}
	// Add member.
	if err := s.AddChannelMember(&store.ChannelMember{
		ChannelID: channelID, UserID: member.ID,
	}); err != nil {
		t.Fatalf("AddChannelMember: %v", err)
	}
	_ = owner
	return ts.URL, ownerToken, memberToken, channelID, s
}

// REG-CHN10-002a — owner HappyPath PUT /description → 200.
func TestCHN_PutDescription_OwnerHappyPath(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, channelID, s := setupCHN10(t)

	resp, body := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", ownerToken,
		map[string]any{"description": "首页频道说明文本"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	// Verify topic column written.
	ch, err := s.GetChannelByID(channelID)
	if err != nil || ch == nil {
		t.Fatalf("reload channel: %v", err)
	}
	if ch.Topic != "首页频道说明文本" {
		t.Errorf("topic: got %q, want %q", ch.Topic, "首页频道说明文本")
	}
}

// REG-CHN10-002b — non-owner member 403 (设计第 2 条 owner-only).
func TestCHN_PutDescription_NonOwnerRejected(t *testing.T) {
	t.Parallel()
	url, _, memberToken, channelID, _ := setupCHN10(t)

	resp, _ := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", memberToken,
		map[string]any{"description": "非 owner 不能改"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner: got %d, want 403 (owner-only ACL 对齐链第 20 处)", resp.StatusCode)
	}
}

// REG-CHN10-002c — 401 unauthorized (空 token).
func TestCHN_PutDescription_Unauthorized401(t *testing.T) {
	t.Parallel()
	url, _, _, channelID, _ := setupCHN10(t)

	resp, _ := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", "",
		map[string]any{"description": "x"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}
}

// REG-CHN10-003 — length cap 500 (501 reject 400).
func TestCHN_PutDescription_LengthCap500(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, channelID, _ := setupCHN10(t)

	// 500 ASCII chars → OK.
	exact500 := strings.Repeat("a", 500)
	resp, body := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", ownerToken,
		map[string]any{"description": exact500})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("500 chars: got %d, want 200: %v", resp.StatusCode, body)
	}

	// 501 chars → 400.
	over501 := strings.Repeat("b", 501)
	resp, _ = testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", ownerToken,
		map[string]any{"description": over501})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("501 chars: got %d, want 400 (length cap %d)", resp.StatusCode, api.DescriptionMaxLength)
	}
}

// REG-CHN10-002d — channel not found → 404.
func TestCHN_PutDescription_ChannelNotFound(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, _, _ := setupCHN10(t)
	resp, _ := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/00000000-0000-0000-0000-000000000000/description",
		ownerToken,
		map[string]any{"description": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not-found: got %d, want 404", resp.StatusCode)
	}
}

// REG-CHN10-002e — invalid JSON body → 400 (bumps handler coverage).
func TestCHN_PutDescription_InvalidJSONBody(t *testing.T) {
	t.Parallel()
	urlBase, ownerToken, _, channelID, _ := setupCHN10(t)
	// raw HTTP request with non-JSON body.
	req, _ := http.NewRequest(http.MethodPut,
		urlBase+"/api/v1/channels/"+channelID+"/description",
		strings.NewReader("not json {{{"))
	req.AddCookie(&http.Cookie{Name: "borgee_token", Value: ownerToken})
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid-json: got %d, want 400", resp.StatusCode)
	}
}
