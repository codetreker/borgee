// Package api — messages_acl_audit_test.go: AP-5 unit tests for
// post-removal ACL checks on PUT/DELETE /api/v1/messages/{id} +
// PATCH /api/v1/channels/{id}/messages/{id} (DM-4).
//
// AP-5 assertions (跟 docs/implementation/modules/ap-5-spec.md §0):
//  ① 3 handler 各加 IsChannelMember + CanAccessChannel membership checks
//    after removal.
//  ② cross-org 403 先于 channel-member 404 (TestCrossOrgRead403 lock).
//  ③ 既有 sender_id check 不破 (member-but-non-sender 仍 403).
//  ④ 0 schema 改 + 0 新错码 — 复用 messages.go 既有 "Channel not found".
//
// 跟 AP-4 reactions_acl_test.go 同模式 (AP-4 #551 reactions ACL gap 闭合
// → AP-5 #553 messages 三 endpoint 闭合).

package api

import (
	"net/http"
	"testing"
)

// TestAP_PutMessage_PostRemovalReject — sender removed from public channel
// can no longer PUT-edit own message there. Expect 404.
func TestAP_PutMessage_PostRemovalReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")

	ch := createCh(t, ts.URL, adminToken, "ap5-put-removal", "public")
	chID := ch["id"].(string)
	if resp, body := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/join", memberToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("join: %d %v", resp.StatusCode, body)
	}
	msg := postMsg(t, ts.URL, memberToken, chID, "before removal")
	msgID := msg["id"].(string)

	// member leaves channel — post-removal state.
	if resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/leave", memberToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("leave failed: %d", resp.StatusCode)
	}

	resp, body := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msgID, memberToken, map[string]string{"content": "after removal"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expect 404, got %d: %v", resp.StatusCode, body)
	}
}

// TestAP_DeleteMessage_PostRemovalReject — sender removed from channel
// can no longer DELETE own message. Expect 404.
func TestAP_DeleteMessage_PostRemovalReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")

	ch := createCh(t, ts.URL, adminToken, "ap5-del-removal", "public")
	chID := ch["id"].(string)
	if resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/join", memberToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("join failed")
	}
	msg := postMsg(t, ts.URL, memberToken, chID, "to delete")
	msgID := msg["id"].(string)
	if resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/leave", memberToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("leave failed")
	}

	resp, body := jsonReq(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID, memberToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expect 404, got %d: %v", resp.StatusCode, body)
	}
}

// TestAP_Member_PutDelete_OK — channel member sender can still
// PUT/DELETE own message (既有行为不破).
func TestAP_Member_PutDelete_OK(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")

	ch := createCh(t, ts.URL, adminToken, "ap5-member-flow", "public")
	chID := ch["id"].(string)
	msg := postMsg(t, ts.URL, adminToken, chID, "owned")
	msgID := msg["id"].(string)

	resp, body := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msgID, adminToken, map[string]string{"content": "edited"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT expect 200, got %d: %v", resp.StatusCode, body)
	}

	resp, body = jsonReq(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID, adminToken, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE expect 204, got %d: %v", resp.StatusCode, body)
	}
}

// TestAP_NonSenderMember_403 — channel member who is NOT the
// sender still gets 403 from existing sender_id check (sender-only ACL
// 不破; matches existing PUT/DELETE messages behavior).
func TestAP_NonSenderMember_403(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")

	ch := createCh(t, ts.URL, adminToken, "ap5-non-sender", "public")
	chID := ch["id"].(string)
	if resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/join", memberToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("join failed")
	}
	msg := postMsg(t, ts.URL, adminToken, chID, "owner-msg")
	msgID := msg["id"].(string)

	// member is in channel but is NOT the sender → existing sender_id
	// check returns 403 (post AP-5 channel-member check which would
	// pass since member is in channel).
	resp, body := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msgID, memberToken, map[string]string{"content": "x"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT non-sender expect 403, got %d: %v", resp.StatusCode, body)
	}

	resp, body = jsonReq(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID, memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE non-sender expect 403, got %d: %v", resp.StatusCode, body)
	}
}

// TestAP_PatchDM_PostRemovalReject — DM-4 PATCH endpoint after sender
// removed from DM channel returns 404 (channel-member check, 跟 messages
// PUT/DELETE 同模式).
func TestAP_PatchDM_PostRemovalReject(t *testing.T) {
	t.Parallel()
	ts, st, _ := setupFullTestServer(t)
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")

	// Resolve owner + member ids via store.
	users, _ := st.ListUsers()
	var memberID, ownerID string
	for _, u := range users {
		if u.Role == "member" && memberID == "" {
			memberID = u.ID
		}
		if u.Role == "admin" && ownerID == "" {
			ownerID = u.ID
		}
	}
	if memberID == "" || ownerID == "" {
		t.Skip("missing owner/member fixture user")
	}

	// member opens DM with owner.
	resp, dm := jsonReq(t, "POST", ts.URL+"/api/v1/dm/"+ownerID, memberToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Skipf("DM open: %d %v", resp.StatusCode, dm)
	}
	chID, _ := dm["channel_id"].(string)
	if chID == "" {
		if c, ok := dm["channel"].(map[string]any); ok {
			chID, _ = c["id"].(string)
		}
	}
	if chID == "" {
		t.Skipf("no channel id in DM response: %v", dm)
	}

	msg := postMsg(t, ts.URL, memberToken, chID, "dm before removal")
	msgID := msg["id"].(string)

	if err := st.RemoveChannelMember(chID, memberID); err != nil {
		t.Fatalf("RemoveChannelMember: %v", err)
	}

	resp, body := jsonReq(t, "PATCH", ts.URL+"/api/v1/channels/"+chID+"/messages/"+msgID, memberToken, map[string]string{"content": "after removal"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expect 404, got %d: %v", resp.StatusCode, body)
	}
}
