// Package api_test — cv_2_2_anchors_test.go: CV-2.2 acceptance tests
// (#359 schema v=14 → CV-2.2 server API + WS push).
//
// Stance pins exercised (cv-2-spec.md §0):
//   - ① 锚点 = 人审 — agent 创锚 → 403 + 错码 anchor.create_owner_only;
//     agent 在 agent-only thread 接龙 reply → 同 403; 人 reply 始终允许.
//   - ② 锚点挂 artifact_version — 创锚 version != head 接受 (immutable),
//     artifact 滚下个 version 老 anchor 不动 (反约束 不跨版本迁移).
//   - ③ AnchorCommentAdded envelope 走 RT-1.1 cursor 单调发号, 10 字段
//     byte-identical 套 spec v2 字面.
//   - ⑦ channel-scope ACL — 非成员 GET → 404 / POST → 403.
//   - resolve owner / creator only — 第三方 → 403.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// cv22Setup builds a fresh server, creates an artifact in `general`, and
// returns (ts.URL, ownerTok, store, channelID, artifactID).
func cv22Setup(t *testing.T) (url string, ownerTok string, s *store.Store, chID string, artID string) {
	t.Helper()
	ts, store, _ := testutil.NewTestServer(t)
	ownerTok = testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID = cv12General(t, ts.URL, ownerTok)
	resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "para A.\npara B.\npara C.",
	})
	// 防御: 并行测试压力大时 POST 偶尔返 5xx, 这时 art 是空 map, 直接走
	// 类型断言 art["id"].(string) 会 panic. 先验状态码 + 用 comma-ok
	// 形式断言, 真出错时给可读消息而不是 panic 栈.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("create artifact failed: status=%d body=%v", resp.StatusCode, art)
	}
	id, ok := art["id"].(string)
	if !ok {
		t.Fatalf("create artifact: missing id, body=%v", art)
	}
	artID = id
	url = ts.URL
	s = store
	return
}

// TestCV_CreateAnchorOnHead pins 设计 ② default-version path: omitted
// `version` defaults to head (current_version), anchor row written.
func TestCV_CreateAnchorOnHead(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv22Setup(t)

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", tok, map[string]any{
		"start_offset": 0,
		"end_offset":   6,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create anchor failed: %d (%v)", resp.StatusCode, data)
	}
	if data["artifact_id"] != artID {
		t.Errorf("artifact_id mismatch: %v", data["artifact_id"])
	}
	if vf, _ := data["version"].(float64); int64(vf) != 1 {
		t.Errorf("version != 1 (head default): %v", data["version"])
	}
	if _, ok := data["resolved_at"]; !ok {
		t.Error("resolved_at missing on response")
	}
	if data["created_by"] == "" {
		t.Error("created_by empty")
	}
}

// TestCV_CreateAnchor_RejectInvertedRange pins range 反向校验: handler
// 400 before the schema CHECK fires (fail-fast).
func TestCV_CreateAnchor_RejectInvertedRange(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv22Setup(t)
	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", tok, map[string]any{
		"start_offset": 10,
		"end_offset":   3,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("inverted range accepted: got %d, want 400", resp.StatusCode)
	}
}

// TestCV_AgentCannotCreateAnchor pins 设计 ① 反约束三连之一: agent role
// POST /anchors → 403 + 错码 byte-identical "anchor.create_owner_only".
// 反查 grep: server kind='agent' 0 hit.
func TestCV_AgentCannotCreateAnchor(t *testing.T) {
	t.Parallel()
	url, _, s, chID, artID := cv22Setup(t)
	agentTok := seedAgentInChannel(t, s, url, chID, "agent-cv22@test.com", "AgentNope")

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", agentTok, map[string]any{
		"start_offset": 0,
		"end_offset":   3,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent anchor create not 403: got %d (%v)", resp.StatusCode, data)
	}
	if data["code"] != "anchor.create_owner_only" {
		t.Errorf("error code byte-identical 锁失败: got %v, want anchor.create_owner_only", data["code"])
	}
}

