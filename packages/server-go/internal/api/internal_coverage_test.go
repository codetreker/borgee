package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"borgee-server/internal/auth"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

func setupFullTestServer(t *testing.T) (*httptest.Server, *store.Store, *config.Config) {
	t.Helper()
	// TEST-FIX-3-COV: 走 store.MigratedStoreFromTemplate (1.24ms) 替代
	// store.Open(":memory:") + Migrate (26ms), ~20x 加速. 750+ tests 调用
	// 此 setup, 估省 ~18s / 36s wall clock (>50%). byte-identical schema.
	s := store.MigratedStoreFromTemplate(t)

	cfg := &config.Config{
		JWTSecret:     "test-secret",
		NodeEnv:       "development",
		AdminUser:     "admin",
		AdminPassword: "password123",
		UploadDir:     t.TempDir(),
		WorkspaceDir:  t.TempDir(),
		ClientDist:    t.TempDir(),
		CORSOrigin:    "*",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	authMw := auth.AuthMiddleware(s, cfg)

	authH := &AuthHandler{Store: s, Config: cfg, Logger: logger}
	authH.RegisterRoutes(mux)
	mux.Handle("GET /api/v1/users/me", authMw(http.HandlerFunc(authH.HandleGetMe)))

	chH := &ChannelHandler{Store: s, Config: cfg, Logger: logger}
	chH.RegisterRoutes(mux, authMw)

	msgH := &MessageHandler{Store: s, Logger: logger}
	sendPerm := func(next http.Handler) http.Handler { return next }
	readPerm := func(next http.Handler) http.Handler { return next }
	msgH.RegisterRoutes(mux, authMw, sendPerm, readPerm)

	adminH := &AdminHandler{Store: s, Logger: logger}
	adminH.RegisterRoutes(mux, authMw)

	agentH := &AgentHandler{Store: s, Logger: logger}
	agentH.RegisterRoutes(mux, authMw)

	dmH := &DmHandler{Store: s, Config: cfg, Logger: logger}
	dmH.RegisterRoutes(mux, authMw)

	rxH := &ReactionHandler{Store: s, Logger: logger}
	rxH.RegisterRoutes(mux, authMw)

	remH := &RemoteHandler{Store: s, Logger: logger}
	remH.RegisterRoutes(mux, authMw)

	wsH := &WorkspaceHandler{Store: s, Config: cfg, Logger: logger}
	wsH.RegisterRoutes(mux, authMw)

	userH := &UserHandler{Store: s, Logger: logger}
	userH.RegisterRoutes(mux, authMw)

	pollH := &PollHandler{Store: s, Logger: logger, Config: cfg}
	pollH.RegisterRoutes(mux, authMw)

	// Seed data
	// TEST-FIX-3-COV PERF: 用 bcrypt.MinCost (cost=4) 替代 auth.HashPassword
	// (cost=10). Compare 1ms vs 65ms × 2 user × N test = 大头 setup 浪费.
	// 跟 testutil/server.go::NewTestServer 同 idiom (MinCost for tests).
	ownerHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	ownerEmail := "owner@test.com"
	owner := &store.User{DisplayName: "Owner", Role: "admin", Email: &ownerEmail, PasswordHash: string(ownerHash)}
	s.CreateUser(owner)
	s.GrantDefaultPermissions(owner.ID, "admin")
	// ADM-0.2: legacy role=='admin' shortcut removed. Owner-as-admin needs
	// explicit (*, *) to keep blanket capability on the user-rail.
	s.GrantPermission(&store.UserPermission{UserID: owner.ID, Permission: "*", Scope: "*"})
	for _, permission := range []string{"channel.delete", "channel.manage_members", "channel.manage_visibility", "message.delete"} {
		s.GrantPermission(&store.UserPermission{UserID: owner.ID, Permission: permission, Scope: "*"})
	}

	memberHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	memberEmail := "member@test.com"
	member := &store.User{DisplayName: "Member", Role: "member", Email: &memberEmail, PasswordHash: string(memberHash)}
	s.CreateUser(member)
	s.GrantDefaultPermissions(member.ID, "member")

	general := &store.Channel{Name: "general", Visibility: "public", CreatedBy: owner.ID, Type: "channel", Position: store.GenerateInitialRank()}
	s.CreateChannel(general)
	s.AddChannelMember(&store.ChannelMember{ChannelID: general.ID, UserID: owner.ID})
	s.AddChannelMember(&store.ChannelMember{ChannelID: general.ID, UserID: member.ID})

	s.DB().Create(&store.InviteCode{Code: "test-invite", CreatedBy: "admin"})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, s, cfg
}

func loginAs(t *testing.T, url, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, _ := http.Post(url+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c.Value
		}
	}
	t.Fatal("no token")
	return ""
}

