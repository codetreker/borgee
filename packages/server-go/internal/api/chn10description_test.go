// Package api_test — chn_10_description_test.go: CHN-10 owner-only PUT
// /api/v1/channels/{channelId}/description endpoint + 约束守门.
//
// Pins:
//   REG-CHN10-001 TestChn10description_NoSchemaChange (filepath.Walk migrations/)
//   REG-CHN10-002 TestCHN_PutDescription_OwnerHappyPath
//                 + _NonOwnerRejected + _Unauthorized401
//   REG-CHN10-003 TestCHN_PutDescription_LengthCap500
//   REG-CHN10-004 TestCHN_TopicPathByteIdentical (grep 检查 dm_10/chn_10
//                 字面 在 channels.go::handleSetTopic block 0 hit)
//   REG-CHN10-005 TestCHN_NoAdminDescriptionPath
//   REG-CHN10-006 TestCHN_NoDescriptionQueue
package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-CHN10-001 — 0 schema 改 (grep 检查 migrations/chn_10_*).
func TestChn10description_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "chn_10_") {
			t.Errorf("CHN-10 设计第 1 条 broken — found schema migration %q (must be 0 schema)", name)
		}
	}
	// DescriptionMaxLength 字节级一致 跟 channels.topic GORM size:500.
	if api.DescriptionMaxLength != 500 {
		t.Errorf("DescriptionMaxLength: got %d, want 500 (字节级一致 跟 channels.topic GORM size:500)", api.DescriptionMaxLength)
	}
}

// chnHelper — minimal channel + owner setup. Returns ownerToken, owner,
// non-owner-member, and a channel they both belong to.
type chn10Setup struct {
	ts          *httptestServerSurrogate // type alias-like
	store       *store.Store
	ownerToken  string
	memberToken string
	owner       *store.User
	member      *store.User
	channelID   string
}

// httptestServerSurrogate aliases httptest.Server fields used here. We
// don't import httptest directly because testutil.NewTestServer returns
// *httptest.Server typed.
type httptestServerSurrogate = struct {
	URL string
}

func setupCHN10(t *testing.T) (string, string, string, string, *store.Store) {
	t.Helper()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	// Create channel via POST /api/v1/channels (owner becomes creator).
	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels", ownerToken,
		map[string]any{"name": "chn10-test", "visibility": "public"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel: %d %v", resp.StatusCode, body)
	}
	ch, _ := body["channel"].(map[string]any)
	channelID, _ := ch["id"].(string)
	if channelID == "" {
		t.Fatalf("channel.id missing in response: %v", body)
	}
	// Add member.
	if err := s.AddChannelMember(&store.ChannelMember{
		ChannelID: channelID, UserID: member.ID,
	}); err != nil {
		t.Fatalf("AddChannelMember: %v", err)
	}
	_ = owner
	return ts.URL, ownerToken, memberToken, channelID, s
}

