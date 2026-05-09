package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/auth"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushRecorder) Flush() { r.flushed = true }

type remoteProxyStub struct {
	online bool
	resp   json.RawMessage
	err    error
}

func (s remoteProxyStub) IsNodeOnline(string) bool { return s.online }

func (s remoteProxyStub) ProxyRequest(string, string, map[string]string) (json.RawMessage, error) {
	return s.resp, s.err
}

type agentFileProxyStub struct {
	status int
	body   []byte
	err    error
}

func (s agentFileProxyStub) ProxyPluginRequest(string, string, string, []byte) (int, []byte, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.status, s.body, s.err
}

type commandSourceStub struct{}

func (commandSourceStub) GetAllCommands() []AgentCommandGroup {
	return []AgentCommandGroup{{AgentID: "agent-1", AgentName: "Agent", Commands: []AgentCmdDef{{Name: "run", Description: "Run"}}}}
}

type eventHubStub struct {
	ch chan struct{}
}

func (h *eventHubStub) SubscribeEvents() chan struct{} {
	if h.ch == nil {
		h.ch = make(chan struct{}, 1)
	}
	return h.ch
}

func (h *eventHubStub) UnsubscribeEvents(chan struct{}) {}
func (h *eventHubStub) SignalNewEvents()                {}
func (h *eventHubStub) GetOnlineUserIDs() []string      { return []string{"online-user"} }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func adminIDFromStore(t *testing.T, s *store.Store) string {
	t.Helper()
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			return u.ID
		}
	}
	t.Fatal("owner not found")
	return ""
}

func memberIDFromStore(t *testing.T, s *store.Store) string {
	t.Helper()
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	for _, u := range users {
		if u.Role == "member" {
			return u.ID
		}
	}
	t.Fatal("member not found")
	return ""
}