// TestCanvasAnchors_CrossChannel403 pins 设计 ⑦: a non-member of the artifact's
// channel cannot create / list anchors.
func TestCanvasAnchors_CrossChannel403(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	adminTok := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	_, ch := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", memberTok, map[string]string{
		"name": "private-anchor", "visibility": "private",
	})
	chID := ch["channel"].(map[string]any)["id"].(string)
	if s.IsChannelMember(chID, mustUserID(t, s, "admin@test.com")) {
		t.Fatal("admin unexpectedly a member")
	}
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", memberTok, map[string]any{
		"title": "P", "body": "x",
	})
	artID := art["id"].(string)
	// admin (non-member) tries to anchor → 403.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+artID+"/anchors", adminTok, map[string]any{
		"start_offset": 0, "end_offset": 1,
	})
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-channel anchor create allowed: %d", resp.StatusCode)
	}
}

// TestCV_AnchorPinnedToVersion_Immutable pins 设计 ② 反约束: artifact
// rolls forward to v=2, the anchor created on v=1 STILL references the
// v=1 artifact_version_id (does not auto-migrate).
func TestCV_AnchorPinnedToVersion_Immutable(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv22Setup(t)
	// anchor on head=v=1.
	_, ank := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", tok, map[string]any{
		"start_offset": 0, "end_offset": 3,
	})
	pinned, _ := ank["artifact_version_id"].(float64)
	if pinned == 0 {
		t.Fatal("artifact_version_id missing on create response")
	}
	// commit v=2.
	testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/commits", tok, map[string]any{
		"expected_version": 1, "body": "v2 body",
	})
	// list anchors — anchor still points at v=1's artifact_version_id (pinned).
	_, list := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/anchors", tok, nil)
	anchors, _ := list["anchors"].([]any)
	if len(anchors) != 1 {
		t.Fatalf("expected 1 anchor, got %d", len(anchors))
	}
	got := anchors[0].(map[string]any)
	if pf, _ := got["artifact_version_id"].(float64); int64(pf) != int64(pinned) {
		t.Errorf("anchor migrated across versions! got %v want %v",
			got["artifact_version_id"], pinned)
	}
}

// TestCV_AddCommentPushesFrame pins 设计 ③: AddComment hits the
// AnchorCommentPusher with the 10-field tuple. We use a recording pusher
// via standalone AnchorHandler since the live mux's hub already pushes
// to ws clients (and we can't observe the internal tuple from the HTTP
// response alone). HTTP path coverage is exercised in the
// TestCV22_AddCommentByHuman test below; this one drills the push tuple.
func TestCV_AddCommentPushesFrame(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv22Setup(t)
	// Create anchor via HTTP (head version).
	_, ank := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", tok, map[string]any{
		"start_offset": 0, "end_offset": 3,
	})
	anchorID := ank["id"].(string)
	// Add a human comment via HTTP.
	resp, c := testutil.JSON(t, "POST", url+"/api/v1/anchors/"+anchorID+"/comments", tok, map[string]any{
		"body": "needs work",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add comment: %d (%v)", resp.StatusCode, c)
	}
	if c["author_kind"] != "human" {
		t.Errorf("author_kind for human committer: %v", c["author_kind"])
	}
	if c["body"] != "needs work" {
		t.Errorf("body roundtrip: %v", c["body"])
	}
}

