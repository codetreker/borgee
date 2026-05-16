package testutil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/auth"
	"borgee-server/internal/config"
	"borgee-server/internal/server"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil/clock"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

var testPasswordHash string

// Performance note: ADM-0.1/0.2 require BORGEE_ADMIN_* env vars when
// server.New() runs admin.Bootstrap. Before 2026-04-29, NewTestServer used
// t.Setenv per call, which prevents tests from using t.Parallel. Env setup now
// lives in package init so callers can use `t.Parallel()` freely. The values
// are stable test-only literals (bcrypt cost=10 hash of "password123") that
// never reach production.
func init() {
	os.Setenv("BORGEE_ADMIN_LOGIN", "test-admin")
	os.Setenv("BORGEE_ADMIN_PASSWORD_HASH", "$2a$10$1TyjYX4YfwjnX5EpcGsH2uY5IUVuZZm4HFZBtMz1m5yBO4qM9Ulr6")
	// TEST-FIX-3-COV performance: lower bcrypt cost in tests to MinCost (4) so admin
	// /users register paths run ~1ms instead of ~150ms (cost=10). Production
	// keeps cost=10 (var BcryptCost defaults via env BORGEE_TEST_FAST_BCRYPT
	// not set in production cmd/*). Direct override to bypass env-once init.
	auth.BcryptCost = bcrypt.MinCost
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	testPasswordHash = string(hash)
}

func NewTestServer(t *testing.T) (*httptest.Server, *store.Store, *config.Config) {
	t.Helper()

	// TEST-FIX-3-COV: use store.MigratedStoreFromTemplate instead of
	// store.Open(":memory:") + Migrate. This reduces setup from ~26ms to ~7ms
	// per call. The schema stays byte-identical because the template runs the
	// same migration path, while each test still gets an isolated file-backed
	// SQLite clone and t.Cleanup handles teardown.
	s := store.MigratedStoreFromTemplate(t)

	// PERF (was t.Setenv blocking parallel): admin env now in package init.

	cfg := &config.Config{
		JWTSecret:                "test-secret",
		NodeEnv:                  "development",
		DevAuthBypass:            false,
		DisableBackgroundWorkers: true,
		AdminUser:                "admin",
		AdminPassword:            "password123",
		UploadDir:                t.TempDir(),
		WorkspaceDir:             t.TempDir(),
		ClientDist:               t.TempDir(),
		CORSOrigin:               "*",
	}

	// ADM-0.3 (v=10): users.role enum collapsed to {'member', 'agent'}; admin
	// authority lives exclusively on /admin-api/* behind admin sessions
	// (see admin-model.md §1.2). Owner + admin fixtures here are user API
	// `member` accounts with the AP-0 default `(*, *)` wildcard — they retain
	// every user API capability without re-introducing the role short-circuit.
	// The ADM-0.2 explicit `(*, *)` splice is now redundant (the member default
	// grant covers it) and the ADM-0.3 migration sweeps any leftover wildcard
	// rows belonging to deleted role='admin' users.
	ownerEmail := "owner@test.com"
	owner := &store.User{
		DisplayName:  "Owner",
		Role:         "member",
		Email:        &ownerEmail,
		PasswordHash: testPasswordHash,
	}
	if err := s.CreateUser(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	// CM-3 fixture: owner has its own org; admin + member share owner.OrgID
	// so existing tests stay single-org. Foreign-org tests in cross_org_test.go
	// build a separate user via SeedForeignOrgUser.
	if _, err := s.CreateOrgForUser(owner, "Owner Org"); err != nil {
		t.Fatalf("create owner org: %v", err)
	}
	if err := s.GrantDefaultPermissions(owner.ID, "member"); err != nil {
		t.Fatalf("grant owner perms: %v", err)
	}

	adminEmail := "admin@test.com"
	admin := &store.User{
		// Display name retained for back-compat with existing tests that
		// resolve users by name (`testutil.GetUserIDByName(... "Admin")`).
		// ADM-0.3: this is a user API member fixture, not an admin bypass account.
		DisplayName:  "Admin",
		Role:         "member",
		Email:        &adminEmail,
		PasswordHash: testPasswordHash,
	}
	if err := s.CreateUser(admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	// CM-3: this fixture shares the owner's org so existing single-tenant tests still see
	// shared resources.
	if err := s.UpdateUser(admin.ID, map[string]any{"org_id": owner.OrgID}); err != nil {
		t.Fatalf("set admin org_id: %v", err)
	}
	if err := s.GrantDefaultPermissions(admin.ID, "member"); err != nil {
		t.Fatalf("grant admin perms: %v", err)
	}

	memberEmail := "member@test.com"
	member := &store.User{
		DisplayName:  "Member",
		Role:         "member",
		Email:        &memberEmail,
		PasswordHash: testPasswordHash,
	}
	if err := s.CreateUser(member); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if err := s.UpdateUser(member.ID, map[string]any{"org_id": owner.OrgID}); err != nil {
		t.Fatalf("set member org_id: %v", err)
	}
	if err := s.GrantDefaultPermissions(member.ID, "member"); err != nil {
		t.Fatalf("grant member perms: %v", err)
	}

	s.DB().Create(&store.InviteCode{
		Code:      "test-invite",
		CreatedBy: owner.ID,
	})

	general := &store.Channel{
		Name:       "general",
		Visibility: "public",
		CreatedBy:  owner.ID,
		Type:       "channel",
		Position:   store.GenerateInitialRank(),
		OrgID:      owner.OrgID, // CM-3.1
	}
	if err := s.CreateChannel(general); err != nil {
		t.Fatalf("create general: %v", err)
	}
	s.AddChannelMember(&store.ChannelMember{ChannelID: general.ID, UserID: owner.ID})
	s.AddChannelMember(&store.ChannelMember{ChannelID: general.ID, UserID: admin.ID})
	s.AddChannelMember(&store.ChannelMember{ChannelID: general.ID, UserID: member.ID})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// TEST-FIX-2: pass t.Context() so goroutines spawned by the server exit on
	// test teardown. DisableBackgroundWorkers keeps unrelated production ticker
	// workers out of the API harness; those workers have dedicated package tests.
	srv := server.New(t.Context(), cfg, logger, s)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		s.Close()
	})

	return ts, s, cfg
}

