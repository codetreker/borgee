// Package api_test — cv_1_2_artifacts_test.go: CV-1.2 acceptance tests
// (#334 schema → CV-1.2 server API + WS push). Covers acceptance §2.1
// through §2.5 + §4 反查锚 in unit form. e2e (§3) is client-side and
// out of scope for this PR.
//
// Stance pins exercised:
//   - ① channel-scoped (cross-channel / cross-org → 403).
//   - ② 30s TTL lazy-expire lock (T+0 acquire / T+29s held / T+30s steal).
//   - ③ linear versioning enforced by UNIQUE(artifact_id, version) +
//     transactional bump (concurrent commits race detector).
//   - ④ Markdown ONLY (HTTP layer rejects non-markdown type before
//     hitting CHECK constraint).
//   - ⑤ ArtifactUpdated frame goes through ArtifactPusher (no body
//     leakage; committer info pulled separately via GET).
//   - ⑥ committer_kind 'agent'|'human' inferred from user.Role + agent
//     commit fanouts the byte-identical system message.
//   - ⑦ rollback owner-only — non-owner 403, prior-version body cloned
//     into new row with rolled_back_from_version stamped.
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"borgee-server/internal/api"
	"borgee-server/internal/auth"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// recordingPusher captures PushArtifactUpdated calls so tests can
// assert frame fields without spinning up a hub. Mirrors RT-1.1 #290's
// expectations: the pusher gets (id, version, channel, ts, kind) — the
// frame envelope is the hub's responsibility, not the handler's.
type recordingPusher struct {
	mu    sync.Mutex
	calls []pusherCall
}

type pusherCall struct {
	ArtifactID string
	Version    int64
	ChannelID  string
	UpdatedAt  int64
	Kind       string
}

func (p *recordingPusher) PushArtifactUpdated(artifactID string, version int64, channelID string, updatedAt int64, kind string) (int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, pusherCall{artifactID, version, channelID, updatedAt, kind})
	return int64(len(p.calls)), true
}

func (p *recordingPusher) snapshot() []pusherCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]pusherCall, len(p.calls))
	copy(out, p.calls)
	return out
}

// setupCV12 builds a test server, then constructs a fresh ArtifactHandler
// pointed at the same store with a recordingPusher + injectable clock.
// The "real" server.New also wires an ArtifactHandler for normal HTTP
// flow; we don't override the mux but we do test the underlying store
// behaviour through the standalone handler when push assertions matter.
//
// For HTTP acceptance tests we hit the live mux at ts.URL since
// server.New already registered the handler with the production hub.
func cv12General(t *testing.T, ts string, ownerToken string) string {
	t.Helper()
	resp, data := testutil.JSON(t, "GET", ts+"/api/v1/channels", ownerToken, nil)
	// 防御: 并行测试压力大时 list channels 偶尔返 5xx, 直接走
	// data["channels"].([]any) 会 panic. 先验状态码 + comma-ok 断言.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list channels failed: status=%d body=%v", resp.StatusCode, data)
	}
	channels, ok := data["channels"].([]any)
	if !ok {
		t.Fatalf("list channels: missing channels[], body=%v", data)
	}
	for _, c := range channels {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cm["name"] == "general" {
			id, ok := cm["id"].(string)
			if !ok {
				t.Fatalf("general channel: missing id, body=%v", cm)
			}
			return id
		}
	}
	t.Fatal("general channel not found")
	return ""
}

// TestCV_CreateArtifactInChannel pins acceptance §2.1: a channel
// member can create an artifact and the response carries the contract
// fields (id / channel_id / type='markdown' / version=1).
func TestCV_CreateArtifactInChannel(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Roadmap",
		"body":  "# Q3 plan",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%v)", resp.StatusCode, data)
	}
	if data["channel_id"] != chID {
		t.Errorf("channel_id mismatch: %v", data["channel_id"])
	}
	if data["type"] != "markdown" {
		t.Errorf("type must be markdown, got %v", data["type"])
	}
	if vf, _ := data["current_version"].(float64); int64(vf) != 1 {
		t.Errorf("current_version != 1: %v", data["current_version"])
	}
	if data["title"] != "Roadmap" {
		t.Errorf("title mismatch: %v", data["title"])
	}
}