func exerciseAuthedHandler(t *testing.T, s *store.Store, cfg *config.Config, token, pattern, method, target string, body io.Reader, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(pattern, auth.AuthMiddleware(s, cfg)(handler))
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func newClosedStoreTestServer(t *testing.T) (*httptest.Server, *store.Store, *config.Config) {
	t.Helper()
	return setupFullTestServer(t)
}

func rawReq(t *testing.T, method, url, token, contentType string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func multipartBody(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func typedMultipartBody(t *testing.T, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func TestHandlerUnauthorizedBranches(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	cfg := &config.Config{JWTSecret: "test-secret", WorkspaceDir: t.TempDir(), UploadDir: t.TempDir()}

	checks := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		target  string
		paths   map[string]string
		body    io.Reader
	}{
		{"agent-create", (&AgentHandler{Logger: logger}).handleCreateAgent, "POST", "/", nil, strings.NewReader(`{}`)},
		{"agent-list", (&AgentHandler{Logger: logger}).handleListAgents, "GET", "/", nil, nil},
		{"agent-get", (&AgentHandler{Logger: logger}).handleGetAgent, "GET", "/", map[string]string{"id": "agent"}, nil},
		{"agent-delete", (&AgentHandler{Logger: logger}).handleDeleteAgent, "DELETE", "/", map[string]string{"id": "agent"}, nil},
		{"agent-rotate", (&AgentHandler{Logger: logger}).handleRotateAPIKey, "POST", "/", map[string]string{"id": "agent"}, nil},
		{"agent-permissions", (&AgentHandler{Logger: logger}).handleGetPermissions, "GET", "/", map[string]string{"id": "agent"}, nil},
		{"agent-set-permissions", (&AgentHandler{Logger: logger}).handleSetPermissions, "PUT", "/", map[string]string{"id": "agent"}, strings.NewReader(`{}`)},
		{"agent-files", (&AgentHandler{Logger: logger}).handleGetAgentFiles, "GET", "/", map[string]string{"id": "agent"}, nil},
		{"channel-list", (&ChannelHandler{Logger: logger}).handleListChannels, "GET", "/", nil, nil},
		{"channel-create", (&ChannelHandler{Logger: logger}).handleCreateChannel, "POST", "/", nil, strings.NewReader(`{}`)},
		{"channel-get", (&ChannelHandler{Logger: logger}).handleGetChannel, "GET", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-preview", (&ChannelHandler{Logger: logger}).handlePreviewChannel, "GET", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-update", (&ChannelHandler{Logger: logger}).handleUpdateChannel, "PUT", "/", map[string]string{"channelId": "ch"}, strings.NewReader(`{}`)},
		{"channel-topic", (&ChannelHandler{Logger: logger}).handleSetTopic, "PUT", "/", map[string]string{"channelId": "ch"}, strings.NewReader(`{}`)},
		{"channel-join", (&ChannelHandler{Logger: logger}).handleJoinChannel, "POST", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-leave", (&ChannelHandler{Logger: logger}).handleLeaveChannel, "POST", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-add-member", (&ChannelHandler{Logger: logger}).handleAddMember, "POST", "/", map[string]string{"channelId": "ch"}, strings.NewReader(`{}`)},
		{"channel-remove-member", (&ChannelHandler{Logger: logger}).handleRemoveMember, "DELETE", "/", map[string]string{"channelId": "ch", "userId": "u"}, nil},
		{"channel-list-members", (&ChannelHandler{Logger: logger}).handleListMembers, "GET", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-mark-read", (&ChannelHandler{Logger: logger}).handleMarkRead, "PUT", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-delete", (&ChannelHandler{Logger: logger}).handleDeleteChannel, "DELETE", "/", map[string]string{"channelId": "ch"}, nil},
		{"channel-reorder", (&ChannelHandler{Logger: logger}).handleReorderChannel, "PUT", "/", nil, strings.NewReader(`{}`)},
		{"group-create", (&ChannelHandler{Logger: logger}).handleCreateGroup, "POST", "/", nil, strings.NewReader(`{}`)},
		{"group-update", (&ChannelHandler{Logger: logger}).handleUpdateGroup, "PUT", "/", map[string]string{"groupId": "g"}, strings.NewReader(`{}`)},
		{"group-delete", (&ChannelHandler{Logger: logger}).handleDeleteGroup, "DELETE", "/", map[string]string{"groupId": "g"}, nil},
		{"group-reorder", (&ChannelHandler{Logger: logger}).handleReorderGroup, "PUT", "/", nil, strings.NewReader(`{}`)},
		{"command-list", (&CommandHandler{Logger: logger}).handleListCommands, "GET", "/", nil, nil},
		{"dm-create", (&DmHandler{Config: cfg, Logger: logger}).handleCreateDm, "POST", "/", map[string]string{"userId": "u"}, nil},
		{"dm-list", (&DmHandler{Config: cfg, Logger: logger}).handleListDms, "GET", "/", nil, nil},
		{"message-create", (&MessageHandler{Logger: logger}).handleCreateMessage, "POST", "/", map[string]string{"channelId": "ch"}, strings.NewReader(`{}`)},
		{"message-update", (&MessageHandler{Logger: logger}).handleUpdateMessage, "PUT", "/", map[string]string{"messageId": "m"}, strings.NewReader(`{}`)},
		{"message-delete", (&MessageHandler{Logger: logger}).handleDeleteMessage, "DELETE", "/", map[string]string{"messageId": "m"}, nil},
		{"reaction-add", (&ReactionHandler{Logger: logger}).handleAddReaction, "PUT", "/", map[string]string{"messageId": "m"}, strings.NewReader(`{}`)},
		{"reaction-remove", (&ReactionHandler{Logger: logger}).handleRemoveReaction, "DELETE", "/", map[string]string{"messageId": "m"}, strings.NewReader(`{}`)},
		{"user-permissions", (&UserHandler{Logger: logger}).handleMyPermissions, "GET", "/", nil, nil},
		{"workspace-list", (&WorkspaceHandler{Config: cfg, Logger: logger}).handleListFiles, "GET", "/", map[string]string{"channelId": "ch"}, nil},
		{"workspace-upload", (&WorkspaceHandler{Config: cfg, Logger: logger}).handleUploadFile, "POST", "/", map[string]string{"channelId": "ch"}, strings.NewReader("")},
		{"workspace-mkdir", (&WorkspaceHandler{Config: cfg, Logger: logger}).handleMkdir, "POST", "/", map[string]string{"channelId": "ch"}, strings.NewReader(`{}`)},
		{"workspace-all", (&WorkspaceHandler{Config: cfg, Logger: logger}).handleListAllWorkspaces, "GET", "/", nil, nil},
		{"remote-list", (&RemoteHandler{Logger: logger}).handleListNodes, "GET", "/", nil, nil},
		{"remote-create", (&RemoteHandler{Logger: logger}).handleCreateNode, "POST", "/", nil, strings.NewReader(`{}`)},
		{"remote-delete", (&RemoteHandler{Logger: logger}).handleDeleteNode, "DELETE", "/", map[string]string{"id": "node"}, nil},
		{"remote-bindings", (&RemoteHandler{Logger: logger}).handleListBindings, "GET", "/", map[string]string{"nodeId": "node"}, nil},
		{"remote-create-binding", (&RemoteHandler{Logger: logger}).handleCreateBinding, "POST", "/", map[string]string{"nodeId": "node"}, strings.NewReader(`{}`)},
		{"remote-delete-binding", (&RemoteHandler{Logger: logger}).handleDeleteBinding, "DELETE", "/", map[string]string{"nodeId": "node", "id": "bind"}, nil},
		{"remote-channel-bindings", (&RemoteHandler{Logger: logger}).handleListChannelBindings, "GET", "/", map[string]string{"channelId": "ch"}, nil},
		{"remote-status", (&RemoteHandler{Logger: logger}).handleNodeStatus, "GET", "/", map[string]string{"nodeId": "node"}, nil},
		{"remote-ls", (&RemoteHandler{Logger: logger}).handleNodeLs, "GET", "/", map[string]string{"nodeId": "node"}, nil},
		{"remote-read", (&RemoteHandler{Logger: logger}).handleNodeRead, "GET", "/", map[string]string{"nodeId": "node"}, nil},
		{"poll-stream", (&PollHandler{Config: cfg, Logger: logger}).handleStreamGet, "GET", "/", nil, nil},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, tc.body)
			for k, v := range tc.paths {
				req.SetPathValue(k, v)
			}
			rec := httptest.NewRecorder()
			tc.handler(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestHTTPErrorBranches(t *testing.T) {
	t.Parallel()
	ts, s, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	generalID := getGeneralID(t, ts.URL, adminToken)
	msgID := postMsg(t, ts.URL, adminToken, generalID, "reaction validation")["id"].(string)

	tests := []struct {
		name   string
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{"removed-users", "GET", "/api/v1/users", "", nil, http.StatusNotFound},
		{"admin-users", "GET", "/admin-api/v1/users", memberToken, nil, http.StatusOK},
		{"not-found-channel", "GET", "/api/v1/channels/missing", adminToken, nil, http.StatusNotFound},
		{"bad-admin-create-json", "POST", "/admin-api/v1/users", adminToken, map[string]any{"display_name": ""}, http.StatusBadRequest},
		{"bad-agent-create", "POST", "/api/v1/agents", adminToken, map[string]any{"display_name": ""}, http.StatusBadRequest},
		{"bad-message-search", "GET", "/api/v1/channels/" + generalID + "/messages/search", adminToken, nil, http.StatusBadRequest},
		{"missing-message-delete", "DELETE", "/api/v1/messages/missing", adminToken, nil, http.StatusNotFound},
		{"bad-reaction", "PUT", "/api/v1/messages/" + msgID + "/reactions", adminToken, map[string]string{"emoji": ""}, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, data := jsonReq(t, tc.method, ts.URL+tc.path, tc.token, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d: %v", tc.want, resp.StatusCode, data)
			}
		})
	}

	node, err := s.CreateRemoteNode(adminIDFromStore(t, s), "branch-node")
	if err != nil {
		t.Fatalf("create remote node: %v", err)
	}
	resp, data := jsonReq(t, "POST", ts.URL+"/api/v1/remote/nodes/"+node.ID+"/bindings", adminToken, map[string]string{"channel_id": "", "path": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected remote binding 400, got %d: %v", resp.StatusCode, data)
	}
}

func TestUploadAndWorkspaceHardErrorBranches(t *testing.T) {
	t.Parallel()
	t.Run("upload save failure returns 500", func(t *testing.T) {
		ts, s, cfg := setupFullTestServer(t)
		token := loginAs(t, ts.URL, "owner@test.com", "password123")

		blocked := t.TempDir() + "/not-a-directory"
		if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
			t.Fatalf("write blocker: %v", err)
		}
		cfg.UploadDir = blocked

		mux := http.NewServeMux()
		(&UploadHandler{Config: cfg, Logger: testLogger()}).RegisterRoutes(mux, auth.AuthMiddleware(s, cfg))
		uploadServer := httptest.NewServer(mux)
		defer uploadServer.Close()

		body, contentType := typedMultipartBody(t, "image.png", "image/png", []byte("not really an image, content type is enough"))
		req, _ := http.NewRequest("POST", uploadServer.URL+"/api/v1/upload", body)
		req.Header.Set("Content-Type", contentType)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", resp.StatusCode)
		}
	})

	t.Run("workspace upload too large returns 413", func(t *testing.T) {
		ts, _, _ := setupFullTestServer(t)
		token := loginAs(t, ts.URL, "owner@test.com", "password123")
		generalID := getGeneralID(t, ts.URL, token)

		body, contentType := multipartBody(t, "big.txt", bytes.Repeat([]byte("x"), 11<<20))
		req, _ := http.NewRequest("POST", ts.URL+"/api/v1/channels/"+generalID+"/workspace/upload", body)
		req.Header.Set("Content-Type", contentType)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("workspace upload request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d", resp.StatusCode)
		}
	})

	t.Run("closed store returns 500 after auth", func(t *testing.T) {
		ts, s, cfg := newClosedStoreTestServer(t)
		token := loginAs(t, ts.URL, "owner@test.com", "password123")
		generalID := getGeneralID(t, ts.URL, token)
		h := &WorkspaceHandler{Store: s, Config: cfg, Logger: testLogger()}

		rec := exerciseAuthedHandler(t, s, cfg, token, "GET /api/v1/channels/{channelId}/workspace", "GET", "/api/v1/channels/"+generalID+"/workspace", nil, func(w http.ResponseWriter, r *http.Request) {
			_ = s.Close()
			h.handleListFiles(w, r)
		})
		if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusForbidden {
			t.Fatalf("expected 500 or 403, got %d", rec.Code)
		}
	})
}

func TestRemoteProxyErrorBranches(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	token := loginAs(t, ts.URL, "owner@test.com", "password123")
	adminID := adminIDFromStore(t, s)
	memberID := memberIDFromStore(t, s)
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	node, err := s.CreateRemoteNode(adminID, "proxy-node")
	if err != nil {
		t.Fatalf("create admin node: %v", err)
	}
	otherNode, err := s.CreateRemoteNode(memberID, "member-node")
	if err != nil {
		t.Fatalf("create member node: %v", err)
	}

	tests := []struct {
		name  string
		token string
		hub   RemoteProxy
		path  string
		want  int
	}{
		{"ls forbidden", memberToken, remoteProxyStub{online: true}, "/api/v1/remote/nodes/" + node.ID + "/ls?path=/", http.StatusForbidden},
		{"ls offline", token, remoteProxyStub{online: false}, "/api/v1/remote/nodes/" + node.ID + "/ls?path=/", http.StatusServiceUnavailable},
		{"ls timeout", token, remoteProxyStub{online: true, err: context.DeadlineExceeded}, "/api/v1/remote/nodes/" + node.ID + "/ls?path=/", http.StatusGatewayTimeout},
		{"read proxy error", token, remoteProxyStub{online: true, err: errors.New("boom")}, "/api/v1/remote/nodes/" + node.ID + "/read?path=/tmp/a", http.StatusBadGateway},
		{"read success", token, remoteProxyStub{online: true, resp: json.RawMessage(`{"content":"ok"}`)}, "/api/v1/remote/nodes/" + node.ID + "/read?path=/tmp/a", http.StatusOK},
	}
	_ = otherNode

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &RemoteHandler{Store: s, Logger: testLogger(), Hub: tc.hub}
			handler := h.handleNodeLs
			pattern := "GET /api/v1/remote/nodes/{nodeId}/ls"
			if strings.Contains(tc.path, "/read") {
				handler = h.handleNodeRead
				pattern = "GET /api/v1/remote/nodes/{nodeId}/read"
			}
			rec := exerciseAuthedHandler(t, s, cfg, tc.token, pattern, "GET", tc.path, nil, handler)
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d body=%s", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSSEBackfillAndFlush(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	token := loginAs(t, ts.URL, "owner@test.com", "password123")
	generalID := getGeneralID(t, ts.URL, token)
	postMsg(t, ts.URL, token, generalID, "sse backfill")

	adminID := adminIDFromStore(t, s)
	apiKey, err := store.GenerateAPIKey()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	if err := s.SetAPIKey(adminID, apiKey); err != nil {
		t.Fatalf("set api key: %v", err)
	}

	h := &PollHandler{Store: s, Logger: testLogger(), Config: cfg}
	req := httptest.NewRequest("GET", "/api/v1/stream", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Last-Event-ID", "0")
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	h.handleStreamGet(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !rec.flushed {
		t.Fatal("expected stream flush")
	}
	if !strings.Contains(rec.Body.String(), ":connected") || !strings.Contains(rec.Body.String(), "event:") {
		t.Fatalf("expected connected event and backfill, got %q", rec.Body.String())
	}
}

func TestAdminAndAgentAdditionalBranches(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	adminID := adminIDFromStore(t, s)
	memberID := memberIDFromStore(t, s)

	resp, data := jsonReq(t, "POST", ts.URL+"/api/v1/agents", memberToken, map[string]any{
		"display_name": "Member Agent",
		"permissions":  []map[string]string{{"permission": "message.send"}},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create member agent: %d %v", resp.StatusCode, data)
	}
	memberAgentID := data["agent"].(map[string]any)["id"].(string)

	resp, data = jsonReq(t, "POST", ts.URL+"/api/v1/agents", adminToken, map[string]any{"display_name": "Admin Agent"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create admin agent: %d %v", resp.StatusCode, data)
	}
	adminAgentID := data["agent"].(map[string]any)["id"].(string)

	checks := []struct {
		name   string
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{"list-admin-users", "GET", "/admin-api/v1/users", adminToken, nil, http.StatusOK},
		{"create-invalid-role", "POST", "/admin-api/v1/users", adminToken, map[string]string{"display_name": "Bad", "role": "owner"}, http.StatusBadRequest},
		{"create-agent-user", "POST", "/admin-api/v1/users", adminToken, map[string]string{"display_name": "Loose Agent", "role": "agent"}, http.StatusBadRequest},
		{"own-role-change", "PATCH", "/admin-api/v1/users/" + adminID, adminToken, map[string]string{"role": "admin"}, http.StatusBadRequest},
		{"invalid-role-change", "PATCH", "/admin-api/v1/users/" + memberID, adminToken, map[string]string{"role": "owner"}, http.StatusBadRequest},
		{"non-agent-to-agent", "PATCH", "/admin-api/v1/users/" + memberID, adminToken, map[string]string{"role": "agent"}, http.StatusBadRequest},
		{"owned-agent-to-admin", "PATCH", "/admin-api/v1/users/" + memberAgentID, adminToken, map[string]string{"role": "admin"}, http.StatusBadRequest},
		{"update-all-fields", "PATCH", "/admin-api/v1/users/" + memberID, adminToken, map[string]any{"display_name": "Member Renamed", "password": "password456", "require_mention": true}, http.StatusOK},
		{"delete-missing-user", "DELETE", "/admin-api/v1/users/missing", adminToken, nil, http.StatusNotFound},
		{"get-admin-permissions", "GET", "/admin-api/v1/users/" + adminID + "/permissions", adminToken, nil, http.StatusOK},
		{"grant-missing-permission", "POST", "/admin-api/v1/users/" + memberID + "/permissions", adminToken, map[string]string{}, http.StatusBadRequest},
		{"grant-permission", "POST", "/admin-api/v1/users/" + memberID + "/permissions", adminToken, map[string]string{"permission": "channel.manage_members"}, http.StatusCreated},
		{"grant-duplicate", "POST", "/admin-api/v1/users/" + memberID + "/permissions", adminToken, map[string]string{"permission": "channel.manage_members"}, http.StatusConflict},
		{"get-member-permissions", "GET", "/admin-api/v1/users/" + memberID + "/permissions", adminToken, nil, http.StatusOK},
		{"revoke-missing", "DELETE", "/admin-api/v1/users/" + memberID + "/permissions", adminToken, map[string]string{"permission": "channel.delete"}, http.StatusNotFound},
		{"revoke-permission", "DELETE", "/admin-api/v1/users/" + memberID + "/permissions", adminToken, map[string]string{"permission": "channel.manage_members"}, http.StatusOK},
		{"create-expiring-invite", "POST", "/admin-api/v1/invites", adminToken, map[string]any{"expires_in_hours": 1, "note": "short"}, http.StatusCreated},
		{"delete-missing-invite", "DELETE", "/admin-api/v1/invites/missing", adminToken, nil, http.StatusNotFound},
		{"list-admin-channels", "GET", "/admin-api/v1/channels", adminToken, nil, http.StatusOK},
		{"agent-get-forbidden", "GET", "/api/v1/agents/" + adminAgentID, memberToken, nil, http.StatusForbidden},
		{"agent-get-not-found", "GET", "/api/v1/agents/missing", adminToken, nil, http.StatusNotFound},
		{"agent-get", "GET", "/api/v1/agents/" + adminAgentID, adminToken, nil, http.StatusOK},
		{"agent-rotate-forbidden", "POST", "/api/v1/agents/" + adminAgentID + "/rotate-api-key", memberToken, nil, http.StatusForbidden},
		{"agent-rotate", "POST", "/api/v1/agents/" + adminAgentID + "/rotate-api-key", adminToken, nil, http.StatusOK},
		{"agent-perms-forbidden", "GET", "/api/v1/agents/" + adminAgentID + "/permissions", memberToken, nil, http.StatusForbidden},
		{"agent-perms", "GET", "/api/v1/agents/" + adminAgentID + "/permissions", adminToken, nil, http.StatusOK},
		{"agent-set-perms", "PUT", "/api/v1/agents/" + adminAgentID + "/permissions", adminToken, map[string]any{"permissions": []map[string]string{{"permission": "workspace.write"}}}, http.StatusOK},
		{"agent-files-disconnected", "GET", "/api/v1/agents/" + adminAgentID + "/files?path=/tmp/a", adminToken, nil, http.StatusServiceUnavailable},
		{"agent-delete-forbidden", "DELETE", "/api/v1/agents/" + adminAgentID, memberToken, nil, http.StatusForbidden},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := jsonReq(t, tc.method, ts.URL+tc.path, tc.token, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d: %v", tc.want, resp.StatusCode, body)
			}
		})
	}

	jsonReq(t, "PATCH", ts.URL+"/admin-api/v1/users/"+memberID, adminToken, map[string]any{"disabled": true})
	agent, err := s.GetAgent(memberAgentID)
	if err != nil || !agent.Disabled {
		t.Fatalf("expected owned agent disabled, agent=%+v err=%v", agent, err)
	}
	jsonReq(t, "PATCH", ts.URL+"/admin-api/v1/users/"+memberID, adminToken, map[string]any{"disabled": false})
	agent, err = s.GetAgent(memberAgentID)
	if err != nil || agent.Disabled {
		t.Fatalf("expected owned agent re-enabled, agent=%+v err=%v", agent, err)
	}

	h := &AgentHandler{Store: s, Logger: testLogger(), Hub: agentFileProxyStub{status: http.StatusAccepted, body: []byte(`{"ok":true}`)}}
	rec := exerciseAuthedHandler(t, s, cfg, adminToken, "GET /api/v1/agents/{id}/files", "GET", "/api/v1/agents/"+adminAgentID+"/files?path=/tmp/a", nil, h.handleGetAgentFiles)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected proxied status, got %d body=%s", rec.Code, rec.Body.String())
	}

	h.Hub = agentFileProxyStub{err: errors.New("offline")}
	rec = exerciseAuthedHandler(t, s, cfg, adminToken, "GET /api/v1/agents/{id}/files", "GET", "/api/v1/agents/"+adminAgentID+"/files?path=/tmp/a", nil, h.handleGetAgentFiles)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestChannelsMessagesWorkspaceAdditionalBranches(t *testing.T) {
	t.Parallel()
	ts, _, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	generalID := getGeneralID(t, ts.URL, adminToken)

	publicCh := createCh(t, ts.URL, adminToken, "branches-public", "public")
	publicID := publicCh["id"].(string)
	privateCh := createCh(t, ts.URL, adminToken, "branches-private", "private")
	privateID := privateCh["id"].(string)

	// CHN-1.2 设计 ②: creator-only default member. Member must explicitly join
	// the public channel before mark-read / update-topic / leave checks below.
	if resp, body := jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+publicID+"/join", memberToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("setup: member join public: %d %v", resp.StatusCode, body)
	}

	msg := postMsg(t, ts.URL, adminToken, publicID, "branch message")
	msgID := msg["id"].(string)

	checks := []struct {
		name   string
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{"preview-public", "GET", "/api/v1/channels/" + publicID + "/preview", memberToken, nil, http.StatusOK},
		{"preview-private", "GET", "/api/v1/channels/" + privateID + "/preview", memberToken, nil, http.StatusNotFound},
		{"list-members", "GET", "/api/v1/channels/" + publicID + "/members", memberToken, nil, http.StatusOK},
		{"list-members-private-not-found", "GET", "/api/v1/channels/" + privateID + "/members", memberToken, nil, http.StatusNotFound},
		{"mark-read", "PUT", "/api/v1/channels/" + publicID + "/read", memberToken, nil, http.StatusOK},
		{"mark-read-nonmember", "PUT", "/api/v1/channels/" + privateID + "/read", memberToken, nil, http.StatusForbidden},
		{"topic-nonmember", "PUT", "/api/v1/channels/" + privateID + "/topic", memberToken, map[string]string{"topic": "x"}, http.StatusForbidden},
		{"update-topic-only", "PUT", "/api/v1/channels/" + publicID, memberToken, map[string]string{"topic": "member topic"}, http.StatusOK},
		{"update-name-conflict", "PUT", "/api/v1/channels/" + publicID, adminToken, map[string]string{"name": "general"}, http.StatusConflict},
		{"update-bad-visibility", "PUT", "/api/v1/channels/" + publicID, adminToken, map[string]string{"visibility": "hidden"}, http.StatusBadRequest},
		{"join-private", "POST", "/api/v1/channels/" + privateID + "/join", memberToken, nil, http.StatusForbidden},
		{"leave-public", "POST", "/api/v1/channels/" + publicID + "/leave", memberToken, nil, http.StatusOK},
		// ADM-0.3 + AP-0: members default to (*, *) wildcard, so the user-rail
		// reorder permission check now passes. Test asserts the happy path.
		{"reorder-member-allowed", "PUT", "/api/v1/channels/reorder", memberToken, map[string]string{"channel_id": publicID}, http.StatusOK},
		{"group-update-missing", "PUT", "/api/v1/channel-groups/missing", adminToken, map[string]string{"name": "x"}, http.StatusNotFound},
		{"group-delete-missing", "DELETE", "/api/v1/channel-groups/missing", adminToken, nil, http.StatusNotFound},
		{"group-reorder-missing", "PUT", "/api/v1/channel-groups/reorder", adminToken, map[string]string{"group_id": "missing"}, http.StatusNotFound},
		{"list-messages-window", "GET", "/api/v1/channels/" + publicID + "/messages?before=9999999999999&after=1&limit=500", adminToken, nil, http.StatusOK},
		{"search-limit", "GET", "/api/v1/channels/" + publicID + "/messages/search?q=branch&limit=999", adminToken, nil, http.StatusOK},
		{"create-message-missing-content", "POST", "/api/v1/channels/" + publicID + "/messages", adminToken, map[string]string{"content": "   "}, http.StatusBadRequest},
		{"create-message-forbidden", "POST", "/api/v1/channels/" + privateID + "/messages", memberToken, map[string]string{"content": "x"}, http.StatusNotFound},
		// AP-5: post-leave-public (line 600) member is no longer in publicID,
		// so channel-member ACL gate fires before sender_id check → 404
		// "Channel not found" fail-closed (跟 AP-4 reactions 同模式).
		{"update-message-forbidden", "PUT", "/api/v1/messages/" + msgID, memberToken, map[string]string{"content": "member edit"}, http.StatusNotFound},
		{"delete-message-forbidden", "DELETE", "/api/v1/messages/" + msgID, memberToken, nil, http.StatusNotFound},
		// ADM-0.3: admin fixture is the sender (line 565), so sender-only
		// delete on the user-rail succeeds with 204.
		{"delete-message-admin", "DELETE", "/api/v1/messages/" + msgID, adminToken, nil, http.StatusNoContent},
		{"dm-list", "GET", "/api/v1/dm", adminToken, nil, http.StatusOK},
		{"dm-missing-user", "POST", "/api/v1/dm/missing", adminToken, nil, http.StatusNotFound},
		{"users-permissions", "GET", "/api/v1/me/permissions", adminToken, nil, http.StatusOK},
		{"users-online", "GET", "/api/v1/online", adminToken, nil, http.StatusOK},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := jsonReq(t, tc.method, ts.URL+tc.path, tc.token, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d: %v", tc.want, resp.StatusCode, body)
			}
		})
	}

	resp, groupData := jsonReq(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{"name": "Branches Group"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group: %d %v", resp.StatusCode, groupData)
	}
	groupID := groupData["group"].(map[string]any)["id"].(string)
	for _, step := range []struct {
		method string
		path   string
		body   any
	}{
		{"PUT", "/api/v1/channel-groups/" + groupID, map[string]string{"name": "Nope"}},
		{"PUT", "/api/v1/channel-groups/reorder", map[string]string{"group_id": groupID}},
		{"DELETE", "/api/v1/channel-groups/" + groupID, nil},
	} {
		resp, body := jsonReq(t, step.method, ts.URL+step.path, memberToken, step.body)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s expected 403 got %d: %v", step.method, step.path, resp.StatusCode, body)
		}
	}
	for _, step := range []struct {
		method string
		path   string
		body   any
		want   int
	}{
		{"PUT", "/api/v1/channel-groups/" + groupID, map[string]string{"name": "Branches Group 2"}, http.StatusOK},
		{"PUT", "/api/v1/channel-groups/reorder", map[string]string{"group_id": groupID}, http.StatusOK},
		{"DELETE", "/api/v1/channel-groups/" + groupID, nil, http.StatusOK},
	} {
		resp, body := jsonReq(t, step.method, ts.URL+step.path, adminToken, step.body)
		if resp.StatusCode != step.want {
			t.Fatalf("%s %s expected %d got %d: %v", step.method, step.path, step.want, resp.StatusCode, body)
		}
	}

	fileBody, contentType := multipartBody(t, "note.txt", []byte("hello"))
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/channels/"+generalID+"/workspace/upload", fileBody)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: adminToken})
	respHTTP, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("workspace upload: %v", err)
	}
	var uploadData map[string]any
	json.NewDecoder(respHTTP.Body).Decode(&uploadData)
	respHTTP.Body.Close()
	if respHTTP.StatusCode != http.StatusCreated {
		t.Fatalf("workspace upload status %d: %v", respHTTP.StatusCode, uploadData)
	}
	fileID := uploadData["file"].(map[string]any)["id"].(string)

	workspaceChecks := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"download", "GET", "/api/v1/channels/" + generalID + "/workspace/files/" + fileID, nil, http.StatusOK},
		{"rename-empty", "PATCH", "/api/v1/channels/" + generalID + "/workspace/files/" + fileID, map[string]string{"name": ""}, http.StatusBadRequest},
		{"rename", "PATCH", "/api/v1/channels/" + generalID + "/workspace/files/" + fileID, map[string]string{"name": "renamed.txt"}, http.StatusOK},
		{"mkdir-empty", "POST", "/api/v1/channels/" + generalID + "/workspace/mkdir", map[string]string{"name": ""}, http.StatusBadRequest},
		{"mkdir", "POST", "/api/v1/channels/" + generalID + "/workspace/mkdir", map[string]string{"name": "folder"}, http.StatusCreated},
		{"move-root", "POST", "/api/v1/channels/" + generalID + "/workspace/files/" + fileID + "/move", map[string]any{"parentId": nil}, http.StatusOK},
		{"delete", "DELETE", "/api/v1/channels/" + generalID + "/workspace/files/" + fileID, nil, http.StatusOK},
		{"download-deleted", "GET", "/api/v1/channels/" + generalID + "/workspace/files/" + fileID, nil, http.StatusNotFound},
		{"list-all", "GET", "/api/v1/workspaces", nil, http.StatusOK},
	}
	for _, tc := range workspaceChecks {
		t.Run("workspace-"+tc.name, func(t *testing.T) {
			resp, body := jsonReq(t, tc.method, ts.URL+tc.path, adminToken, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d: %v", tc.want, resp.StatusCode, body)
			}
		})
	}
}