// NewTestServerWithFakeClock is the PERF-JWT-CLOCK variant: returns the same
// httptest server as NewTestServer plus a *clock.Fake injected into the
// server's AuthHandler for JWT iat/exp minting. Tests use the fake clock to
// advance the JWT timestamp (1s granularity) without time.Sleep.
//
// Example (token rotation test):
//
//	ts, _, _, fake := testutil.NewTestServerWithFakeClock(t)
//	tok1 := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
//	fake.Advance(2 * time.Second)
//	tok2 := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
//	// tok1 != tok2 (different iat, equivalent to the production 1s sleep path)
//
// Production path is unchanged: server.New() defaults clk=nil, and
// AuthHandler.now() uses time.Now() as it did before PERF-JWT-CLOCK.
func NewTestServerWithFakeClock(t *testing.T) (*httptest.Server, *store.Store, *config.Config, *clock.Fake) {
	t.Helper()
	ts, s, cfg := NewTestServer(t)
	// Start fake clock at real now() — JWT exp/iat are validated by
	// auth.AuthMiddleware via stdlib jwt.WithLeeway/time.Now (NOT injected),
	// so the minted exp must be in the future relative to wall-clock.
	// Tests Advance the fake clock to skip over JWT 1s iat granularity.
	fake := clock.NewFake(time.Now())
	// Reach into the test server to install the fake clock. We need a handle
	// on *server.Server but NewTestServer returns *httptest.Server only —
	// this is fine because httptest wraps srv.Handler() and we constructed
	// srv inside NewTestServer above. For the fake-clock variant we re-build:
	// reset httptest with a fresh srv that has SetClock called. To keep
	// shared fixture seeding, we reuse the store + cfg (already populated).
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(t.Context(), cfg, logger, s)
	srv.SetClock(fake)
	// Replace the handler atomically: close prior ts, mount new one.
	ts.Close()
	ts2 := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts2.Close() })
	return ts2, s, cfg, fake
}

// LoginAsAdmin posts to the new ADM-0.2 admin auth endpoint and returns the
// `borgee_admin_session` cookie value. The legacy borgee_admin_token JWT path
// was removed in ADM-0.2.
func LoginAsAdmin(t *testing.T, serverURL string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"login": "test-admin", "password": "password123"})
	resp, err := http.Post(serverURL+"/admin-api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("admin login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("admin login failed (%d): %s", resp.StatusCode, b)
	}

	for _, c := range resp.Cookies() {
		if c.Name == "borgee_admin_session" {
			return c.Value
		}
	}
	t.Fatal("no borgee_admin_session cookie in admin login response")
	return ""
}

// AdminJSON sends a request authenticated by the borgee_admin_session cookie.
// User API JSON helper does not work for admin API endpoints anymore.
func AdminJSON(t *testing.T, method, url, sessionToken string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: "borgee_admin_session", Value: sessionToken})
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]any
	json.Unmarshal(respBody, &result)
	return resp, result
}