func jsonReq(t *testing.T, method, url, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		// COOKIE-NAME-CLEANUP D2: legacy `borgee_admin_token` cookie
		// removed — admin-rail uses `borgee_admin_session` (admin/auth.go::
		// CookieName), this jsonReq helper is user-rail only (callers all
		// hit /api/v1/* with borgee_token). Multi-cookie attach was dead
		// since admin-rail never reads borgee_admin_token. 真删 0 行为改.
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]any
	json.Unmarshal(b, &result)
	return resp, result
}

func createCh(t *testing.T, url, token, name, vis string) map[string]any {
	t.Helper()
	resp, data := jsonReq(t, "POST", url+"/api/v1/channels", token, map[string]string{"name": name, "visibility": vis})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel %s: %d %v", name, resp.StatusCode, data)
	}
	return data["channel"].(map[string]any)
}

func postMsg(t *testing.T, url, token, channelID, content string) map[string]any {
	t.Helper()
	resp, data := jsonReq(t, "POST", url+"/api/v1/channels/"+channelID+"/messages", token, map[string]string{"content": content})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post msg: %d %v", resp.StatusCode, data)
	}
	return data["message"].(map[string]any)
}

func getGeneralID(t *testing.T, url, token string) string {
	t.Helper()
	_, data := jsonReq(t, "GET", url+"/api/v1/channels", token, nil)
	for _, c := range data["channels"].([]any) {
		cm := c.(map[string]any)
		if cm["name"] == "general" {
			return cm["id"].(string)
		}
	}
	t.Fatal("general not found")
	return ""
}

func TestWriteRemoteResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		wantCode int
	}{
		{"path_not_allowed", `{"error":"path_not_allowed"}`, http.StatusForbidden},
		{"file_not_found", `{"error":"file_not_found"}`, http.StatusNotFound},
		{"file_too_large", `{"error":"file_too_large"}`, http.StatusRequestEntityTooLarge},
		{"timeout", `{"error":"timeout"}`, http.StatusGatewayTimeout},
		{"generic_error", `{"error":"something_else"}`, http.StatusBadGateway},
		{"success", `{"entries":[]}`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeRemoteResponse(rec, json.RawMessage(tt.input))
			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d", tt.wantCode, rec.Code)
			}
		})
	}
}

func TestPollHelpers(t *testing.T) {
	t.Parallel()
	// intersect
	result := intersect([]string{"a", "b", "c"}, map[string]bool{"b": true, "c": true})
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}

	// contains
	if !contains([]string{"a", "b"}, "a") {
		t.Fatal("expected true")
	}
	if contains([]string{"a", "b"}, "c") {
		t.Fatal("expected false")
	}

	// sseWrite
	rec := httptest.NewRecorder()
	sseWrite(rec, "test", 1, `{"hello":"world"}`)
	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected body")
	}
}

func TestChannelScopeFunc(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/api/v1/channels/test-id/topic", nil)
	req.SetPathValue("channelId", "test-id")
	scope := channelScope(req)
	if scope != "channel:test-id" {
		t.Fatalf("expected channel:test-id, got %s", scope)
	}
}

func TestReadJSONFunc(t *testing.T) {
	t.Parallel()
	// Test with normal JSON
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`{"name":"test"}`)))
	var body struct{ Name string }
	if err := readJSON(req, &body); err != nil {
		t.Fatal(err)
	}
	if body.Name != "test" {
		t.Fatalf("expected test, got %s", body.Name)
	}

	// Test with invalid JSON
	req2 := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("not json")))
	var body2 struct{ Name string }
	if err := readJSON(req2, &body2); err == nil {
		t.Fatal("expected error")
	}
}