func TestInvalidJSONBranches(t *testing.T) {
	t.Parallel()
	ts, s, _ := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	generalID := getGeneralID(t, ts.URL, adminToken)
	ch := createCh(t, ts.URL, adminToken, "invalid-json-public", "public")
	chID := ch["id"].(string)
	msg := postMsg(t, ts.URL, adminToken, chID, "invalid json target")
	msgID := msg["id"].(string)
	node, err := s.CreateRemoteNode(adminIDFromStore(t, s), "json-node")
	if err != nil {
		t.Fatalf("create remote node: %v", err)
	}
	resp, groupData := jsonReq(t, "POST", ts.URL+"/api/v1/channel-groups", adminToken, map[string]string{"name": "Invalid JSON Group"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group: %d %v", resp.StatusCode, groupData)
	}
	groupID := groupData["group"].(map[string]any)["id"].(string)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"admin-create", "POST", "/admin-api/v1/users"},
		{"admin-update", "PATCH", "/admin-api/v1/users/" + adminIDFromStore(t, s)},
		{"admin-grant", "POST", "/admin-api/v1/users/" + adminIDFromStore(t, s) + "/permissions"},
		{"admin-revoke", "DELETE", "/admin-api/v1/users/" + adminIDFromStore(t, s) + "/permissions"},
		{"admin-invite", "POST", "/admin-api/v1/invites"},
		{"agent-create", "POST", "/api/v1/agents"},
		{"agent-set-perms", "PUT", "/api/v1/agents/missing/permissions"},
		{"channel-create", "POST", "/api/v1/channels"},
		{"channel-update", "PUT", "/api/v1/channels/" + chID},
		{"channel-topic", "PUT", "/api/v1/channels/" + chID + "/topic"},
		{"channel-add-member", "POST", "/api/v1/channels/" + chID + "/members"},
		{"channel-reorder", "PUT", "/api/v1/channels/reorder"},
		{"group-create", "POST", "/api/v1/channel-groups"},
		{"group-update", "PUT", "/api/v1/channel-groups/" + groupID},
		{"group-reorder", "PUT", "/api/v1/channel-groups/reorder"},
		{"message-create", "POST", "/api/v1/channels/" + chID + "/messages"},
		{"message-update", "PUT", "/api/v1/messages/" + msgID},
		{"reaction-add", "PUT", "/api/v1/messages/" + msgID + "/reactions"},
		{"reaction-remove", "DELETE", "/api/v1/messages/" + msgID + "/reactions"},
		{"remote-create-node", "POST", "/api/v1/remote/nodes"},
		{"remote-create-binding", "POST", "/api/v1/remote/nodes/" + node.ID + "/bindings"},
		{"workspace-update", "PUT", "/api/v1/channels/" + generalID + "/workspace/files/missing"},
		{"workspace-rename", "PATCH", "/api/v1/channels/" + generalID + "/workspace/files/missing"},
		{"workspace-mkdir", "POST", "/api/v1/channels/" + generalID + "/workspace/mkdir"},
		{"workspace-move", "POST", "/api/v1/channels/" + generalID + "/workspace/files/missing/move"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := rawReq(t, tc.method, ts.URL+tc.path, adminToken, "application/json", strings.NewReader(`{"bad"`))
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 400, precondition 404, or 403, got %d", resp.StatusCode)
			}
		})
	}
}