func LoginAs(t *testing.T, serverURL, email, password string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login failed (%d): %s", resp.StatusCode, b)
	}

	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c.Value
		}
	}
	t.Fatal("no borgee_token cookie in login response")
	return ""
}

func JSON(t *testing.T, method, url, token string, body any) (*http.Response, map[string]any) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		// ADM-0.2: /admin-api/* is gated by admin.RequireAdmin, which only
		// recognizes the borgee_admin_session cookie. /api/* is gated by
		// auth.AuthMiddleware which uses borgee_token + Bearer. Same helper
		// signature picks the right API path by URL prefix.
		if strings.Contains(url, "/admin-api/") {
			req.AddCookie(&http.Cookie{Name: "borgee_admin_session", Value: token})
		} else {
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]any
	json.Unmarshal(respBody, &result)
	return resp, result
}

func CreateChannel(t *testing.T, serverURL, token, name, visibility string) map[string]any {
	t.Helper()
	resp, data := JSON(t, "POST", serverURL+"/api/v1/channels", token, map[string]string{
		"name":       name,
		"visibility": visibility,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel %q: status %d, body %v", name, resp.StatusCode, data)
	}
	ch, _ := data["channel"].(map[string]any)
	return ch
}

func GetGeneralChannelID(t *testing.T, serverURL, token string) string {
	t.Helper()
	resp, data := JSON(t, http.MethodGet, serverURL+"/api/v1/channels", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list channels: status %d, body %v", resp.StatusCode, data)
	}
	channels, ok := data["channels"].([]any)
	if !ok {
		t.Fatalf("expected channels array, got %v", data)
	}
	for _, raw := range channels {
		ch, ok := raw.(map[string]any)
		if ok && ch["name"] == "general" {
			id, ok := ch["id"].(string)
			if !ok || id == "" {
				t.Fatalf("expected non-empty general channel id in %v", ch)
			}
			return id
		}
	}
	t.Fatal("general channel not found")
	return ""
}

func GetUserIDByName(t *testing.T, serverURL, token, displayName string) string {
	t.Helper()
	// Use admin API since /api/v1/users is removed
	adminToken := LoginAsAdmin(t, serverURL)
	resp, data := JSON(t, http.MethodGet, serverURL+"/admin-api/v1/users", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin list users: status %d, body %v", resp.StatusCode, data)
	}
	users, ok := data["users"].([]any)
	if !ok {
		t.Fatalf("expected users array, got %v", data)
	}
	for _, raw := range users {
		u, ok := raw.(map[string]any)
		if ok && u["display_name"] == displayName {
			id, ok := u["id"].(string)
			if !ok || id == "" {
				t.Fatalf("expected non-empty user id in %v", u)
			}
			return id
		}
	}
	t.Fatalf("user %q not found", displayName)
	return ""
}

func PostMessage(t *testing.T, serverURL, token, channelID, content string) map[string]any {
	t.Helper()
	resp, data := JSON(t, "POST", serverURL+"/api/v1/channels/"+channelID+"/messages", token, map[string]string{
		"content": content,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post message: status %d, body %v", resp.StatusCode, data)
	}
	msg, _ := data["message"].(map[string]any)
	return msg
}

func DialWS(t *testing.T, serverURL, path, token string) *websocket.Conn {
	t.Helper()
	if path == "" {
		path = "/ws"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + path
	header := http.Header{}
	if token != "" {
		header.Set("Cookie", auth.CookieName+"="+url.QueryEscape(token))
		header.Set("Authorization", "Bearer "+token)
	}

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("ws dial %s: %v", path, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func WSWriteJSON(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("ws set write deadline: %v", err)
	}
	if err := conn.WriteJSON(v); err != nil {
		t.Fatalf("ws write json: %v", err)
	}
}

func WSReadJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("ws set read deadline: %v", err)
	}
	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("ws read json: %v", err)
	}
	return msg
}

func WSReadUntil(t *testing.T, conn *websocket.Conn, eventType string) map[string]any {
	t.Helper()
	for {
		msg := WSReadJSON(t, conn)
		if msg["type"] == eventType {
			return msg
		}
	}
}

type SSEEvent struct {
	Event string
	ID    string
	Data  string
}

type SSEClient struct {
	resp    *http.Response
	scanner *bufio.Scanner
}

func DialSSE(t *testing.T, serverURL, token string) *SSEClient {
	return DialSSEWithLastEventID(t, serverURL, token, "")
}

func DialSSEWithLastEventID(t *testing.T, serverURL, token, lastID string) *SSEClient {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, serverURL+"/api/v1/stream", nil)
	if err != nil {
		t.Fatalf("new sse request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse connect: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("sse connect status %d: %s", resp.StatusCode, b)
	}

	scanner := bufio.NewScanner(resp.Body)
	client := &SSEClient{resp: resp, scanner: scanner}
	t.Cleanup(client.Close)
	return client
}

func (c *SSEClient) Close() {
	if c != nil && c.resp != nil && c.resp.Body != nil {
		c.resp.Body.Close()
	}
}

func (c *SSEClient) ReadEvent(t *testing.T) SSEEvent {
	t.Helper()
	event := SSEEvent{}
	for c.scanner.Scan() {
		line := c.scanner.Text()
		if line == "" {
			if event.Event != "" || event.ID != "" || event.Data != "" {
				return event
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if v, ok := strings.CutPrefix(line, "event: "); ok {
			event.Event = v
			continue
		}
		if v, ok := strings.CutPrefix(line, "id: "); ok {
			event.ID = v
			continue
		}
		if v, ok := strings.CutPrefix(line, "data: "); ok {
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += v
		}
	}
	if err := c.scanner.Err(); err != nil {
		t.Fatalf("sse read: %v", err)
	}
	t.Fatal("sse stream ended")
	return SSEEvent{}
}

// SeedLegacyAgent inserts a role='agent' user that simulates a pre-AP-0-bis
// agent (only message.send granted, NO message.read). Used by tests for
// migration v=8 (ap_0_bis_message_read) idempotent backfill verification, and
// the reverse 403 assertion on GET /channels/:id/messages when the row is
// absent.
func SeedLegacyAgent(t *testing.T, s *store.Store, displayName string) *store.User {
	t.Helper()
	email := strings.ToLower(strings.ReplaceAll(displayName, " ", "-")) + "@legacy-agent.test"
	u := &store.User{
		DisplayName:  displayName,
		Role:         "agent",
		Email:        &email,
		PasswordHash: testPasswordHash,
	}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create legacy agent: %v", err)
	}
	// CHN-1.2: ListChannelsWithUnread scopes public discovery by `c.org_id =
	// u.org_id`, so the legacy agent must share the fixture owner's org to see
	// `general` (replaces the dropped AddAllUsersToChannel auto-join).
	if owner, err := s.GetUserByEmail("owner@test.com"); err == nil && owner != nil {
		_ = s.UpdateUser(u.ID, map[string]any{"org_id": owner.OrgID})
		u.OrgID = owner.OrgID
		// Also auto-join `general` so the agent is a channel member (legacy
		// behavior before AddAllUsersToChannel was removed).
		if gen, err := s.GetChannelByNameInOrg(owner.OrgID, "general"); err == nil && gen != nil {
			_ = s.AddChannelMember(&store.ChannelMember{ChannelID: gen.ID, UserID: u.ID})
		}
	}
	// Pre-AP-0-bis: legacy agents only had message.send. Do NOT grant read —
	// that's exactly what migration v=8 backfills.
	if err := s.GrantPermission(&store.UserPermission{
		UserID:     u.ID,
		Permission: "message.send",
		Scope:      "*",
		GrantedAt:  time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("grant legacy agent send: %v", err)
	}
	return u
}

func CreateAgent(t *testing.T, serverURL, token, displayName string) map[string]any {
	t.Helper()
	resp, data := JSON(t, http.MethodPost, serverURL+"/api/v1/agents", token, map[string]any{
		"display_name": displayName,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent %q: status %d, body %v", displayName, resp.StatusCode, data)
	}
	agent, _ := data["agent"].(map[string]any)
	return agent
}

// SeedForeignOrgUser creates a member user in a brand-new org (separate from
// the test fixture's "Owner Org") and grants default member perms. Used by
// CM-3 cross-org 403 reverse assertions. Returns the user with OrgID populated.
func SeedForeignOrgUser(t *testing.T, s *store.Store, displayName, email string) *store.User {
	t.Helper()
	u := &store.User{
		DisplayName:  displayName,
		Role:         "member",
		Email:        &email,
		PasswordHash: testPasswordHash,
	}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create foreign-org user: %v", err)
	}
	if _, err := s.CreateOrgForUser(u, displayName+" Org"); err != nil {
		t.Fatalf("create foreign org: %v", err)
	}
	if err := s.GrantDefaultPermissions(u.ID, "member"); err != nil {
		t.Fatalf("grant foreign-org perms: %v", err)
	}
	// Reload to get OrgID populated on the returned struct.
	got, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("reload foreign-org user: %v", err)
	}
	return got
}
