package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func TestAPIKeyAuth(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	users, _ := s.ListUsers()
	var adminID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			adminID = u.ID
			break
		}
	}

	apiKey, _ := store.GenerateAPIKey()
	s.SetAPIKey(adminID, apiKey)

	t.Run("BearerAPIKeyChannels", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/channels", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("BearerAPIKeyMe", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	_ = adminToken
}

func TestAuthRegisterValidation(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)

	users, _ := s.ListUsers()
	var adminID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			adminID = u.ID
			break
		}
	}
	_ = adminID

	t.Run("ShortPassword", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/auth/register", "", map[string]string{
			"invite_code": "test-invite", "email": "short@test.com", "password": "short", "display_name": "Short",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/auth/register", "", map[string]string{
			"invite_code": "test-invite", "email": "invalidemail", "password": "password123", "display_name": "Invalid",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("MissingFields", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/auth/register", "", map[string]string{
			"email": "x@test.com",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ValidRegistration", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/auth/register", "", map[string]string{
			"invite_code": "test-invite", "email": "valid@test.com", "password": "password123", "display_name": "Valid",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
	})
}

func TestDisabledUserCantLogin(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	users, _ := s.ListUsers()
	var memberID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "member@test.com" {
			memberID = u.ID
			break
		}
	}

	testutil.JSON(t, "PATCH", ts.URL+"/admin-api/v1/users/"+memberID, adminToken, map[string]any{"disabled": true})

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/auth/login", "", map[string]string{
		"email": "member@test.com", "password": "password123",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for disabled user, got %d", resp.StatusCode)
	}

	testutil.JSON(t, "PATCH", ts.URL+"/admin-api/v1/users/"+memberID, adminToken, map[string]any{"disabled": false})
}

func TestWorkspaceUpdate(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	ch := testutil.CreateChannel(t, ts.URL, adminToken, "ws-update-test", "public")
	chID := ch["id"].(string)

	status, data := uploadWorkspaceFile(t, ts.URL, adminToken, chID, "update-me.txt", "original")
	if status != http.StatusCreated {
		t.Fatalf("upload failed: %d", status)
	}
	fileID := data["file"].(map[string]any)["id"].(string)

	t.Run("UpdateContent", func(t *testing.T) {
		resp, rData := testutil.JSON(t, "PUT", fmt.Sprintf("%s/api/v1/channels/%s/workspace/files/%s", ts.URL, chID, fileID), adminToken, map[string]string{
			"content": "updated content",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, rData)
		}
	})

	t.Run("MoveFile", func(t *testing.T) {
		dirResp, dirData := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/workspace/mkdir", adminToken, map[string]string{
			"name": "move-target",
		})
		if dirResp.StatusCode != http.StatusCreated {
			t.Fatalf("mkdir failed: %d", dirResp.StatusCode)
		}
		dirID := dirData["file"].(map[string]any)["id"].(string)

		resp, _ := testutil.JSON(t, "POST", fmt.Sprintf("%s/api/v1/channels/%s/workspace/files/%s/move", ts.URL, chID, fileID), adminToken, map[string]any{
			"parentId": dirID,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestChannelMemberOperations(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	users, _ := s.ListUsers()
	var memberID string
	// ADM-0.3: testutil seeds owner + admin + member as role='member' user-rail
	// fixtures. Pick the member fixture by display name so DM-self-paths skip.
	for _, u := range users {
		if u.Role == "member" && u.DisplayName == "Member" {
			memberID = u.ID
			break
		}
	}

	t.Run("CannotRemoveFromGeneral", func(t *testing.T) {
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
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+generalID+"/members/"+memberID, adminToken, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("CannotJoinDM", func(t *testing.T) {
		// Create DM first
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("dm creation failed: %d", resp.StatusCode)
		}
		dmID := data["channel"].(map[string]any)["id"].(string)
		resp2, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+dmID+"/join", adminToken, nil)
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for joining DM, got %d", resp2.StatusCode)
		}
	})

	t.Run("CannotLeaveDM", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Skip()
		}
		dmID := data["channel"].(map[string]any)["id"].(string)
		resp2, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+dmID+"/leave", adminToken, nil)
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for leaving DM, got %d", resp2.StatusCode)
		}
	})

	t.Run("CannotDeleteDM", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Skip()
		}
		dmID := data["channel"].(map[string]any)["id"].(string)
		resp2, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+dmID, adminToken, nil)
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for deleting DM, got %d", resp2.StatusCode)
		}
	})

	t.Run("PrivateChannelAccessControl", func(t *testing.T) {
		privCh := testutil.CreateChannel(t, ts.URL, adminToken, "access-control-test", "private")
		privID := privCh["id"].(string)
		memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

		// AP-1 设计 ① (REG-CHN1-007): 非 member GET → 403.
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+privID, memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}

		// Member cannot list messages in private channel
		resp2, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+privID+"/messages", memberToken, nil)
		if resp2.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp2.StatusCode)
		}
	})
}

func TestMessageInPrivateChannel(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	privCh := testutil.CreateChannel(t, ts.URL, adminToken, "priv-msg-test", "private")
	privID := privCh["id"].(string)

	// Admin can post (they're the creator/member)
	msg := testutil.PostMessage(t, ts.URL, adminToken, privID, "private hello")
	if msg["id"] == nil {
		t.Fatal("expected message")
	}

	// Member cannot post (not a member)
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+privID+"/messages", memberToken, map[string]string{
		"content": "should fail",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for non-member posting to private, got %d", resp.StatusCode)
	}
}

func TestTopicValidation(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

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

	longTopic := ""
	for i := 0; i < 260; i++ {
		longTopic += "x"
	}
	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+generalID+"/topic", adminToken, map[string]string{
		"topic": longTopic,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for long topic, got %d", resp.StatusCode)
	}
}

func TestChannelCreationValidation(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	t.Run("EmptyName", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", adminToken, map[string]string{
			"name": "", "visibility": "public",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidVisibility", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", adminToken, map[string]string{
			"name": "vis-test", "visibility": "secret",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestInvalidContentType(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

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

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]any{
		"content":      "test",
		"content_type": "invalid",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