func TestAuthPollAndMessageAdditionalBranches(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	adminID := adminIDFromStore(t, s)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"login-invalid-json", "POST", "/api/v1/auth/login", `{"bad"`, http.StatusBadRequest},
		{"login-no-user", "POST", "/api/v1/auth/login", `{"email":"nobody@test.com","password":"password123"}`, http.StatusUnauthorized},
		{"register-invalid-json", "POST", "/api/v1/auth/register", `{"bad"`, http.StatusBadRequest},
		{"register-duplicate", "POST", "/api/v1/auth/register", `{"invite_code":"test-invite","email":"owner@test.com","password":"password123","display_name":"Dup"}`, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := rawReq(t, tc.method, ts.URL+tc.path, "", "application/json", strings.NewReader(tc.body))
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, resp.StatusCode)
			}
		})
	}

	usedBy := adminID
	expiredAt := time.Now().Add(-time.Hour).UnixMilli()
	s.DB().Create(&store.InviteCode{Code: "used-invite", CreatedBy: adminID, UsedBy: &usedBy})
	s.DB().Create(&store.InviteCode{Code: "expired-invite", CreatedBy: adminID, ExpiresAt: &expiredAt})
	for _, code := range []string{"used-invite", "expired-invite"} {
		resp := rawReq(t, "POST", ts.URL+"/api/v1/auth/register", "", "application/json", strings.NewReader(`{"invite_code":"`+code+`","email":"`+code+`@test.com","password":"password123","display_name":"Bad"}`))
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s expected 404, got %d", code, resp.StatusCode)
		}
	}

	resp, data := jsonReq(t, "GET", ts.URL+"/api/v1/users/me", memberToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("member me expected 200, got %d: %v", resp.StatusCode, data)
	}

	prodCfg := *cfg
	prodCfg.NodeEnv = "production"
	h := &AuthHandler{Store: s, Config: &prodCfg, Logger: testLogger()}
	req := httptest.NewRequest("POST", "https://example.com/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	admin, err := s.GetUserByID(adminID)
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	h.signAndSetCookie(rec, req, admin)
	if cookies := rec.Result().Cookies(); len(cookies) == 0 || !cookies[0].Secure {
		t.Fatalf("expected secure production cookie, got %#v", cookies)
	}

	msgHandler := &MessageHandler{Store: s, Logger: testLogger()}
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{"list-missing-channel-id", msgHandler.handleListMessages, "GET"},
		{"search-missing-channel-id", msgHandler.handleSearchMessages, "GET"},
		{"create-missing-channel-id", msgHandler.handleCreateMessage, "POST"},
		{"update-missing-message-id", msgHandler.handleUpdateMessage, "PUT"},
		{"delete-missing-message-id", msgHandler.handleDeleteMessage, "DELETE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/", strings.NewReader(`{}`))
			rec := httptest.NewRecorder()
			tc.handler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
		})
	}

	apiKey, err := store.GenerateAPIKey()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	if err := s.SetAPIKey(adminID, apiKey); err != nil {
		t.Fatalf("set api key: %v", err)
	}
	poll := &PollHandler{Store: s, Logger: testLogger(), Config: cfg, Hub: &eventHubStub{}}
	cursor := int64(999999)
	timeout := 100
	body, _ := json.Marshal(map[string]any{"api_key": apiKey, "cursor": cursor, "timeout_ms": timeout})
	req = httptest.NewRequest("POST", "/api/v1/poll", bytes.NewReader(body))
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	poll.handlePoll(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("poll cancel expected 200, got %d", rec.Code)
	}

	for _, timeoutMs := range []int{70000, -1} {
		body, _ = json.Marshal(map[string]any{"api_key": apiKey, "cursor": cursor, "timeout_ms": timeoutMs})
		req = httptest.NewRequest("POST", "/api/v1/poll", bytes.NewReader(body))
		rec = httptest.NewRecorder()
		(&PollHandler{Store: s, Logger: testLogger(), Config: cfg}).handlePoll(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("poll timeout %d expected 200, got %d", timeoutMs, rec.Code)
		}
	}

	for _, target := range []string{"/api/v1/stream?api_key=" + apiKey, "/api/v1/stream"} {
		req = httptest.NewRequest("GET", target, nil)
		if target == "/api/v1/stream" {
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: adminToken})
		}
		ctx, cancel = context.WithCancel(req.Context())
		cancel()
		req = req.WithContext(ctx)
		fr := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
		poll.handleStreamGet(fr, req)
		if fr.Code != http.StatusOK {
			t.Fatalf("stream %s expected 200, got %d", target, fr.Code)
		}
	}
}

