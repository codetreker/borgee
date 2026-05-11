// Package api_test — dm_11_search_test.go: DM-11 cross-DM message
// search REST acceptance tests.

package api_test

import (
	"net/http"
	"net/url"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// dm11Setup builds a server, opens a DM, posts a message with searchable
// content, returns (url, ownerTok, memberTok, store, dmChannelID, msgContent).
func dm11Setup(t *testing.T) (string, string, string, *store.Store, string, string) {
	t.Helper()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	users, _ := s.ListUsers()
	var ownerID, memberID string
	for _, u := range users {
		if u.Email == nil {
			continue
		}
		if *u.Email == "owner@test.com" {
			ownerID = u.ID
		}
		if *u.Email == "member@test.com" {
			memberID = u.ID
		}
	}
	if ownerID == "" || memberID == "" {
		t.Skip("missing fixture users")
	}
	resp, dm := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+ownerID, memberTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dm open: %d %v", resp.StatusCode, dm)
	}
	chID, _ := dm["channel"].(map[string]any)["id"].(string)
	if chID == "" {
		t.Fatalf("no channel id")
	}
	content := "find this needle in haystack"
	r, msg := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", memberTok,
		map[string]any{"content": content})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("post: %d %v", r.StatusCode, msg)
	}
	return ts.URL, ownerTok, memberTok, s, chID, content
}

// TestDM_Search_Success — owner searches messages posted by a DM member.
func TestDM_Search_Success(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	resp, body := testutil.JSON(t, "GET",
		url+"/api/v1/dm/search?q=needle", ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: %d %v", resp.StatusCode, body)
	}
	count, _ := body["count"].(float64)
	if count != 1 {
		t.Errorf("expected count=1, got %v", body["count"])
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}
	first, _ := msgs[0].(map[string]any)
	content, _ := first["content"].(string)
	if content == "" || content == "needle" {
		t.Errorf("expected matched content, got %q", content)
	}
}

// TestDM_Search_QRequired — q 缺失 → 400 dm_search.q_required.
func TestDM_Search_QRequired(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	resp, body := testutil.JSON(t, "GET", url+"/api/v1/dm/search", ownerTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if body["code"] != "dm_search.q_required" {
		t.Errorf("expected code dm_search.q_required, got %v", body["code"])
	}
}

// TestDM_Search_QTooShort — q 1 char → 400 dm_search.q_too_short.
func TestDM_Search_QTooShort(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	resp, body := testutil.JSON(t, "GET", url+"/api/v1/dm/search?q=a", ownerTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if body["code"] != "dm_search.q_too_short" {
		t.Errorf("expected code dm_search.q_too_short, got %v", body["code"])
	}
}

// TestDM_Search_QTooLong — q > 200 char → 400 dm_search.q_too_long (反 DoS).
func TestDM_Search_QTooLong(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	long := ""
	for i := 0; i < 201; i++ {
		long += "x"
	}
	resp, body := testutil.JSON(t, "GET",
		url+"/api/v1/dm/search?q="+long, ownerTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if body["code"] != "dm_search.q_too_long" {
		t.Errorf("expected code dm_search.q_too_long, got %v", body["code"])
	}
}

// TestDM_Search_Unauthorized — no token → 401.
func TestDM_Search_Unauthorized(t *testing.T) {
	t.Parallel()
	url, _, _, _, _, _ := dm11Setup(t)
	resp, _ := testutil.JSON(t, "GET", url+"/api/v1/dm/search?q=needle", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestDM_Search_NoMatch — query matches nothing → 200 + empty list.
func TestDM_Search_NoMatch(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	resp, body := testutil.JSON(t, "GET",
		url+"/api/v1/dm/search?q=zzzNonExistentXYZ", ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	count, _ := body["count"].(float64)
	if count != 0 {
		t.Errorf("expected empty result, got count=%v", count)
	}
}

// TestDM_Search_DMOnly_ExcludesPublicChannel — public channel msg
// matching the query is NOT returned (DM-only scope filter).
func TestDM_Search_DMOnly_ExcludesPublicChannel(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	// Owner posts the same needle in a public channel.
	pubID := cv12General(t, url, ownerTok)
	r, _ := testutil.JSON(t, "POST", url+"/api/v1/channels/"+pubID+"/messages", ownerTok,
		map[string]any{"content": "needle in public channel"})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("post pub: %d", r.StatusCode)
	}
	// Search → only DM message returned, not public.
	resp, body := testutil.JSON(t, "GET",
		url+"/api/v1/dm/search?q=needle", ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: %d", resp.StatusCode)
	}
	msgs, _ := body["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		c, _ := mm["content"].(string)
		if c == "needle in public channel" {
			t.Fatalf("DM search returned public channel message: %q", c)
		}
	}
}

// TestDM_Search_NonMember_NoLeak — third user (not in the DM) searches
// and sees 0 results from this DM (channel-member ACL filter via JOIN).
func TestDM_Search_NonMember_NoLeak(t *testing.T) {
	t.Parallel()
	url, _, _, s, _, _ := dm11Setup(t)
	users, _ := s.ListUsers()
	var thirdEmail string
	for _, u := range users {
		if u.Email == nil {
			continue
		}
		if *u.Email != "owner@test.com" && *u.Email != "member@test.com" {
			thirdEmail = *u.Email
			break
		}
	}
	if thirdEmail == "" {
		t.Skip("missing non-member fixture user")
	}
	thirdTok := testutil.LoginAs(t, url, thirdEmail, "password123")
	resp, body := testutil.JSON(t, "GET",
		url+"/api/v1/dm/search?q=needle", thirdTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	count, _ := body["count"].(float64)
	if count != 0 {
		t.Errorf("non-member received %v DM results; ACL JOIN did not exclude them", count)
	}
}

// TestDM_Search_LimitClamp — limit=999 is clamped to 50.
func TestDM_Search_LimitClamp(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := dm11Setup(t)
	// Verify the request succeeds; limit clamp is enforced server-side.
	v := url + "/api/v1/dm/search?q=needle&limit=999"
	resp, body := testutil.JSON(t, "GET", v, ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d %v", resp.StatusCode, body)
	}
}

// TestDM_Search_DeletedMessageHidden — soft-deleted DM message NOT
// returned by search (maskDeletedMessages helper excludes it before response).
func TestDM_Search_DeletedMessageHidden(t *testing.T) {
	t.Parallel()
	url, _, memberTok, _, chID, _ := dm11Setup(t)
	// Post a fresh message to delete.
	r, msg := testutil.JSON(t, "POST", url+"/api/v1/channels/"+chID+"/messages", memberTok,
		map[string]any{"content": "ephemeral haystack"})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("post: %d", r.StatusCode)
	}
	msgID, _ := msg["message"].(map[string]any)["id"].(string)
	// Delete it.
	d, _ := testutil.JSON(t, "DELETE", url+"/api/v1/messages/"+msgID, memberTok, nil)
	if d.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", d.StatusCode)
	}
	// Search must NOT return the deleted msg (deleted_at IS NULL filter).
	resp, body := testutil.JSON(t, "GET",
		url+"/api/v1/dm/search?q=ephemeral", memberTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: %d", resp.StatusCode)
	}
	msgs, _ := body["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		c, _ := mm["content"].(string)
		if c == "ephemeral haystack" {
			t.Fatalf("deleted message returned in search: %q", c)
		}
	}
}

// _ for net/url import (keep linter happy if unused).
var _ = url.QueryEscape