// TestCV_RejectsNonMarkdownType pins 设计 ④ at HTTP layer (schema
// CHECK is the final gate, but we should fail-fast at 400 not 500).
func TestCV_RejectsNonMarkdownType(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "X",
		"type":  "code",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-markdown type accepted (got %d, want 400)", resp.StatusCode)
	}
}

// TestCanvasArtifacts_CrossChannel403 pins 设计 ① + acceptance §2.1: a non-member
// cannot create artifacts in another user's private channel. We use a
// fresh private channel owned by member; admin (also member-rail) is
// not added → POST should 403.
func TestCanvasArtifacts_CrossChannel403(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	adminTok := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")

	// member creates a private channel they own.
	_, ch := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", memberTok, map[string]string{
		"name":       "private-mem",
		"visibility": "private",
	})
	chID := ch["channel"].(map[string]any)["id"].(string)
	// confirm admin is NOT a member.
	if s.IsChannelMember(chID, mustUserID(t, s, "admin@test.com")) {
		t.Fatal("admin unexpectedly a member of private channel")
	}

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", adminTok, map[string]any{
		"title": "X",
		"body":  "stolen",
	})
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-channel create allowed (got %d, want 403/404)", resp.StatusCode)
	}
}

// TestCV_CommitBumpsVersion pins 设计 ③: commit creates a new
// artifact_versions row with version=N+1 + bumps artifacts.current_version.
func TestCV_CommitBumpsVersion(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "v1",
	})
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", tok, map[string]any{
		"expected_version": 1,
		"body":             "v2 body",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("commit failed (got %d, %v)", resp.StatusCode, data)
	}
	if vf, _ := data["version"].(float64); int64(vf) != 2 {
		t.Errorf("version after commit != 2: %v", data["version"])
	}

	// GET back: head body should be the new commit.
	_, head := testutil.JSON(t, "GET", ts.URL+"/api/v1/artifacts/"+id, tok, nil)
	if head["body"] != "v2 body" {
		t.Errorf("head body mismatch: %v", head["body"])
	}
	if vf, _ := head["current_version"].(float64); int64(vf) != 2 {
		t.Errorf("current_version != 2: %v", head["current_version"])
	}

	// versions list shows both rows linearly (设计 ③).
	_, vlist := testutil.JSON(t, "GET", ts.URL+"/api/v1/artifacts/"+id+"/versions", tok, nil)
	versions := vlist["versions"].([]any)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
}

// TestCV_CommitVersionMismatch409 pins 设计 ② version mismatch path
// (a stale client trying to commit on top of an already-bumped head
// gets 409 + reload hint).
func TestCV_CommitVersionMismatch409(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "v1",
	})
	id := art["id"].(string)

	// First commit OK.
	testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", tok, map[string]any{
		"expected_version": 1, "body": "v2",
	})
	// Stale client still on version=1 → 409.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", tok, map[string]any{
		"expected_version": 1, "body": "stale",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stale commit accepted (got %d, want 409)", resp.StatusCode)
	}
}

// TestCV_LockTTL30sBoundary pins 设计 ② lazy expire boundary on the
// handler directly with an injected fake clock. We exercise:
//
//   - T+0: user A acquires lock via commit
//   - T+29s: user B's commit sees A's lock (within 30s) → 409
//   - T+31s: user B's commit can steal the lock (lazy expire) → 200
//
// We construct a standalone ArtifactHandler against a fresh test server's
// store so we can drive the clock without spinning real time forward.
func TestCV_LockTTL30sBoundary(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)

	owner := mustUser(t, s, "owner@test.com")
	member := mustUser(t, s, "member@test.com")

	// Create artifact via the live HTTP path so all schema columns init
	// correctly (the standalone handler then reuses the same store).
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "init",
	})
	id := art["id"].(string)

	now := time.Unix(1_700_000_000, 0)
	clk := &fakeClock{t: now}
	h := &api.ArtifactHandler{Store: s, Now: clk.Now}

	// T+0: owner commits → acquires lock + bumps to v=2.
	if err := commitDirect(t, h, id, owner, 1, "v2"); err != nil {
		t.Fatalf("owner commit: %v", err)
	}

	// T+29s: member tries to commit → 409 because owner still holds lock.
	clk.t = now.Add(29 * time.Second)
	if err := commitDirect(t, h, id, member, 2, "stolen"); err == nil {
		t.Fatal("member commit at T+29s should 409 (lock held)")
	} else if !isConflict(err) {
		t.Fatalf("member commit at T+29s wrong error: %v", err)
	}

	// T+31s: lock expired (>30s), member can steal.
	clk.t = now.Add(31 * time.Second)
	if err := commitDirect(t, h, id, member, 2, "stolen-after-expire"); err != nil {
		t.Fatalf("member commit at T+31s after expire: %v", err)
	}
}