func TestWorkspaceAndRemoteAdditionalBranches(t *testing.T) {
	t.Parallel()
	ts, s, cfg := setupFullTestServer(t)
	adminToken := loginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := loginAs(t, ts.URL, "member@test.com", "password123")
	adminID := adminIDFromStore(t, s)
	generalID := getGeneralID(t, ts.URL, adminToken)

	blocked := t.TempDir() + "/not-dir"
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	cfg.WorkspaceDir = blocked
	body, contentType := multipartBody(t, "fail.txt", []byte("hello"))
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/channels/"+generalID+"/workspace/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: adminToken})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("workspace upload failure request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("workspace blocked dir expected 500, got %d", resp.StatusCode)
	}

	cfg.WorkspaceDir = t.TempDir()
	body, contentType = multipartBody(t, "ok.txt", []byte("hello"))
	req, _ = http.NewRequest("POST", ts.URL+"/api/v1/channels/"+generalID+"/workspace/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: adminToken})
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("workspace upload request: %v", err)
	}
	var uploadData map[string]any
	json.NewDecoder(resp.Body).Decode(&uploadData)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("workspace upload expected 201, got %d: %v", resp.StatusCode, uploadData)
	}
	fileID := uploadData["file"].(map[string]any)["id"].(string)
	os.RemoveAll(filepath.Join(cfg.WorkspaceDir, adminID, generalID))
	resp, data := jsonReq(t, "PUT", ts.URL+"/api/v1/channels/"+generalID+"/workspace/files/"+fileID, adminToken, map[string]string{"content": "new"})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("workspace update missing dir expected 500, got %d: %v", resp.StatusCode, data)
	}
	resp, data = jsonReq(t, "POST", ts.URL+"/api/v1/channels/"+generalID+"/workspace/mkdir", adminToken, map[string]string{"name": "delete-dir"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("workspace mkdir expected 201, got %d: %v", resp.StatusCode, data)
	}
	dirID := data["file"].(map[string]any)["id"].(string)
	resp, data = jsonReq(t, "DELETE", ts.URL+"/api/v1/channels/"+generalID+"/workspace/files/"+dirID, adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("workspace delete dir expected 200, got %d: %v", resp.StatusCode, data)
	}

	node, err := s.CreateRemoteNode(adminID, "remote-more")
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	for _, tc := range []struct {
		name   string
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{"delete-node-forbidden", "DELETE", "/api/v1/remote/nodes/" + node.ID, memberToken, nil, http.StatusForbidden},
		{"node-status-offline", "GET", "/api/v1/remote/nodes/" + node.ID + "/status", adminToken, nil, http.StatusOK},
		{"create-binding", "POST", "/api/v1/remote/nodes/" + node.ID + "/bindings", adminToken, map[string]string{"channel_id": generalID, "path": "/tmp", "label": "Tmp"}, http.StatusCreated},
		{"list-bindings", "GET", "/api/v1/remote/nodes/" + node.ID + "/bindings", adminToken, nil, http.StatusOK},
		{"channel-bindings", "GET", "/api/v1/channels/" + generalID + "/remote-bindings", adminToken, nil, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := jsonReq(t, tc.method, ts.URL+tc.path, tc.token, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d: %v", tc.want, resp.StatusCode, body)
			}
		})
	}

	resp, data = jsonReq(t, "POST", ts.URL+"/api/v1/remote/nodes/"+node.ID+"/bindings", adminToken, map[string]string{"channel_id": generalID, "path": "/var", "label": "Var"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create binding for delete expected 201, got %d: %v", resp.StatusCode, data)
	}
	bindingID := data["binding"].(map[string]any)["id"].(string)
	resp, data = jsonReq(t, "DELETE", ts.URL+"/api/v1/remote/nodes/"+node.ID+"/bindings/"+bindingID, adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete binding expected 200, got %d: %v", resp.StatusCode, data)
	}
}