func TestSlugifyFunc(t *testing.T) {
	t.Parallel()
	tests := []struct{ input, want string }{
		{"Hello World", "hello-world"},
		{"  test  ", "test"},
		{"ABC 123", "abc-123"},
		{"special!@#chars", "special-chars"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Fatalf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInternalChannelHandlers(t *testing.T) {
	t.Parallel()
	_ = os.Setenv("TMPDIR", os.TempDir())
	ts, s, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	generalID := getGeneralID(t, ts.URL, adminToken)

	t.Run("SetTopic", func(t *testing.T) {
		ch := createCh(t, ts.URL, memberToken, "topic-int", "public")
		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+ch["id"].(string)+"/topic", memberToken, map[string]string{"topic": "new"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("TopicTooLong", func(t *testing.T) {
		ch := createCh(t, ts.URL, memberToken, "topic-long-int", "public")
		long := make([]byte, 251)
		for i := range long {
			long[i] = 'a'
		}
		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+ch["id"].(string)+"/topic", memberToken, map[string]string{"topic": string(long)})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("PreviewPublic", func(t *testing.T) {
		ch := createCh(t, ts.URL, memberToken, "preview-int", "public")
		postMsg(t, ts.URL, memberToken, ch["id"].(string), "preview msg")
		resp, data := jsonReq(t, "GET", ts.URL+"/api/v1/channels/"+ch["id"].(string)+"/preview", memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if data["messages"] == nil {
			t.Fatal("expected messages")
		}
	})

	t.Run("PreviewPrivate", func(t *testing.T) {
		ch := createCh(t, ts.URL, adminToken, "preview-priv-int", "private")
		resp, _ := jsonReq(t, "GET", ts.URL+"/api/v1/channels/"+ch["id"].(string)+"/preview", memberToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("UpdateChannelEdgeCases", func(t *testing.T) {
		ch := createCh(t, ts.URL, adminToken, "upd-int", "public")
		chID := ch["id"].(string)
		// CHN-1.2 设计 ②: creator-only default member. Member must explicitly
		// join before topic-only PUT (which gates on IsChannelMember).
		if r, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/join", memberToken, nil); r.StatusCode != http.StatusOK {
			t.Fatalf("setup: member join: %d", r.StatusCode)
		}

		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+chID, adminToken, map[string]any{"visibility": "invalid"})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+chID, memberToken, map[string]any{"visibility": "private"})
		// AP-0: humans default to (*, *), so a member token now passes the
		// visibility-manage gate. Phase 4 AP-1/AP-3 will narrow this back.
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 (AP-0 wildcard), got %d", resp2.StatusCode)
		}

		resp3, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+chID, memberToken, map[string]any{"topic": "ok"})
		if resp3.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp3.StatusCode)
		}
	})

	t.Run("CreateChannelEdgeCases", func(t *testing.T) {
		resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels", memberToken, map[string]string{"name": "   "})
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels", memberToken, map[string]string{"name": "bad-vis", "visibility": "x"})
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}
	})

	t.Run("JoinLeaveEdgeCases", func(t *testing.T) {
		resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/nonexistent/join", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/leave", adminToken, nil)
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}
	})

	t.Run("AddRemoveMember", func(t *testing.T) {
		ch := createCh(t, ts.URL, adminToken, "addrem-int", "public")
		chID := ch["id"].(string)
		users, _ := s.ListUsers()
		var memberID string
		for _, u := range users {
			if u.Email != nil && *u.Email == "member@test.com" {
				memberID = u.ID
				break
			}
		}

		resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/members", adminToken, map[string]string{"user_id": memberID})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/members", adminToken, map[string]string{})
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}

		resp3, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/members", adminToken, map[string]string{"user_id": "nonexistent"})
		if resp3.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp3.StatusCode)
		}

		resp4, _ := jsonReq(t, "GET", ts.URL+"/api/v1/channels/"+chID+"/members", adminToken, nil)
		if resp4.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp4.StatusCode)
		}

		resp5, _ := jsonReq(t, "DELETE", ts.URL+"/api/v1/channels/"+chID+"/members/"+memberID, adminToken, nil)
		if resp5.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp5.StatusCode)
		}

		resp6, _ := jsonReq(t, "DELETE", ts.URL+"/api/v1/channels/"+generalID+"/members/"+memberID, adminToken, nil)
		if resp6.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp6.StatusCode)
		}
	})

	t.Run("MarkRead", func(t *testing.T) {
		ch := createCh(t, ts.URL, memberToken, "markread-int", "public")
		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+ch["id"].(string)+"/read", memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("ReorderChannel", func(t *testing.T) {
		ch1 := createCh(t, ts.URL, adminToken, "reord-int-1", "public")
		ch2 := createCh(t, ts.URL, adminToken, "reord-int-2", "public")
		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/reorder", adminToken, map[string]any{
			"channel_id": ch1["id"], "after_id": ch2["id"],
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/reorder", adminToken, map[string]any{})
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}
	})

	t.Run("ChannelGroups", func(t *testing.T) {
		resp, data := jsonReq(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{"name": "GrpInt"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		gid := data["group"].(map[string]any)["id"].(string)

		jsonReq(t, "PUT", ts.URL+"/api/v1/channel-groups/"+gid, adminToken, map[string]string{"name": "Updated"})
		jsonReq(t, "PUT", ts.URL+"/api/v1/channel-groups/reorder", adminToken, map[string]any{"group_id": gid})
		jsonReq(t, "DELETE", ts.URL+"/api/v1/channel-groups/"+gid, adminToken, nil)

		resp2, _ := jsonReq(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{"name": ""})
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}
	})

	t.Run("DeleteChannel", func(t *testing.T) {
		ch := createCh(t, ts.URL, adminToken, "del-int", "public")
		resp, _ := jsonReq(t, "DELETE", ts.URL+"/api/v1/channels/"+ch["id"].(string), adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "DELETE", ts.URL+"/api/v1/channels/"+ch["id"].(string), adminToken, nil)
		if resp2.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp2.StatusCode)
		}
	})

	t.Run("MessageSearch", func(t *testing.T) {
		postMsg(t, ts.URL, adminToken, generalID, "uniquesearch123")
		resp, _ := jsonReq(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages/search?q=uniquesearch123", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/messages/search?q=", adminToken, nil)
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}
	})

	t.Run("MessageEdgeCases", func(t *testing.T) {
		msg := postMsg(t, ts.URL, adminToken, generalID, "to-edit")
		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msg["id"].(string), adminToken, map[string]string{"content": "edited"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msg["id"].(string), adminToken, map[string]string{"content": ""})
		if resp2.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp2.StatusCode)
		}

		resp3, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/nonexistent", adminToken, map[string]string{"content": "x"})
		if resp3.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp3.StatusCode)
		}

		jsonReq(t, "DELETE", ts.URL+"/api/v1/messages/"+msg["id"].(string), adminToken, nil)
		resp4, _ := jsonReq(t, "DELETE", ts.URL+"/api/v1/messages/"+msg["id"].(string), adminToken, nil)
		if resp4.StatusCode != http.StatusNoContent && resp4.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 204 or 404, got %d", resp4.StatusCode)
		}

		resp5, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msg["id"].(string), adminToken, map[string]string{"content": "revive"})
		if resp5.StatusCode != http.StatusBadRequest && resp5.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 400 or 404, got %d", resp5.StatusCode)
		}
	})

	t.Run("DM", func(t *testing.T) {
		users, _ := s.ListUsers()
		var memberID string
		for _, u := range users {
			if u.Role == "member" {
				memberID = u.ID
				break
			}
		}
		resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "GET", ts.URL+"/api/v1/dm", adminToken, nil)
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp2.StatusCode)
		}
	})

	t.Run("Reactions", func(t *testing.T) {
		msg := postMsg(t, ts.URL, adminToken, generalID, "react-int")
		msgID := msg["id"].(string)
		resp, _ := jsonReq(t, "PUT", ts.URL+"/api/v1/messages/"+msgID+"/reactions", adminToken, map[string]string{"emoji": "👍"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "GET", ts.URL+"/api/v1/messages/"+msgID+"/reactions", adminToken, nil)
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp2.StatusCode)
		}

		resp3, _ := jsonReq(t, "DELETE", ts.URL+"/api/v1/messages/"+msgID+"/reactions", adminToken, map[string]string{"emoji": "👍"})
		if resp3.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp3.StatusCode)
		}
	})

	t.Run("Users", func(t *testing.T) {
		resp, _ := jsonReq(t, "GET", ts.URL+"/api/v1/users", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 (removed), got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "GET", ts.URL+"/api/v1/me/permissions", adminToken, nil)
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp2.StatusCode)
		}

		resp3, _ := jsonReq(t, "GET", ts.URL+"/api/v1/me/permissions", memberToken, nil)
		if resp3.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp3.StatusCode)
		}

		resp4, _ := jsonReq(t, "GET", ts.URL+"/api/v1/online", adminToken, nil)
		if resp4.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp4.StatusCode)
		}
	})

	t.Run("Poll", func(t *testing.T) {
		resp, _ := jsonReq(t, "POST", ts.URL+"/api/v1/poll", adminToken, map[string]any{"timeout_ms": 0})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		resp2, _ := jsonReq(t, "POST", ts.URL+"/api/v1/poll", adminToken, map[string]any{
			"timeout_ms": 0, "channel_ids": []string{generalID},
		})
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp2.StatusCode)
		}
	})

	t.Run("RemoteNodes", func(t *testing.T) {
		resp, data := jsonReq(t, "POST", ts.URL+"/api/v1/remote/nodes", adminToken, map[string]string{"machine_name": "test"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		nodeID := data["node"].(map[string]any)["id"].(string)

		jsonReq(t, "GET", ts.URL+"/api/v1/remote/nodes", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/status", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/ls?path=/", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/read?path=/", adminToken, nil)

		resp2, data2 := jsonReq(t, "POST", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", adminToken, map[string]string{
			"channel_id": generalID, "path": "/home", "label": "test",
		})
		if resp2.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp2.StatusCode)
		}
		bindingID := data2["binding"].(map[string]any)["id"].(string)

		jsonReq(t, "GET", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/api/v1/channels/"+generalID+"/remote-bindings", adminToken, nil)
		jsonReq(t, "DELETE", ts.URL+"/api/v1/remote/nodes/"+nodeID+"/bindings/"+bindingID, adminToken, nil)
		jsonReq(t, "DELETE", ts.URL+"/api/v1/remote/nodes/"+nodeID, adminToken, nil)
	})

	t.Run("Agents", func(t *testing.T) {
		resp, data := jsonReq(t, "POST", ts.URL+"/api/v1/agents", adminToken, map[string]any{"display_name": "Bot"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		aid := data["agent"].(map[string]any)["id"].(string)

		jsonReq(t, "GET", ts.URL+"/api/v1/agents", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/api/v1/agents/"+aid, adminToken, nil)
		jsonReq(t, "POST", ts.URL+"/api/v1/agents/"+aid+"/rotate-api-key", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/api/v1/agents/"+aid+"/permissions", adminToken, nil)
		jsonReq(t, "PUT", ts.URL+"/api/v1/agents/"+aid+"/permissions", adminToken, map[string]any{
			"permissions": []map[string]string{{"permission": "message.send"}},
		})
		jsonReq(t, "GET", ts.URL+"/api/v1/agents/"+aid+"/files", adminToken, nil)
		jsonReq(t, "DELETE", ts.URL+"/api/v1/agents/"+aid, adminToken, nil)
	})

	t.Run("Admin", func(t *testing.T) {
		jsonReq(t, "GET", ts.URL+"/admin-api/v1/users", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/admin-api/v1/channels", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/admin-api/v1/invites", adminToken, nil)

		users, _ := s.ListUsers()
		var memberID string
		for _, u := range users {
			if u.Role == "member" {
				memberID = u.ID
				break
			}
		}

		jsonReq(t, "PATCH", ts.URL+"/admin-api/v1/users/"+memberID, adminToken, map[string]any{"require_mention": true})
		jsonReq(t, "POST", ts.URL+"/admin-api/v1/users/"+memberID+"/api-key", adminToken, nil)
		jsonReq(t, "DELETE", ts.URL+"/admin-api/v1/users/"+memberID+"/api-key", adminToken, nil)
		jsonReq(t, "GET", ts.URL+"/admin-api/v1/users/"+memberID+"/permissions", adminToken, nil)
	})
}
