package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestAgentsCRUD(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	var agentID string

	t.Run("CreateAgent", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", adminToken, map[string]any{
			"display_name": "TestBot",
			"permissions": []map[string]string{
				{"permission": "message.send", "scope": "*"},
			},
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
		agent := data["agent"].(map[string]any)
		agentID = agent["id"].(string)
		if agent["api_key"] == nil {
			t.Fatal("expected api_key")
		}
	})

	t.Run("CreateAgentMissingName", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", adminToken, map[string]string{})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ListAgentsAsAdmin", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		agents := data["agents"].([]any)
		if len(agents) == 0 {
			t.Fatal("expected at least 1 agent")
		}
		// F7 (#1108): list reads MUST redact the full key. Every row carries
		// api_key_last4 (4-char string) and never the full api_key.
		for _, a := range agents {
			agent := a.(map[string]any)
			if agent["api_key"] != nil {
				t.Fatalf("list leaked full api_key: %v", agent["api_key"])
			}
			last4, ok := agent["api_key_last4"].(string)
			if !ok || len(last4) != 4 {
				t.Fatalf("expected api_key_last4 4-char string, got %v", agent["api_key_last4"])
			}
		}
	})

	t.Run("ListAgentsAsMember", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents", memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		agents := data["agents"].([]any)
		if len(agents) != 0 {
			t.Fatal("member should see 0 agents (not owner)")
		}
	})

	t.Run("MemberCreatesAgent", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", memberToken, map[string]any{
			"display_name": "MemberBot",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
		agent := data["agent"].(map[string]any)
		memberAgentID := agent["id"].(string)

		// Member can see own agents
		resp2, data2 := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents", memberToken, nil)
		if resp2.StatusCode != http.StatusOK {
			t.Fatal("list failed")
		}
		agents := data2["agents"].([]any)
		if len(agents) == 0 {
			t.Fatal("member should see own agent")
		}

		// Member can get own agent
		resp3, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+memberAgentID, memberToken, nil)
		if resp3.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp3.StatusCode)
		}

		// Member cannot get admin's agent
		resp4, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, memberToken, nil)
		if resp4.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp4.StatusCode)
		}

		// Cleanup
		testutil.JSON(t, "DELETE", ts.URL+"/api/v1/agents/"+memberAgentID, memberToken, nil)
	})

	t.Run("GetAgent", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		agent := data["agent"].(map[string]any)
		if agent["display_name"] != "TestBot" {
			t.Fatalf("expected TestBot, got %v", agent["display_name"])
		}
		// F7 (#1108): GET read MUST redact. No full api_key; only the 4-char
		// api_key_last4 mask source.
		if agent["api_key"] != nil {
			t.Fatalf("GET leaked full api_key: %v", agent["api_key"])
		}
		last4, ok := agent["api_key_last4"].(string)
		if !ok || len(last4) != 4 {
			t.Fatalf("expected api_key_last4 4-char string, got %v", agent["api_key_last4"])
		}
	})

	t.Run("PatchAgentRedacted", func(t *testing.T) {
		// F7 (#1108): the PATCH read-back MUST redact identically to list/get.
		// A successful mutation (require_mention) returns the updated agent;
		// that response carries only api_key_last4 (4-char string) and never
		// the full api_key. Fails red if handlePatchAgent uses the full-key
		// sanitizeAgentWithKey helper instead of the redacted sanitizeAgent.
		resp, data := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, adminToken, map[string]any{
			"require_mention": true,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}
		agent := data["agent"].(map[string]any)
		if agent["require_mention"] != true {
			t.Fatalf("expected require_mention=true after patch, got %v", agent["require_mention"])
		}
		if agent["api_key"] != nil {
			t.Fatalf("PATCH leaked full api_key: %v", agent["api_key"])
		}
		last4, ok := agent["api_key_last4"].(string)
		if !ok || len(last4) != 4 {
			t.Fatalf("expected api_key_last4 4-char string, got %v", agent["api_key_last4"])
		}
	})

	t.Run("RevealAPIKeyOwner", func(t *testing.T) {
		// F7 (#1108): owner POSTs reveal-api-key → 200 + full api_key. The
		// stored key is unchanged (reveal ≠ rotate), so the returned key's
		// last4 must equal the redacted last4 from the GET read.
		_, getData := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, adminToken, nil)
		getAgent := getData["agent"].(map[string]any)
		wantLast4 := getAgent["api_key_last4"].(string)

		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents/"+agentID+"/reveal-api-key", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}
		key, ok := data["api_key"].(string)
		if !ok || key == "" {
			t.Fatalf("expected full api_key, got %v", data["api_key"])
		}
		if key[len(key)-4:] != wantLast4 {
			t.Fatalf("reveal key last4 %q != GET mask last4 %q", key[len(key)-4:], wantLast4)
		}
	})

	t.Run("RevealAPIKeyNonOwner", func(t *testing.T) {
		// F7 (#1108): non-owner POST reveal-api-key → 403 (anti-IDOR, identical
		// gate to rotate). Reuses the cross-owner memberToken fixture against
		// the admin-owned agentID.
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents/"+agentID+"/reveal-api-key", memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("GetAgentNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/nonexistent", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("RotateAPIKey", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents/"+agentID+"/rotate-api-key", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["api_key"] == nil {
			t.Fatal("expected api_key")
		}
	})

	t.Run("GetPermissions", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/permissions", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["permissions"] == nil {
			t.Fatal("expected permissions")
		}
	})

	t.Run("SetPermissions", func(t *testing.T) {
		resp, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/agents/"+agentID+"/permissions", adminToken, map[string]any{
			"permissions": []map[string]string{
				{"permission": "message.send", "scope": "*"},
				{"permission": "channel.create", "scope": "*"},
			},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}
		perms := data["permissions"].([]any)
		if len(perms) != 2 {
			t.Fatalf("expected 2 permissions, got %d", len(perms))
		}
	})

	t.Run("DeleteAgent", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/agents/"+agentID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteAgentNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/agents/nonexistent", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestChannelGroupsCRUD(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	var groupID string

	t.Run("CreateGroup", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{
			"name": "Engineering",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		group := data["group"].(map[string]any)
		groupID = group["id"].(string)
	})

	t.Run("CreateGroupEmptyName", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{
			"name": "",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("ListGroups", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channel-groups", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		groups := data["groups"].([]any)
		if len(groups) == 0 {
			t.Fatal("expected at least 1 group")
		}
	})

	t.Run("UpdateGroup", func(t *testing.T) {
		resp, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channel-groups/"+groupID, adminToken, map[string]string{
			"name": "Engineering Team",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		group := data["group"].(map[string]any)
		if group["name"] != "Engineering Team" {
			t.Fatalf("expected Engineering Team, got %v", group["name"])
		}
	})

	t.Run("NonCreatorCannotUpdate", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channel-groups/"+groupID, memberToken, map[string]string{
			"name": "Hacked",
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("ReorderGroup", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channel-groups/reorder", adminToken, map[string]any{
			"group_id": groupID,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		resp, data := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channel-groups/"+groupID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}
	})

	t.Run("DeleteGroupNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channel-groups/nonexistent", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestChannelAdvanced(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	users, _ := s.ListUsers()
	var adminID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			adminID = u.ID
			break
		}
	}

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

	t.Run("SetTopic", func(t *testing.T) {
		resp, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+generalID+"/topic", adminToken, map[string]string{
			"topic": "Hello Topic",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		ch := data["channel"].(map[string]any)
		if ch["topic"] != "Hello Topic" {
			t.Fatalf("expected Hello Topic, got %v", ch["topic"])
		}
	})

	t.Run("MarkChannelRead", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+generalID+"/read", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("ListMembers", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/members", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		members := data["members"].([]any)
		if len(members) < 2 {
			t.Fatalf("expected at least 2 members, got %d", len(members))
		}
	})

	t.Run("PreviewPublicChannel", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/preview", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["channel"] == nil {
			t.Fatal("expected channel in preview")
		}
	})

	t.Run("CannotLeaveGeneral", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/leave", memberToken, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("CannotDeleteGeneral", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+generalID, adminToken, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteChannelIdempotent", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, adminToken, "del-test", "public")
		chID := ch["id"].(string)
		resp1, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+chID, adminToken, nil)
		if resp1.StatusCode != http.StatusOK {
			t.Fatalf("first delete expected 200, got %d", resp1.StatusCode)
		}
		req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/channels/"+chID, nil)
		req.AddCookie(&http.Cookie{Name: "borgee_token", Value: adminToken})
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp2, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusNoContent {
			t.Fatalf("idempotent delete expected 204, got %d", resp2.StatusCode)
		}
	})

	t.Run("AddMemberToPrivateChannel", func(t *testing.T) {
		privCh := testutil.CreateChannel(t, ts.URL, adminToken, "priv-member-test", "private")
		privID := privCh["id"].(string)
		memberID := mustUserByEmail(t, s, "member@test.com").ID

		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+privID+"/members", adminToken, map[string]string{
			"user_id": memberID,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		// Remove member
		resp2, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+privID+"/members/"+memberID, adminToken, nil)
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp2.StatusCode)
		}
	})

	t.Run("ReorderChannel", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, adminToken, "reorder-test", "public")
		chID := ch["id"].(string)
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/reorder", adminToken, map[string]any{
			"channel_id": chID,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("CreatePrivateChannel", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", adminToken, map[string]any{
			"name":       "private-ch",
			"visibility": "private",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
	})

	t.Run("PreviewPrivateChannel404", func(t *testing.T) {
		privCh := testutil.CreateChannel(t, ts.URL, adminToken, "priv-preview", "private")
		privID := privCh["id"].(string)
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+privID+"/preview", memberToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("UpdateChannelVisibility", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, adminToken, "vis-test", "public")
		chID := ch["id"].(string)
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID, adminToken, map[string]any{
			"visibility": "private",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("RenameChannel", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, adminToken, "rename-ch-test", "public")
		chID := ch["id"].(string)
		resp, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+chID, adminToken, map[string]any{
			"name": "renamed-channel",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		rCh := data["channel"].(map[string]any)
		if rCh["name"] != "renamed-channel" {
			t.Fatalf("expected renamed-channel, got %v", rCh["name"])
		}
	})

	_ = adminID
}

func TestUsersEndpoints(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	t.Run("ListUsersRemoved", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/users", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 (removed), got %d", resp.StatusCode)
		}
	})

	t.Run("MyPermissionsOwner", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/permissions", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		perms := data["permissions"].([]any)
		if len(perms) == 0 {
			t.Fatal("expected permissions")
		}
	})

	t.Run("MyPermissionsMember", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/permissions", memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		perms := data["permissions"].([]any)
		if len(perms) == 0 {
			t.Fatal("expected member permissions")
		}
	})

	t.Run("OnlineUsers", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/online", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["user_ids"] == nil {
			t.Fatal("expected user_ids")
		}
	})

	t.Run("Commands", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/commands", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["builtin"] == nil {
			t.Fatal("expected builtin commands")
		}
	})

	t.Run("Health", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestMessageAdvanced(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

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

	t.Run("ReplyToMessage", func(t *testing.T) {
		msg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "parent msg")
		parentID := msg["id"].(string)

		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]any{
			"content":     "reply msg",
			"reply_to_id": parentID,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		reply := data["message"].(map[string]any)
		if reply["reply_to_id"] != parentID {
			t.Fatalf("expected reply_to_id %s, got %v", parentID, reply["reply_to_id"])
		}
	})

	t.Run("SearchMessages", func(t *testing.T) {
		testutil.PostMessage(t, ts.URL, adminToken, generalID, "unique-search-token-xyz")

		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages/search?q=unique-search-token-xyz", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		msgs := data["messages"].([]any)
		if len(msgs) == 0 {
			t.Fatal("expected at least 1 search result")
		}
	})

	t.Run("SearchEmptyQuery", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages/search?q=", adminToken, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("AdminCanDeleteOtherMessage", func(t *testing.T) {
		// ADM-0.3: no user-rail admin override. Even an "admin"-named test
		// fixture (now role='member') can only delete its own messages on
		// /api/v1/messages/*; cross-user delete must use /admin-api/v1.
		msg := testutil.PostMessage(t, ts.URL, memberToken, generalID, "member-msg-for-admin-del")
		msgID := msg["id"].(string)
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID, adminToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 (sender-only delete on user-rail), got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteNonexistentMessage", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/nonexistent", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("MessageWithMentions", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]any{
			"content":  "hello @Member",
			"mentions": []string{},
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		msg := data["message"].(map[string]any)
		if msg["content"] != "hello @Member" {
			t.Fatalf("expected content with mention")
		}
	})

	t.Run("PaginationAfter", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			testutil.PostMessage(t, ts.URL, adminToken, generalID, fmt.Sprintf("after-test-%d", i))
		}
		_, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages?limit=2", adminToken, nil)
		msgs := data["messages"].([]any)
		if len(msgs) == 0 {
			t.Fatal("expected messages")
		}
		// Use after=0 to get all from the beginning
		_, data2 := testutil.JSON(t, "GET", fmt.Sprintf("%s/api/v1/channels/%s/messages?after=0&limit=2", ts.URL, generalID), adminToken, nil)
		msgs2 := data2["messages"].([]any)
		if len(msgs2) == 0 {
			t.Fatal("expected messages with after cursor")
		}
	})

	t.Run("EditDeletedMessage", func(t *testing.T) {
		msg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "to-edit-deleted")
		msgID := msg["id"].(string)
		testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID, adminToken, nil)
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msgID, adminToken, map[string]string{
			"content": "edited",
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("MessageImageContentType", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]any{
			"content":      "http://example.com/img.png",
			"content_type": "image",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}
	})
}

func TestReactionsAdvanced(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

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

	msg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "reaction test advanced")
	msgID := msg["id"].(string)

	t.Run("MultipleReactionsSameMessage", func(t *testing.T) {
		testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msgID+"/reactions", adminToken, map[string]string{"emoji": "👍"})
		testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/"+msgID+"/reactions", memberToken, map[string]string{"emoji": "❤️"})

		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/messages/"+msgID+"/reactions", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		reactions := data["reactions"].([]any)
		if len(reactions) < 2 {
			t.Fatalf("expected at least 2 reactions, got %d", len(reactions))
		}
	})

	t.Run("AddReactionToNonexistent", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/nonexistent/reactions", adminToken, map[string]string{"emoji": "👍"})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("RemoveNonexistentReaction", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID+"/reactions", adminToken, map[string]string{"emoji": "🤷"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}