func TestSmallFallbackBranches(t *testing.T) {
	t.Parallel()
	if got := mustJSON(map[string]any{"bad": func() {}}); got != "{}" {
		t.Fatalf("expected fallback JSON, got %q", got)
	}

	t.Run("reaction closed store returns 404 fail-closed (AP-4 ACL gate)", func(t *testing.T) {
		ts, s, cfg := newClosedStoreTestServer(t)
		token := loginAs(t, ts.URL, "owner@test.com", "password123")
		generalID := getGeneralID(t, ts.URL, token)
		msg := postMsg(t, ts.URL, token, generalID, "reaction 500")
		h := &ReactionHandler{Store: s, Logger: testLogger()}
		rec := exerciseAuthedHandler(t, s, cfg, token, "GET /api/v1/messages/{messageId}/reactions", "GET", "/api/v1/messages/"+msg["id"].(string)+"/reactions", nil, func(w http.ResponseWriter, r *http.Request) {
			_ = s.Close()
			h.handleGetReactions(w, r)
		})
		// AP-4: handleGetReactions now runs canAccessMessage FIRST which
		// hits the closed store at GetMessageByID and returns 404 (channel
		// hidden, fail-closed). The error-path 500 from GetReactionsByMessage
		// is now unreachable from this test seam — that's the correct
		// security posture (fail-closed beats fail-loud here).
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 fail-closed (was 500 pre-AP-4), got %d", rec.Code)
		}
	})
}