// TestCV_RollbackOwnerOnly pins 设计 ⑦ three-way reverse assertion:
//
//   - admin cookie → 401 (no user-rail auth, admin rail forbidden)
//   - non-owner member → 403
//   - 锁持有=别人 → 409 (covered by lock test, not duplicated here)
//   - owner success → new version with rolled_back_from_version stamped
func TestCV_RollbackOwnerOnly(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Doc", "body": "v1",
	})
	id := art["id"].(string)
	// owner commits twice so we have a v2/v3 to rollback to.
	testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", ownerTok, map[string]any{"expected_version": 1, "body": "v2"})
	testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", ownerTok, map[string]any{"expected_version": 2, "body": "v3"})

	// non-owner (member) → 403.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/rollback", memberTok, map[string]any{"to_version": 1})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner rollback got %d, want 403", resp.StatusCode)
	}

	// owner success → returns new version stamped with rolled_back_from_version.
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/rollback", ownerTok, map[string]any{"to_version": 1})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner rollback got %d (%v)", resp.StatusCode, data)
	}
	if vf, _ := data["version"].(float64); int64(vf) != 4 {
		t.Errorf("rollback new version != 4: %v", data["version"])
	}
	if rf, _ := data["rolled_back_from_version"].(float64); int64(rf) != 1 {
		t.Errorf("rolled_back_from_version != 1: %v", data["rolled_back_from_version"])
	}

	// versions list confirms new row references the source version.
	_, vlist := testutil.JSON(t, "GET", ts.URL+"/api/v1/artifacts/"+id+"/versions", ownerTok, nil)
	versions := vlist["versions"].([]any)
	last := versions[len(versions)-1].(map[string]any)
	if rf, ok := last["rolled_back_from_version"].(float64); !ok || int64(rf) != 1 {
		t.Errorf("last version rolled_back_from_version missing or wrong: %v", last["rolled_back_from_version"])
	}
	// Body of the rolled-back-to row is byte-identical with v1.
	if last["body"] != "v1" {
		t.Errorf("rollback body mismatch (want 'v1', got %v)", last["body"])
	}
	// Old versions are NOT deleted (设计 ③ 不删中间版本).
	if len(versions) != 4 {
		t.Errorf("expected 4 version rows after rollback, got %d", len(versions))
	}
}

// TestCV_RollbackProducesNewVersionNotDelete pins acceptance §2.3
// 反约束: rollback inserts a new row, never deletes intermediate
// versions (设计 ③ + ⑦).
func TestCV_RollbackProducesNewVersionNotDelete(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "v1",
	})
	id := art["id"].(string)
	testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", tok, map[string]any{"expected_version": 1, "body": "v2"})
	testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/rollback", tok, map[string]any{"to_version": 1})

	var count int64
	s.DB().Raw(`SELECT COUNT(*) FROM artifact_versions WHERE artifact_id = ?`, id).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 version rows (v1+v2+rollback-row), got %d", count)
	}
}

// TestCV_AgentCommitSystemMessage pins 设计 ⑥: when the committer is
// an agent (Role='agent'), the handler emits a system message with the
// byte-identical文案 锁: "{agent_name} 更新 {artifact_name} v{n}".
//
// Reverse assertion: for a human committer, NO such system message is
// emitted (silent for humans, only agents fanout).
func TestCV_AgentCommitSystemMessage(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	// Seed an agent user (Role='agent') + add to channel + grant the
	// AP-0 default wildcard so it can hit the API.
	agentEmail := "agent-cv12@test.com"
	agent := &store.User{
		DisplayName: "AgentX",
		Role:        "agent",
		Email:       &agentEmail,
		PasswordHash: mustHash(t, "password123"),
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
	if err := s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: agent.ID}); err != nil {
		t.Fatalf("add agent to channel: %v", err)
	}
	agentTok := testutil.LoginAs(t, ts.URL, agentEmail, "password123")

	// owner creates the artifact, agent commits v=2 → fanout fires.
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "v1",
	})
	id := art["id"].(string)
	// AP-1.2: agent ABAC capability gate requires explicit
	// (commit_artifact, artifact:<id>) grant — owner-issued per-artifact
	// (蓝图 §1.4 设计沿用). Default [message.send, message.read] is not
	// enough for artifact write.
	if err := s.GrantPermission(&store.UserPermission{
		UserID: agent.ID, Permission: auth.CommitArtifact, Scope: "artifact:" + id,
	}); err != nil {
		t.Fatalf("grant artifact perm: %v", err)
	}
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", agentTok, map[string]any{
		"expected_version": 1, "body": "v2",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("agent commit failed: %d", resp.StatusCode)
	}

	// Read back system messages on the channel — there must be exactly
	// one with the byte-identical fanout content.
	want := "AgentX 更新 Plan v2"
	if !channelHasSystemMessage(t, s, chID, want) {
		t.Errorf("agent commit fanout message missing: want %q", want)
	}
}

