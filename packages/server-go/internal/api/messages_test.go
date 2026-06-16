package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestMessageCRUD(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Get general channel ID
	_, chData := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels", adminToken, nil)
	channels := chData["channels"].([]any)
	var generalID string
	for _, c := range channels {
		cm := c.(map[string]any)
		if cm["name"] == "general" {
			generalID = cm["id"].(string)
			break
		}
	}
	if generalID == "" {
		t.Fatal("general channel not found")
	}
	_ = s

	var messageID string
	var memberMsgID string

	t.Run("CreateMessage", func(t *testing.T) {
		msg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "hello world")
		messageID = msg["id"].(string)
		if messageID == "" {
			t.Fatal("expected message id")
		}
	})

	t.Run("ListMessages", func(t *testing.T) {
		_, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, nil)
		msgs, ok := data["messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Fatal("expected messages")
		}
	})

	t.Run("EditMessage", func(t *testing.T) {
		_, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+messageID, adminToken, map[string]string{
			"content": "edited content",
		})
		msg := data["message"].(map[string]any)
		if msg["content"] != "edited content" {
			t.Fatalf("expected edited content, got %v", msg["content"])
		}
	})

	t.Run("DeleteMessage", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/"+messageID, adminToken, nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
	})

	t.Run("EditOtherUserMessage", func(t *testing.T) {
		msg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "admin msg")
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msg["id"].(string), memberToken, map[string]string{
			"content": "hacked",
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteOtherUserMessage", func(t *testing.T) {
		memberMsgID = ""
		msg := testutil.PostMessage(t, ts.URL, memberToken, generalID, "member msg")
		memberMsgID = msg["id"].(string)

		// admin posted msg, member tries to delete
		adminMsg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "admin only msg")
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/"+adminMsg["id"].(string), memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
		_ = memberMsgID
	})

	t.Run("Pagination", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			testutil.PostMessage(t, ts.URL, adminToken, generalID, fmt.Sprintf("page msg %d", i))
		}

		_, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages?limit=2", adminToken, nil)
		msgs := data["messages"].([]any)
		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
		hasMore, _ := data["has_more"].(bool)
		if !hasMore {
			t.Fatal("expected has_more=true")
		}

		firstMsg := msgs[0].(map[string]any)
		createdAt := firstMsg["created_at"].(float64)
		_, data2 := testutil.JSON(t, "GET", fmt.Sprintf("%s/api/v1/channels/%s/messages?limit=2&before=%d", ts.URL, generalID, int64(createdAt)), adminToken, nil)
		msgs2 := data2["messages"].([]any)
		if len(msgs2) == 0 {
			t.Fatal("expected messages with before cursor")
		}
	})

	t.Run("EmptyContent", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]string{
			"content": "",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	// borgee #1108 F5: image content_type must carry an http(s) URL or a
	// same-origin relative path. javascript:/data:/protocol-relative → 400
	// INVALID_CONTENT; valid http(s) URL + leading-slash relative → 201.
	t.Run("ImageContentRejectsJavascriptScheme", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]string{
			"content":      "javascript:alert(1)",
			"content_type": "image",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		if data["code"] != "INVALID_CONTENT" {
			t.Fatalf("expected code INVALID_CONTENT, got %v", data["code"])
		}
	})

	t.Run("ImageContentRejectsDataScheme", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]string{
			"content":      "data:text/html,<script>alert(1)</script>",
			"content_type": "image",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		if data["code"] != "INVALID_CONTENT" {
			t.Fatalf("expected code INVALID_CONTENT, got %v", data["code"])
		}
	})

	t.Run("ImageContentRejectsProtocolRelative", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]string{
			"content":      "//evil.com/x.png",
			"content_type": "image",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		if data["code"] != "INVALID_CONTENT" {
			t.Fatalf("expected code INVALID_CONTENT, got %v", data["code"])
		}
	})

	t.Run("ImageContentAcceptsHttpsURL", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]string{
			"content":      "https://example.com/x.png",
			"content_type": "image",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
	})

	t.Run("ImageContentAcceptsRelativePath", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]string{
			"content":      "/api/uploads/x.png",
			"content_type": "image",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
	})
}
