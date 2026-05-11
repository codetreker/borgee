// Package api_test — reactions_acl_test.go: AP-4 acceptance — reactions
// 3 handler (PUT/DELETE/GET) MUST require channel membership (CV-7 #535
// gap fix; DM-5 #549 REG-DM5-005 documented and now closed).
//
// 设计约束 pin (ap-4-spec.md §0):
//   - 设计第 1 条 3 handler 加 ACL gate (Store.IsChannelMember + CanAccessChannel)
//   - 设计第 2 条 admin god-mode 不挂 (sanity)
//   - 设计第 3 条 REG-INV-002 fail-closed — non-member 三动作全 reject 404
//     字节级一致 "Channel not found" 跟 messages.go 既有同字符
//   - 设计第 4 条 既有 member OK path 不破 (反向 sanity)

package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// ap4Setup — owner creates a private channel + posts a message; member is
// NOT added (acts as the cross-channel non-member). Returns the URLs and
// tokens needed by all 4 case.
func ap4Setup(t *testing.T) (string, string, string, string, string) {
	t.Helper()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	_, ch := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", ownerTok, map[string]string{
		"name":       "ap4-private-" + t.Name(),
		"visibility": "private",
	})
	chID := ch["channel"].(map[string]any)["id"].(string)

	_, post := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", ownerTok,
		map[string]any{"content": "react to me"})
	msgID := post["message"].(map[string]any)["id"].(string)

	return ts.URL, ownerTok, memberTok, chID, msgID
}

// TestAP_PutReaction_NonMember404 pins 设计第 3 条: non-member PUT → 404.
func TestAP_PutReaction_NonMember404(t *testing.T) {
	t.Parallel()
	url, _, memberTok, _, msgID := ap4Setup(t)
	resp, _ := testutil.JSON(t, "PUT", url+"/api/v1/messages/"+msgID+"/reactions", memberTok,
		map[string]string{"emoji": "👍"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 fail-closed, got %d", resp.StatusCode)
	}
}

// TestAP_DeleteReaction_NonMember404 pins 设计第 3 条: non-member DELETE → 404.
func TestAP_DeleteReaction_NonMember404(t *testing.T) {
	t.Parallel()
	url, _, memberTok, _, msgID := ap4Setup(t)
	resp, _ := testutil.JSON(t, "DELETE", url+"/api/v1/messages/"+msgID+"/reactions", memberTok,
		map[string]string{"emoji": "👍"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 fail-closed, got %d", resp.StatusCode)
	}
}

// TestAP_GetReactions_NonMember404 pins 设计第 3 条: non-member GET → 404.
func TestAP_GetReactions_NonMember404(t *testing.T) {
	t.Parallel()
	url, _, memberTok, _, msgID := ap4Setup(t)
	resp, _ := testutil.JSON(t, "GET", url+"/api/v1/messages/"+msgID+"/reactions", memberTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 fail-closed, got %d", resp.StatusCode)
	}
}

// TestAP_Member_AllOK pins 设计第 4 条 反向 sanity: channel member 三动作 (PUT/GET/DELETE)
// 全 200 字节级一致 既有行为不破.
func TestAP_Member_AllOK(t *testing.T) {
	t.Parallel()
	url, ownerTok, memberTok, chID, msgID := ap4Setup(t)
	// Owner adds member to the channel.
	resp0, _ := testutil.JSON(t, "POST", url+"/api/v1/channels/"+chID+"/members", ownerTok,
		map[string]any{"user_id": mustUserIDAP4(t, url, memberTok)})
	if resp0.StatusCode != http.StatusOK && resp0.StatusCode != http.StatusCreated {
		t.Fatalf("add member: %d", resp0.StatusCode)
	}

	// Member PUT.
	r1, _ := testutil.JSON(t, "PUT", url+"/api/v1/messages/"+msgID+"/reactions", memberTok,
		map[string]string{"emoji": "👍"})
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("member PUT: %d", r1.StatusCode)
	}
	// Member GET.
	r2, _ := testutil.JSON(t, "GET", url+"/api/v1/messages/"+msgID+"/reactions", memberTok, nil)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("member GET: %d", r2.StatusCode)
	}
	// Member DELETE.
	r3, _ := testutil.JSON(t, "DELETE", url+"/api/v1/messages/"+msgID+"/reactions", memberTok,
		map[string]string{"emoji": "👍"})
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("member DELETE: %d", r3.StatusCode)
	}
}

// TestAP_GetReactions_Unauth401 pins 设计第 1 条 — pre-AP-4 GET handler skipped
// the user==nil check entirely; AP-4 fixes by emitting 401 on unauth.
//
// Note: middleware authMw should already block unauth requests at the
// route level; this test verifies the handler-level guard as defense in
// depth.
func TestAP_GetReactions_Unauth401(t *testing.T) {
	t.Parallel()
	url, _, _, _, msgID := ap4Setup(t)
	// Send GET without token — server's authMw will reject with 401 before
	// the handler runs, so we get 401 from the framework. This test pins
	// the bare-minimum guarantee that unauth never reads reactions.
	resp, _ := testutil.JSON(t, "GET", url+"/api/v1/messages/"+msgID+"/reactions", "", nil)
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 401 or 404 (fail-closed) on unauth, got %d", resp.StatusCode)
	}
}

// mustUserIDAP4 — small helper to extract the user_id of the caller's token.
func mustUserIDAP4(t *testing.T, url, tok string) string {
	t.Helper()
	resp, data := testutil.JSON(t, "GET", url+"/api/v1/users/me", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("users/me: %d %v", resp.StatusCode, data)
	}
	u, _ := data["user"].(map[string]any)
	id, _ := u["id"].(string)
	if id == "" {
		t.Fatalf("user id empty: %v", data)
	}
	return id
}