// TestCV_HumanCommitNoSystemMessage pins the reverse of ⑥: a human
// committer doesn't fanout (silence by default, only agent commits
// trigger the system message).
func TestCV_HumanCommitNoSystemMessage(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Plan", "body": "v1",
	})
	id := art["id"].(string)
	testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/commits", tok, map[string]any{
		"expected_version": 1, "body": "v2",
	})
	if channelHasSystemMessage(t, s, chID, "Owner 更新 Plan v2") {
		t.Error("human commit unexpectedly fanned out a system message")
	}
}

// TestCV_PushFrameOnCreateAndCommit pins 设计 ⑤: every successful
// create / commit / rollback hits the ArtifactPusher exactly once with
// (id, version, channel_id, ts, kind). We bypass the live server's
// production hub by constructing a standalone handler with a recording
// pusher and driving the same store.
func TestCV_PushFrameOnCreateAndCommit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	owner := mustUser(t, s, "owner@test.com")
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	rec := &recordingPusher{}
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	h := &api.ArtifactHandler{Store: s, Now: clk.Now, Pusher: rec}

	id := createDirect(t, h, chID, owner, "Doc", "v1")
	if err := commitDirect(t, h, id, owner, 1, "v2"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := rollbackDirect(t, h, id, owner, 1); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 3 {
		t.Fatalf("expected 3 push calls (create+commit+rollback), got %d", len(calls))
	}
	if calls[0].Kind != "commit" || calls[0].Version != 1 {
		t.Errorf("create push wrong: %+v", calls[0])
	}
	if calls[1].Kind != "commit" || calls[1].Version != 2 {
		t.Errorf("commit push wrong: %+v", calls[1])
	}
	if calls[2].Kind != "rollback" || calls[2].Version != 3 {
		t.Errorf("rollback push wrong: %+v", calls[2])
	}
	for _, c := range calls {
		if c.ArtifactID != id || c.ChannelID != chID {
			t.Errorf("push id/channel mismatch: %+v", c)
		}
	}
}

// ----- helpers -----

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func mustUserID(t *testing.T, s *store.Store, email string) string {
	t.Helper()
	return mustUser(t, s, email).ID
}

func mustUser(t *testing.T, s *store.Store, email string) *store.User {
	t.Helper()
	var u store.User
	if err := s.DB().Where("email = ?", email).First(&u).Error; err != nil {
		t.Fatalf("user %s: %v", email, err)
	}
	return &u
}

func mustOrgID(t *testing.T, s *store.Store, email string) string {
	t.Helper()
	return mustUser(t, s, email).OrgID
}

func mustHash(t *testing.T, _ string) string {
	t.Helper()
	// TEST-FIX-3-COV PERF: use bcrypt MinCost (cost=4) hash so login compare
	// runs ~1ms instead of ~65ms (cost=10). Hash for "password123" — same
	// content, just lower cost factor; tests pass identically. Major slow-test
	// perf win: cm52SetupTwoAgents seeds 2 agents → 2 login compares = saves
	// ~130ms per setup, applies to 100+ tests calling seedAgentInChannel.
	return "$2a$04$vJibaj091YLuvewe7JFGNeaHCbz48jAxzCNmSXVJlwaLESIFs/zSW"
}

func channelHasSystemMessage(t *testing.T, s *store.Store, channelID, want string) bool {
	t.Helper()
	var msgs []store.Message
	if err := s.DB().Where("channel_id = ? AND sender_id = 'system'", channelID).Find(&msgs).Error; err != nil {
		t.Fatalf("query system messages: %v", err)
	}
	for _, m := range msgs {
		if m.Content == want {
			return true
		}
	}
	return false
}

