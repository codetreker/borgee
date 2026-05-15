package api_test

import (
	"net/http"
	"net/url"
	"testing"

	"borgee-server/internal/testutil"
)

func TestP0MessageCRUD(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	channelID := testutil.GetGeneralChannelID(t, ts.URL, adminToken)
	memberID := testutil.GetUserIDByName(t, ts.URL, adminToken, "Member")

	root := testutil.PostMessage(t, ts.URL, adminToken, channelID, "root searchable message")
	rootID := stringField(t, root, "id")

	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+channelID+"/messages", adminToken, map[string]any{
		"content":     "reply mentions <@" + memberID + "> @Member",
		"reply_to_id": rootID,
	})
	requireStatus(t, resp, http.StatusCreated, data)
	reply := data["message"].(map[string]any)
	replyID := stringField(t, reply, "id")
	if reply["reply_to_id"] != rootID {
		t.Fatalf("expected reply_to_id %s, got %v", rootID, reply["reply_to_id"])
	}
	mentions := reply["mentions"].([]any)
	if len(mentions) != 1 || mentions[0] != memberID {
		t.Fatalf("expected deduplicated mention %s, got %v", memberID, mentions)
	}

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID+"/messages?limit=20", adminToken, nil)
	requireStatus(t, resp, http.StatusOK, data)
	messages := data["messages"].([]any)
	if !containsObjectWithID(messages, rootID) || !containsObjectWithID(messages, replyID) {
		t.Fatalf("expected root and reply in list, got %v", messages)
	}

	searchURL := ts.URL + "/api/v1/channels/" + channelID + "/messages/search?q=" + url.QueryEscape("searchable")
	resp, data = testutil.JSON(t, http.MethodGet, searchURL, adminToken, nil)
	requireStatus(t, resp, http.StatusOK, data)
	if results := data["messages"].([]any); !containsObjectWithID(results, rootID) {
		t.Fatalf("expected search result %s, got %v", rootID, results)
	}

	resp, data = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/messages/"+rootID, adminToken, map[string]string{"content": "edited searchable message"})
	requireStatus(t, resp, http.StatusOK, data)
	if data["message"].(map[string]any)["content"] != "edited searchable message" {
		t.Fatalf("message was not edited: %v", data)
	}

	resp, data = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/messages/"+rootID, memberToken, map[string]string{"content": "not allowed"})
	requireStatus(t, resp, http.StatusForbidden, data)

	// ADM-0.3: user-rail message delete is sender-only. The admin@test.com
	// fixture is now role='member' on the user-rail and cannot cross-delete.
	memberMsg := testutil.PostMessage(t, ts.URL, memberToken, channelID, "member owned")
	resp, data = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/messages/"+stringField(t, memberMsg, "id"), adminToken, nil)
	requireStatus(t, resp, http.StatusForbidden, data)
	resp, data = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/messages/"+stringField(t, memberMsg, "id"), memberToken, nil)
	requireStatus(t, resp, http.StatusNoContent, data)

	resp, data = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/messages/"+replyID, adminToken, nil)
	requireStatus(t, resp, http.StatusNoContent, data)
	resp, data = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/messages/"+replyID, adminToken, map[string]string{"content": "after delete"})
	requireStatus(t, resp, http.StatusNotFound, data)
}
