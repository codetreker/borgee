package api_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func readSSEUntil(t *testing.T, resp *http.Response, want string) string {
	t.Helper()
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var b strings.Builder
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SSE content %q; got %q", want, b.String())
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				continue
			}
			t.Fatalf("read SSE: %v", err)
		}
		b.WriteString(line)
		if strings.Contains(b.String(), want) {
			return b.String()
		}
	}
}

func openSSE(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected SSE 200, got %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		resp.Body.Close()
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	return resp
}

func TestSSEStream(t *testing.T) {
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
	apiKey, _ := store.GenerateAPIKey()
	s.SetAPIKey(adminID, apiKey)

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

	t.Run("SSEStreamWithBearerKey", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/stream", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp := openSSE(t, req)
		readSSEUntil(t, resp, ":connected")
	})

	t.Run("SSEStreamWithQueryKey", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/stream?api_key="+apiKey, nil)
		resp := openSSE(t, req)
		readSSEUntil(t, resp, ":connected")
	})

	t.Run("SSEStreamWithCookie", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/stream", nil)
		req.AddCookie(&http.Cookie{Name: "borgee_token", Value: adminToken})
		resp := openSSE(t, req)
		readSSEUntil(t, resp, ":connected")
	})

	t.Run("SSEStreamWithLastEventID", func(t *testing.T) {
		testutil.PostMessage(t, ts.URL, adminToken, generalID, "sse-backfill-test")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/stream", nil)
		req.AddCookie(&http.Cookie{Name: "borgee_token", Value: adminToken})
		req.Header.Set("Last-Event-ID", "0")
		resp := openSSE(t, req)
		got := readSSEUntil(t, resp, "event: new_message")
		if !strings.Contains(got, "sse-backfill-test") {
			t.Fatalf("expected backfill message in SSE stream, got %q", got)
		}
	})

	t.Run("PollWithSinceID", func(t *testing.T) {
		msg := testutil.PostMessage(t, ts.URL, adminToken, generalID, "since-id-test")
		msgID := msg["id"].(string)

		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/poll", adminToken, map[string]any{
			"since_id":   msgID,
			"timeout_ms": 0,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		_ = data
	})
}

func TestMemberNonAdminEndpoints(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	t.Run("MemberCannotAccessAdmin", func(t *testing.T) {
		endpoints := []struct {
			method string
			path   string
		}{
			{"GET", "/admin-api/v1/users"},
			{"GET", "/admin-api/v1/invites"},
			{"GET", "/admin-api/v1/channels"},
		}
		for _, ep := range endpoints {
			resp, _ := testutil.JSON(t, ep.method, ts.URL+ep.path, memberToken, nil)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s %s: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
			}
		}
	})
}

func TestSearchInPrivateChannel(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	privCh := testutil.CreateChannel(t, ts.URL, adminToken, "search-priv", "private")
	privID := privCh["id"].(string)

	testutil.PostMessage(t, ts.URL, adminToken, privID, "searchable content")

	// Admin can search
	resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+privID+"/messages/search?q=searchable", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Member cannot search in channel they're not in
	resp2, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+privID+"/messages/search?q=searchable", memberToken, nil)
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp2.StatusCode)
	}
}

func TestChannelReorderWithGroup(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Create group
	_, gData := testutil.JSON(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{"name": "Reorder Group"})
	groupID := gData["group"].(map[string]any)["id"].(string)

	ch := testutil.CreateChannel(t, ts.URL, adminToken, "reorder-grp-ch", "public")
	chID := ch["id"].(string)

	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/reorder", adminToken, map[string]any{
		"channel_id": chID,
		"group_id":   groupID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGroupNameTooLong(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	longName := ""
	for i := 0; i < 60; i++ {
		longName += "x"
	}
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{"name": longName})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for long group name, got %d", resp.StatusCode)
	}
}

func TestAdminChannelsDMCreation(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	users, _ := s.ListUsers()
	var memberID string
	for _, u := range users {
		if u.Role == "member" {
			memberID = u.ID
			break
		}
	}

	t.Run("CannotAddMemberToDM", func(t *testing.T) {
		testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		_, dmData := testutil.JSON(t, "GET", ts.URL+"/api/v1/dm", adminToken, nil)
		dms := dmData["channels"].([]any)
		if len(dms) == 0 {
			t.Skip("no DMs")
		}
		dmID := dms[0].(map[string]any)["id"].(string)

		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+dmID+"/members", adminToken, map[string]string{
			"user_id": memberID,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for adding member to DM, got %d", resp.StatusCode)
		}
	})
}

func TestCreateChannelWithMemberIDs(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	users, _ := s.ListUsers()
	var memberID string
	for _, u := range users {
		if u.Role == "member" {
			memberID = u.ID
			break
		}
	}

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", adminToken, map[string]any{
		"name":       "with-members",
		"visibility": "private",
		"member_ids": []string{memberID},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
	}
}

func TestCreateChannelWithTopic(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", adminToken, map[string]any{
		"name":  "with-topic",
		"topic": "A nice topic",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
	}
	ch := data["channel"].(map[string]any)
	_ = ch
}

func TestMessageEditNotFound(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/messages/nonexistent", adminToken, map[string]string{
		"content": "test",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWorkspaceNonMember(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	privCh := testutil.CreateChannel(t, ts.URL, adminToken, "ws-nonmember", "private")
	privID := privCh["id"].(string)

	resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+privID+"/workspace", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestMessageCommandContentType(t *testing.T) {
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

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]any{
		"content":      "/help",
		"content_type": "command",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
	}
}

func TestAdminUpdateUserNotFound(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/admin-api/v1/users/nonexistent/api-key", adminToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestMessageWithExplicitMentionIDs(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	users, _ := s.ListUsers()
	var memberID string
	for _, u := range users {
		if u.Role == "member" {
			memberID = u.ID
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

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/messages", adminToken, map[string]any{
		"content": fmt.Sprintf("hello <@%s>", memberID),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
	}
	msg := data["message"].(map[string]any)
	mentions := msg["mentions"].([]any)
	if len(mentions) == 0 {
		t.Fatal("expected mentions")
	}
}

func TestRemoteNodeNotFoundScenarios(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	t.Run("ListBindingsNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/nonexistent/bindings", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("CreateBindingNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/remote/nodes/nonexistent/bindings", adminToken, map[string]string{
			"channel_id": "ch", "path": "/",
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("DeleteBindingNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/remote/nodes/nonexistent/bindings/x", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("LsNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/nonexistent/ls?path=/", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("ReadNotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/remote/nodes/nonexistent/read?path=/", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}