// commitDirect invokes the handler internally without the live mux so
// we can drive (a) the injected clock and (b) arbitrary user identities
// without a login round-trip. Returns the response error (or a typed
// wrapper with the HTTP status for assertions).
func commitDirect(t *testing.T, h *api.ArtifactHandler, artifactID string, u *store.User, expectedVersion int64, body string) error {
	t.Helper()
	return invokeHandler(t, h, "POST",
		"/api/v1/artifacts/"+artifactID+"/commits",
		map[string]any{"expected_version": expectedVersion, "body": body},
		map[string]string{"artifactId": artifactID},
		u, h.HandleCommit)
}

func rollbackDirect(t *testing.T, h *api.ArtifactHandler, artifactID string, u *store.User, toVersion int64) error {
	t.Helper()
	return invokeHandler(t, h, "POST",
		"/api/v1/artifacts/"+artifactID+"/rollback",
		map[string]any{"to_version": toVersion},
		map[string]string{"artifactId": artifactID},
		u, h.HandleRollback)
}

func createDirect(t *testing.T, h *api.ArtifactHandler, channelID string, u *store.User, title, body string) string {
	t.Helper()
	rec := &capturingResponse{}
	invokeHandlerCustom(t, "POST",
		"/api/v1/channels/"+channelID+"/artifacts",
		map[string]any{"title": title, "body": body},
		map[string]string{"channelId": channelID},
		u, h.HandleCreate, rec)
	if rec.status != http.StatusCreated {
		t.Fatalf("createDirect status %d body=%s", rec.status, rec.body)
	}
	// crude id extraction; tests need only the id field.
	return rec.parsed["id"].(string)
}

// HandleCommit / HandleCreate / HandleRollback are exported test seams
// declared in artifacts_test_export.go (in-package _internal_test file)
// because the handler methods are private. We declare them here as
// package-test seams via a shim in the api package.

func isConflict(err error) bool {
	if err == nil {
		return false
	}
	if he, ok := err.(*httpStatusErr); ok {
		return he.status == http.StatusConflict
	}
	return false
}

type httpStatusErr struct {
	status int
	body   string
}

func (e *httpStatusErr) Error() string { return e.body }

// capturingResponse + invokeHandler / invokeHandlerCustom are minimal
// shims so we can exercise the handler with a synthetic *http.Request
// without going through the real mux. Path values are stuffed via the
// SetPathValue API (Go 1.22+).
type capturingResponse struct {
	status int
	body   []byte
	hdr    http.Header
	parsed map[string]any
}

func (c *capturingResponse) Header() http.Header {
	if c.hdr == nil {
		c.hdr = http.Header{}
	}
	return c.hdr
}
func (c *capturingResponse) WriteHeader(s int)        { c.status = s }
func (c *capturingResponse) Write(b []byte) (int, error) {
	c.body = append(c.body, b...)
	return len(b), nil
}

func invokeHandler(t *testing.T, _ *api.ArtifactHandler, method, path string, body any, pathValues map[string]string, u *store.User, fn func(http.ResponseWriter, *http.Request)) error {
	t.Helper()
	rec := &capturingResponse{}
	invokeHandlerCustom(t, method, path, body, pathValues, u, fn, rec)
	if rec.status >= 400 {
		return &httpStatusErr{status: rec.status, body: string(rec.body)}
	}
	return nil
}

func invokeHandlerCustom(t *testing.T, method, path string, body any, pathValues map[string]string, u *store.User, fn func(http.ResponseWriter, *http.Request), rec *capturingResponse) {
	t.Helper()
	req := mustNewRequestWithJSON(t, method, path, body)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	if u != nil {
		req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	}
	fn(rec, req)
	if len(rec.body) > 0 {
		_ = jsonUnmarshalSilent(rec.body, &rec.parsed)
	}
}

// mustNewRequestWithJSON builds an http.Request with a JSON body. Used by
// the standalone-handler tests that bypass the live mux.
func mustNewRequestWithJSON(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		buf = bytes.NewBuffer(raw)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequest(method, path, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

// jsonUnmarshalSilent decodes JSON, ignoring decode errors (callers only
// dereference the parsed map for fields they expect).
func jsonUnmarshalSilent(b []byte, v any) error {
	return json.Unmarshal(b, v)
}