// REG-CHN10-002a — owner HappyPath PUT /description → 200.
func TestCHN_PutDescription_OwnerHappyPath(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, channelID, s := setupCHN10(t)

	resp, body := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", ownerToken,
		map[string]any{"description": "首页频道说明文本"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	// Verify topic column written.
	ch, err := s.GetChannelByID(channelID)
	if err != nil || ch == nil {
		t.Fatalf("reload channel: %v", err)
	}
	if ch.Topic != "首页频道说明文本" {
		t.Errorf("topic: got %q, want %q", ch.Topic, "首页频道说明文本")
	}
}

// REG-CHN10-002b — non-owner member 403 (设计第 2 条 owner-only).
func TestCHN_PutDescription_NonOwnerRejected(t *testing.T) {
	t.Parallel()
	url, _, memberToken, channelID, _ := setupCHN10(t)

	resp, _ := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", memberToken,
		map[string]any{"description": "非 owner 不能改"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner: got %d, want 403 (owner-only ACL 对齐链第 20 处)", resp.StatusCode)
	}
}

// REG-CHN10-002c — 401 unauthorized (空 token).
func TestCHN_PutDescription_Unauthorized401(t *testing.T) {
	t.Parallel()
	url, _, _, channelID, _ := setupCHN10(t)

	resp, _ := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", "",
		map[string]any{"description": "x"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}
}

// REG-CHN10-003 — length cap 500 (501 reject 400).
func TestCHN_PutDescription_LengthCap500(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, channelID, _ := setupCHN10(t)

	// 500 ASCII chars → OK.
	exact500 := strings.Repeat("a", 500)
	resp, body := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", ownerToken,
		map[string]any{"description": exact500})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("500 chars: got %d, want 200: %v", resp.StatusCode, body)
	}

	// 501 chars → 400.
	over501 := strings.Repeat("b", 501)
	resp, _ = testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/"+channelID+"/description", ownerToken,
		map[string]any{"description": over501})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("501 chars: got %d, want 400 (length cap %d)", resp.StatusCode, api.DescriptionMaxLength)
	}
}

// REG-CHN10-004 — 既有 PUT /topic 字节级一致 不变 — handleSetTopic
// block 内grep 检查 `chn_10|description` 0 hit (CHN-10 不漂入既有 path).
func TestCHN_TopicPathByteIdentical(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "api", "channels.go"))
	if err != nil {
		t.Fatalf("read channels.go: %v", err)
	}
	src := string(body)
	idx := strings.Index(src, "func (h *ChannelHandler) handleSetTopic")
	if idx < 0 {
		t.Fatal("handleSetTopic not found in channels.go")
	}
	// Take a 2KB slice after the function start.
	end := idx + 2000
	if end > len(src) {
		end = len(src)
	}
	block := src[idx:end]
	for _, tok := range []string{"chn_10", "DescriptionMaxLength", "/description"} {
		if strings.Contains(block, tok) {
			t.Errorf("既有 PUT /topic block 漂移 — token %q 在 handleSetTopic block (CHN-10 设计第 4 条 broken)", tok)
		}
	}
}

// REG-CHN10-002d — channel not found → 404.
func TestCHN_PutDescription_ChannelNotFound(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, _, _ := setupCHN10(t)
	resp, _ := testutil.JSON(t, http.MethodPut,
		url+"/api/v1/channels/00000000-0000-0000-0000-000000000000/description",
		ownerToken,
		map[string]any{"description": "x"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not-found: got %d, want 404", resp.StatusCode)
	}
}

// REG-CHN10-002e — invalid JSON body → 400 (bumps handler coverage).
func TestCHN_PutDescription_InvalidJSONBody(t *testing.T) {
	t.Parallel()
	urlBase, ownerToken, _, channelID, _ := setupCHN10(t)
	// raw HTTP request with non-JSON body.
	req, _ := http.NewRequest(http.MethodPut,
		urlBase+"/api/v1/channels/"+channelID+"/description",
		strings.NewReader("not json {{{"))
	req.AddCookie(&http.Cookie{Name: "borgee_token", Value: ownerToken})
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid-json: got %d, want 400", resp.StatusCode)
	}
}

// REG-CHN10-005 — admin god-mode 不挂 PATCH/PUT/POST/DELETE 在 admin-api/v1/.../description.
func TestCHN_NoAdminDescriptionPath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/[^"]*description`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			fb, _ := os.ReadFile(p)
			if loc := pat.FindIndex(fb); loc != nil {
				t.Errorf("CHN-10 设计第 2 条 broken — admin description path in %s: %q",
					p, fb[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-CHN10-006 — AST 对齐链延伸第 17 处 forbidden 3 token.
func TestCHN_NoDescriptionQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingDescription",
		"descriptionQueue",
		"deadLetterDescription",
	}
	dir := filepath.Join("..", "api")
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fb, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(fb), tok) {
				t.Errorf("AST 对齐链延伸第 17 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}
