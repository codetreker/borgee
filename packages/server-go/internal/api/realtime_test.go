// Package api_test — rt_4_presence_test.go: RT-4 channel presence
// indicator member-only GET + 反约束守门.
//
// Pins:
//   REG-RT4-001 TestRT_NoSchemaChange
//   REG-RT4-002 TestRT_GetPresence_MemberHappyPath + _NonMemberRejected
//                + _Unauthorized401
//   REG-RT4-003 TestRT_TypingPathByteIdentical (grep 检查 rt_4 在
//               ws/client.go::handleTyping block 0 hit)
//   REG-RT4-004 TestRT_NoAdminPresencePath
//   REG-RT4-005 TestRT_NoPresenceQueue
package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-RT4-001 — 0 schema 改.
func TestRT_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "rt_4_") {
			t.Errorf("RT-4 设计 ① broken — found schema migration %q (must be 0 schema)", e.Name())
		}
	}
}

// REG-RT4-003 — 既有 RT-2 typing WS path byte-identical — ws/client.go
// `case "typing":` block 不漂入 rt_4 字面.
func TestRT_TypingPathByteIdentical(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "ws", "client.go"))
	if err != nil {
		t.Fatalf("read ws/client.go: %v", err)
	}
	src := string(body)
	idx := strings.Index(src, `case "typing":`)
	if idx < 0 {
		t.Skip("typing case not found in ws/client.go (existing path may have moved)")
	}
	end := idx + 800
	if end > len(src) {
		end = len(src)
	}
	block := src[idx:end]
	for _, tok := range []string{"rt_4", "RT4", "rt4"} {
		if strings.Contains(block, tok) {
			t.Errorf("既有 typing path 漂入 RT-4 — token %q 在 client.go::typing block (RT-4 边界 ④ broken)", tok)
		}
	}
}

func setupRT4(t *testing.T) (string, string, string, string, *store.Store) {
	t.Helper()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels", ownerToken,
		map[string]any{"name": "rt4-test", "visibility": "public"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel: %d %v", resp.StatusCode, body)
	}
	ch, _ := body["channel"].(map[string]any)
	channelID, _ := ch["id"].(string)
	if channelID == "" {
		t.Fatalf("channel.id missing in response: %v", body)
	}
	if err := s.AddChannelMember(&store.ChannelMember{
		ChannelID: channelID, UserID: member.ID,
	}); err != nil {
		t.Fatalf("AddChannelMember: %v", err)
	}
	_ = owner
	return ts.URL, ownerToken, memberToken, channelID, s
}

// REG-RT4-002a — member HappyPath GET /presence → 200 + shape.
func TestRT_GetPresence_MemberHappyPath(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, channelID, _ := setupRT4(t)

	resp, body := testutil.JSON(t, http.MethodGet,
		url+"/api/v1/channels/"+channelID+"/presence", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	for _, k := range []string{"online_user_ids", "counted_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing key %q in response: %v", k, body)
		}
	}
	if _, ok := body["online_user_ids"].([]any); !ok {
		t.Errorf("online_user_ids: got %T, want []any", body["online_user_ids"])
	}
}

// REG-RT4-002b — non-member 403.
func TestRT_GetPresence_NonMemberRejected(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, _, s := setupRT4(t)
	// Create a second channel where the calling user is NOT a member.
	resp, body := testutil.JSON(t, http.MethodPost,
		url+"/api/v1/channels", ownerToken,
		map[string]any{"name": "rt4-private", "visibility": "public"})
	ch, _ := body["channel"].(map[string]any)
	otherChannel, _ := ch["id"].(string)
	_ = resp
	_ = s
	// member@test.com is NOT added to otherChannel.
	memberToken := testutil.LoginAs(t, url, "member@test.com", "password123")
	resp, _ = testutil.JSON(t, http.MethodGet,
		url+"/api/v1/channels/"+otherChannel+"/presence", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-member: got %d, want 403", resp.StatusCode)
	}
}

// REG-RT4-002c — 401 unauthorized.
func TestRT_GetPresence_Unauthorized401(t *testing.T) {
	t.Parallel()
	url, _, _, channelID, _ := setupRT4(t)
	resp, _ := testutil.JSON(t, http.MethodGet,
		url+"/api/v1/channels/"+channelID+"/presence", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}
}

// REG-RT4-004 — admin god-mode 不挂 PATCH/POST/PUT/DELETE 在 admin-api/v1/.../presence.
func TestRT_NoAdminPresencePath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT|GET)[^"]*admin-api/v[0-9]+/[^"]*presence`)
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
				t.Errorf("RT-4 设计 ③ broken — admin-rail presence path in %s: %q",
					p, fb[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-RT4-005 — AST 锁链延伸第 18 处 forbidden 3 token.
func TestRT_NoPresenceQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingPresenceQuery",
		"presenceQueueRetry",
		"deadLetterPresence",
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
				t.Errorf("AST 锁链延伸第 18 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-RT4-005b — 0 新 WS frame (grep 检查 `presence_changed | presenceChanged
// | user_online_pushed` 在 internal/ws + internal/api 0 hit).
func TestRT_NoNewPresenceFrame(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"presence_changed",
		"presenceChanged",
		"user_online_pushed",
	}
	dirs := []string{filepath.Join("..", "ws"), filepath.Join("..", "api")}
	for _, dir := range dirs {
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
					t.Errorf("RT-4 设计 ② broken — token %q in %s (no new WS frame allowed)", tok, p)
				}
			}
			return nil
		})
	}
}