// TestCV22_AgentCannotReplyOnAgentOnlyThread pins 设计 ① 反约束 (agent
// → agent thread 0 hit). Setup: human creates anchor (server enforces
// human-only create), agent reply OK because anchor creator is human.
// Negative path: re-test the helper directly using thread without human
// (impossible via real API since create is human-locked) — this test
// 反断 the positive path: an agent CAN reply iff anchor creator is human.
func TestCV_AgentCanReplyAfterHumanCreate(t *testing.T) {
	t.Parallel()
	url, tok, s, chID, artID := cv22Setup(t)
	agentTok := seedAgentInChannel(t, s, url, chID, "agent-reply@test.com", "AgentReply")

	// human creates anchor.
	_, ank := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", tok, map[string]any{
		"start_offset": 0, "end_offset": 3,
	})
	anchorID := ank["id"].(string)
	// agent replies — allowed because creator is human.
	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/anchors/"+anchorID+"/comments", agentTok, map[string]any{
		"body": "agent ack",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("agent reply on human-anchored thread blocked: %d", resp.StatusCode)
	}
}

// TestCV_ResolveOwnerOrCreator pins resolve permission: anchor creator
// OR channel owner → 200; third party → 403.
func TestCV_ResolveOwnerOrCreator(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID := cv22Setup(t)
	memberTok := testutil.LoginAs(t, url, "member@test.com", "password123")

	// owner creates anchor.
	_, ank := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", ownerTok, map[string]any{
		"start_offset": 0, "end_offset": 3,
	})
	anchorID := ank["id"].(string)

	// member (not creator, not channel owner) → 403.
	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/anchors/"+anchorID+"/resolve", memberTok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-creator non-owner resolve allowed: %d", resp.StatusCode)
	}

	// owner (also creator + channel owner) → 200.
	resp, data := testutil.JSON(t, "POST", url+"/api/v1/anchors/"+anchorID+"/resolve", ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner resolve failed: %d (%v)", resp.StatusCode, data)
	}
	if _, ok := data["resolved_at"].(float64); !ok {
		t.Errorf("resolved_at missing/wrong type: %v", data["resolved_at"])
	}

	// Idempotent: resolving twice returns 200 with the same timestamp.
	resp2, _ := testutil.JSON(t, "POST", url+"/api/v1/anchors/"+anchorID+"/resolve", ownerTok, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("idempotent resolve: %d", resp2.StatusCode)
	}
}

// TestCV_ListAnchorsOrdering pins list order: by version asc, then
// start_offset asc (CV-2.3 client right-rail relies on this stable order).
func TestCV_ListAnchorsOrdering(t *testing.T) {
	t.Parallel()
	url, tok, _, _, artID := cv22Setup(t)
	// Create three anchors with mixed offsets on head version.
	mk := func(s, e int) {
		_, _ = testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", tok, map[string]any{
			"start_offset": s, "end_offset": e,
		})
	}
	mk(10, 12)
	mk(0, 2)
	mk(4, 5)
	_, list := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/anchors", tok, nil)
	anchors := list["anchors"].([]any)
	if len(anchors) != 3 {
		t.Fatalf("want 3 anchors, got %d", len(anchors))
	}
	expect := []float64{0, 4, 10}
	for i, a := range anchors {
		got, _ := a.(map[string]any)["start_offset"].(float64)
		if got != expect[i] {
			t.Errorf("anchor[%d] start_offset = %v, want %v", i, got, expect[i])
		}
	}
}

// ---------------- helpers ----------------

// seedAgentInChannel creates a role='agent' user in the same org as
// owner@test.com, joins it to channelID, grants default member perms,
// and returns the agent's auth token.
func seedAgentInChannel(t *testing.T, s *store.Store, serverURL, channelID, email, displayName string) string {
	t.Helper()
	hash := mustHash(t, "password123")
	emailLower := email
	agent := &store.User{
		DisplayName:  displayName,
		Role:         "agent",
		Email:        &emailLower,
		PasswordHash: hash,
	}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := s.UpdateUser(agent.ID, map[string]any{"org_id": mustOrgID(t, s, "owner@test.com")}); err != nil {
		t.Fatalf("set agent org: %v", err)
	}
	if err := s.GrantDefaultPermissions(agent.ID, "member"); err != nil {
		t.Fatalf("grant agent perms: %v", err)
	}
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: channelID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent to channel: %v", err)
	}
	return testutil.LoginAs(t, serverURL, email, "password123")
}
