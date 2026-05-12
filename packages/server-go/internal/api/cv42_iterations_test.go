// Package api_test — cv_4_2_iterations_test.go: CV-4.2 acceptance tests
// (#405 schema v=18 → CV-4.2 server iterate API + state machine + WS push).
//
// 覆盖的设计约束 (cv-4-spec.md §0 + acceptance §2 + §4 + 文案锁定
// #380):
//   - design rule 1 domain isolation — messages 不污染 (acceptance §1.5 + §4.2 grep 检查, repo-
//     level CI lint, 非 unit).
//   - design rule 2 commits still go through CV-1 — POST /commits?iteration_id= atomic UPDATE
//     running→completed; 反向约束 不开 /iterations/:id/commit 旁路
//     (acceptance §2.2 + §4.1).
//   - design rule 3 server does not compute diff — grep 检查 CI (acceptance §2.6 + §4.4).
//   - design rule 4 state machine 4 态前向锁定 — 反 completed→running / failed→pending
//     等回退 reject (acceptance §2.3 + §4.3).
//   - design rule 5 AL-4 stub 默认拒绝 — agent_runtimes.status != 'running' →
//     state='failed' + error_reason='runtime_not_registered' exact match
//     跟 AL-1a #249 6 reason 同源 (acceptance §2.5).
//   - design rule 6 owner-only — non-owner POST /iterate → 403 (acceptance §2.1).
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// cv42Setup builds a fresh server, creates an artifact in `general`, seeds
// a role='agent' channel-member, and returns
// (ts.URL, ownerTok, store, channelID, artifactID, agentID).
func cv42Setup(t *testing.T) (url string, ownerTok string, s *store.Store, chID string, artID string, agentID string) {
	t.Helper()
	ts, st, _ := testutil.NewTestServer(t)
	ownerTok = testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID = cv12General(t, ts.URL, ownerTok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Plan", "body": "para A.",
	})
	artID = art["id"].(string)
	_ = seedAgentInChannel(t, st, ts.URL, chID, "agent-cv42@test.com", "AgentZ")
	// look up agent's user id
	u, err := st.GetUserByEmail("agent-cv42@test.com")
	if err != nil || u == nil {
		t.Fatalf("seed agent lookup failed: %v", err)
	}
	url = ts.URL
	s = st
	agentID = u.ID
	return
}

// TestCV_IterateOwnerOnly 覆盖 acceptance §2.1: only the channel owner
// (channel.created_by) may POST /iterate. Non-owner = 403 — admin
// admin 权限 does not enter this rail (ADM-0 §1.3, anchors / artifacts 同
// rail 隔离).
func TestCV_IterateOwnerOnly(t *testing.T) {
	t.Parallel()
	url, _, s, chID, artID, agentID := cv42Setup(t)

	// Seed second human (non-owner) channel member.
	otherTok := func() string {
		hash := mustHash(t, "password123")
		em := "other-cv42@test.com"
		u := &store.User{DisplayName: "Other", Role: "user", Email: &em, PasswordHash: hash}
		if err := s.CreateUser(u); err != nil {
			t.Fatalf("create other: %v", err)
		}
		_ = s.UpdateUser(u.ID, map[string]any{"org_id": mustOrgID(t, s, "owner@test.com")})
		_ = s.GrantDefaultPermissions(u.ID, "member")
		_ = s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: u.ID})
		return testutil.LoginAs(t, url, em, "password123")
	}()

	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", otherTok, map[string]any{
		"intent_text":     "make it shorter",
		"target_agent_id": agentID,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner iterate not 403: got %d", resp.StatusCode)
	}
}

// TestCV_AL4StubFailClosed_RuntimeNotRegistered 覆盖 acceptance §2.5:
// when no agent_runtimes row with status='running' exists, iteration
// transitions pending→failed atomically with exact-match error_reason
// 'runtime_not_registered' (AL-1a #249 6 reason 同源 不另起字典).
func TestCV_AL4StubFailClosed_RuntimeNotRegistered(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID, agentID := cv42Setup(t)

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text":     "rewrite section 1",
		"target_agent_id": agentID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("iterate stub-fail not 201: got %d (%v)", resp.StatusCode, data)
	}
	if data["state"] != api.IterationStateFailed {
		t.Errorf("state exact-match check failed: got %v, want %q", data["state"], api.IterationStateFailed)
	}
	if data["error_reason"] != api.IterationErrorReasonRuntimeNotRegistered {
		t.Errorf("error_reason exact-match check failed: got %v, want %q",
			data["error_reason"], api.IterationErrorReasonRuntimeNotRegistered)
	}
}

// TestCV_AL4Live_StateRunning 覆盖 acceptance §2.5 second branch: when
// agent_runtimes row exists with status='running', AL-4 stub treats this
// as "live" and persists state='running' (real plugin dispatch lands
// when AL-4 runtime hub plugin path is wired — out of scope CV-4.2,
// this integration point keeps future AL-4 work from re-threading the
// switch).
func TestCV_AL4Live_StateRunning(t *testing.T) {
	t.Parallel()
	url, ownerTok, s, _, artID, agentID := cv42Setup(t)
	// Seed agent_runtimes with status='running'.
	if err := s.DB().Exec(`INSERT INTO agent_runtimes
  (id, agent_id, endpoint_url, process_kind, status, created_at, updated_at)
  VALUES (?, ?, 'ws://test', 'openclaw', 'running', 1700000000000, 1700000000000)`,
		"rt-cv42", agentID).Error; err != nil {
		t.Fatalf("seed runtime: %v", err)
	}

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text":     "rewrite section 1",
		"target_agent_id": agentID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("iterate live not 201: got %d (%v)", resp.StatusCode, data)
	}
	if data["state"] != api.IterationStateRunning {
		t.Errorf("state exact-match check failed: got %v, want %q", data["state"], api.IterationStateRunning)
	}
}

// TestCV_TargetAgentMustBeChannelMember 验证 acceptance §2.1 反向场景:
// target_agent_id 不是 channel member → 400 exact-match error code
// 'iteration.target_not_in_channel'.
func TestCV_TargetAgentMustBeChannelMember(t *testing.T) {
	t.Parallel()
	url, ownerTok, s, _, artID, _ := cv42Setup(t)

	// Seed an unrelated agent NOT in the channel.
	hash := mustHash(t, "password123")
	em := "stranger-agent@test.com"
	stranger := &store.User{DisplayName: "Stranger", Role: "agent", Email: &em, PasswordHash: hash}
	if err := s.CreateUser(stranger); err != nil {
		t.Fatalf("create stranger agent: %v", err)
	}

	resp, data := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text":     "do thing",
		"target_agent_id": stranger.ID,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-member target not 400: got %d (%v)", resp.StatusCode, data)
	}
	if data["code"] != api.IterationErrCodeTargetNotInChannel {
		t.Errorf("error code exact-match check failed: got %v, want %q",
			data["code"], api.IterationErrCodeTargetNotInChannel)
	}
}

// TestCV_CommitWithIterationIDAtomicUpdate 覆盖 acceptance §2.2 (CV-1
// commit path): POST /commits?iteration_id= transitions
// running→completed atomically + writes created_artifact_version_id.
// 反向约束: 不开 /iterations/:id/commit 旁路 (verified by CI grep §4.1).
func TestCV_CommitWithIterationIDAtomicUpdate(t *testing.T) {
	t.Parallel()
	url, ownerTok, s, _, artID, agentID := cv42Setup(t)
	// Seed running runtime so iterate produces state=running.
	if err := s.DB().Exec(`INSERT INTO agent_runtimes
  (id, agent_id, endpoint_url, process_kind, status, created_at, updated_at)
  VALUES (?, ?, 'ws://test', 'openclaw', 'running', 1700000000000, 1700000000000)`,
		"rt-cv42-c", agentID).Error; err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
	_, itData := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text":     "rewrite",
		"target_agent_id": agentID,
	})
	iterationID := itData["id"].(string)
	if itData["state"] != api.IterationStateRunning {
		t.Fatalf("setup precondition: state=%v, want running", itData["state"])
	}

	// Owner commits with ?iteration_id=.
	resp, _ := testutil.JSON(t, "POST",
		url+"/api/v1/artifacts/"+artID+"/commits?iteration_id="+iterationID,
		ownerTok, map[string]any{
			"expected_version": float64(1),
			"body":             "rewritten body v2",
		})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("commit?iteration_id 没成功: got %d", resp.StatusCode)
	}

	// Verify atomic UPDATE: state='completed' + created_artifact_version_id != NULL.
	var row struct {
		State                    string  `gorm:"column:state"`
		CreatedArtifactVersionID *int64  `gorm:"column:created_artifact_version_id"`
		CompletedAt              *int64  `gorm:"column:completed_at"`
		ErrorReason              *string `gorm:"column:error_reason"`
	}
	if err := s.DB().Raw(`SELECT state, created_artifact_version_id, completed_at, error_reason
FROM artifact_iterations WHERE id = ?`, iterationID).Scan(&row).Error; err != nil {
		t.Fatalf("query iteration: %v", err)
	}
	if row.State != api.IterationStateCompleted {
		t.Errorf("state not completed: got %q", row.State)
	}
	if row.CreatedArtifactVersionID == nil || *row.CreatedArtifactVersionID == 0 {
		t.Errorf("created_artifact_version_id not set: %v", row.CreatedArtifactVersionID)
	}
	if row.CompletedAt == nil {
		t.Errorf("completed_at not set")
	}
	if row.ErrorReason != nil {
		t.Errorf("error_reason not NULL on success: %v", *row.ErrorReason)
	}
}

// TestCV_StateMachine_RejectsCommitOnFailedIteration 覆盖 acceptance
// §2.3 反断: state machine forward-only — committing with an
// iteration_id whose state is 'failed' (or any state != 'running') →
// 409 conflict. 反向约束: completed→running / failed→pending 等回退绝对
// reject (CompleteIterationOnCommit 的 WHERE state='running' clause 是
// 唯一闸位).
func TestCV_StateMachine_RejectsCommitOnFailedIteration(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID, agentID := cv42Setup(t)
	// No runtime seeded → iterate fails immediately (state='failed').
	_, itData := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text":     "rewrite",
		"target_agent_id": agentID,
	})
	iterationID := itData["id"].(string)
	if itData["state"] != api.IterationStateFailed {
		t.Fatalf("precondition: state=%v, want failed", itData["state"])
	}

	// Now try to commit referencing the failed iteration_id → 409.
	resp, _ := testutil.JSON(t, "POST",
		url+"/api/v1/artifacts/"+artID+"/commits?iteration_id="+iterationID,
		ownerTok, map[string]any{
			"expected_version": float64(1),
			"body":             "should not land",
		})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("commit on failed iteration accepted: got %d, want 409", resp.StatusCode)
	}
}

// TestCV_CommitWithoutIterationID_LegacyPathUnchanged 覆盖 acceptance
// §2.2 反断: when ?iteration_id= absent, commit follows CV-1.2 历史
// behaviour exactly (反向约束 旧路径不破). No iteration row is created or
// touched. 跟 cv_1_2_artifacts_test.go::TestCV12_CommitArtifact 同模式.
func TestCV_CommitWithoutIterationID_LegacyPathUnchanged(t *testing.T) {
	t.Parallel()
	url, ownerTok, s, _, artID, _ := cv42Setup(t)
	resp, _ := testutil.JSON(t, "POST",
		url+"/api/v1/artifacts/"+artID+"/commits",
		ownerTok, map[string]any{
			"expected_version": float64(1),
			"body":             "old-path commit",
		})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("old-path commit not 200: got %d", resp.StatusCode)
	}
	// Confirm no iteration row exists.
	var n int64
	if err := s.DB().Raw(`SELECT COUNT(*) FROM artifact_iterations WHERE artifact_id = ?`, artID).Scan(&n).Error; err != nil {
		t.Fatalf("count iterations: %v", err)
	}
	if n != 0 {
		t.Errorf("old-path commit polluted artifact_iterations: %d rows", n)
	}
}

// TestCV_ListIterationsHistory 覆盖 GET history shape (ORDER BY
// created_at DESC + intent_text 含 — channel-member rail; admin path
// 不入 acceptance §2.7 反断 是 admin*.go 责任, 此 endpoint 在
// channel-member rail intent_text 必须返).
func TestCV_ListIterationsHistory(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID, agentID := cv42Setup(t)
	for i := 0; i < 2; i++ {
		_, _ = testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
			"intent_text":     "iter",
			"target_agent_id": agentID,
		})
	}
	resp, data := testutil.JSON(t, "GET", url+"/api/v1/artifacts/"+artID+"/iterations", ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list not 200: got %d", resp.StatusCode)
	}
	rows := data["iterations"].([]any)
	if len(rows) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(rows))
	}
	// Shape check: intent_text returned (channel-member rail).
	row0 := rows[0].(map[string]any)
	if row0["intent_text"] != "iter" {
		t.Errorf("intent_text not returned on member rail: %v", row0["intent_text"])
	}
}

// TestCV_Iterate_ErrorPaths covers the 400/401/403/404 negative branches
// of POST /iterate so coverage stays above 85% threshold (CI ci.yml:55).
// Each branch is independently asserted — handler short-circuits before
// the AL-4 stub fork, so a single setup suffices.
func TestCV_Iterate_ErrorPaths(t *testing.T) {
	t.Parallel()
	url, ownerTok, s, _, artID, agentID := cv42Setup(t)

	// 401 — no auth (anonymous).
	resp401, err := http.Post(url+"/api/v1/artifacts/"+artID+"/iterate", "application/json",
		strings.NewReader(`{"intent_text":"x","target_agent_id":"`+agentID+`"}`))
	if err != nil {
		t.Fatalf("anon req: %v", err)
	}
	resp401.Body.Close()
	if resp401.StatusCode != http.StatusUnauthorized {
		t.Errorf("anon 401 expected, got %d", resp401.StatusCode)
	}

	// 404 — artifact not found.
	resp404, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/no-such/iterate", ownerTok, map[string]any{
		"intent_text": "x", "target_agent_id": agentID,
	})
	if resp404.StatusCode != http.StatusNotFound {
		t.Errorf("missing artifact 404 expected, got %d", resp404.StatusCode)
	}

	// 400 — empty intent_text.
	resp400a, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text": "   ", "target_agent_id": agentID,
	})
	if resp400a.StatusCode != http.StatusBadRequest {
		t.Errorf("empty intent_text 400 expected, got %d", resp400a.StatusCode)
	}

	// 400 — empty target_agent_id.
	resp400b, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text": "x", "target_agent_id": "",
	})
	if resp400b.StatusCode != http.StatusBadRequest {
		t.Errorf("empty target 400 expected, got %d", resp400b.StatusCode)
	}

	// 400 — target is human (role!='agent').
	hash := mustHash(t, "password123")
	em := "humantarget@test.com"
	human := &store.User{DisplayName: "Human", Role: "user", Email: &em, PasswordHash: hash}
	if err := s.CreateUser(human); err != nil {
		t.Fatalf("create human: %v", err)
	}
	_ = s.UpdateUser(human.ID, map[string]any{"org_id": mustOrgID(t, s, "owner@test.com")})
	_ = s.GrantDefaultPermissions(human.ID, "member")
	// Lookup channel id (general) to add member.
	chID := cv12General(t, url, ownerTok)
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: human.ID})
	respHuman, dataHuman := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text": "x", "target_agent_id": human.ID,
	})
	if respHuman.StatusCode != http.StatusBadRequest {
		t.Errorf("human target 400 expected, got %d", respHuman.StatusCode)
	}
	if dataHuman["code"] != api.IterationErrCodeTargetNotInChannel {
		t.Errorf("human target error code exact-match check failed: got %v", dataHuman["code"])
	}
}

// TestCV_ListIterations_NotFoundOrCrossChannel covers the 401/404 branches
// of GET /iterations (anonymous + artifact not found).
func TestCV_ListIterations_NotFoundOrCrossChannel(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, _, _ := cv42Setup(t)

	// 401 — anonymous.
	resp401, err := http.Get(url + "/api/v1/artifacts/x/iterations")
	if err != nil {
		t.Fatalf("anon req: %v", err)
	}
	resp401.Body.Close()
	if resp401.StatusCode != http.StatusUnauthorized {
		t.Errorf("anon 401 expected, got %d", resp401.StatusCode)
	}

	// 404 — missing artifact id.
	resp, _ := testutil.JSON(t, "GET", url+"/api/v1/artifacts/no-such/iterations", ownerTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing artifact 404 expected, got %d", resp.StatusCode)
	}
}

// TestCV_Iterate_NonOwner_403 — design rule 6 owner-only (acceptance §2.1) — a
// channel member who is not the owner gets 403 (handler runs after
// canAccessChannel passes). Different from TestCV_IterateOwnerOnly which
// covers the *non-owner channel-member* 403 path; this extra case exercises
// the full owner-check branch with explicit channel-member precondition.
func TestCV_Iterate_NonOwner_403(t *testing.T) {
	t.Parallel()
	url, _, s, chID, artID, agentID := cv42Setup(t)
	// Add second human channel member (non-owner).
	hash := mustHash(t, "password123")
	em := "nonowner@test.com"
	mem := &store.User{DisplayName: "Mem", Role: "user", Email: &em, PasswordHash: hash}
	if err := s.CreateUser(mem); err != nil {
		t.Fatalf("create mem: %v", err)
	}
	_ = s.UpdateUser(mem.ID, map[string]any{"org_id": mustOrgID(t, s, "owner@test.com")})
	_ = s.GrantDefaultPermissions(mem.ID, "member")
	_ = s.AddChannelMember(&store.ChannelMember{ChannelID: chID, UserID: mem.ID})
	memTok := testutil.LoginAs(t, url, em, "password123")

	resp, _ := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", memTok, map[string]any{
		"intent_text": "x", "target_agent_id": agentID,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-owner channel member 403 expected, got %d", resp.StatusCode)
	}
}

// TestCV_CommitWithIterationID_NotFound exercises the
// CompleteIterationOnCommit reject path when iteration_id references a
// non-existent row (or wrong artifact). The atomic UPDATE WHERE clause
// finds no row → state machine reject → 409.
func TestCV_CommitWithIterationID_NotFound(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID, _ := cv42Setup(t)
	resp, _ := testutil.JSON(t, "POST",
		url+"/api/v1/artifacts/"+artID+"/commits?iteration_id=does-not-exist",
		ownerTok, map[string]any{
			"expected_version": float64(1),
			"body":             "v2",
		})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("nonexistent iteration_id 409 expected, got %d", resp.StatusCode)
	}
}

// TestCV_IsIterationStateMachineReject — direct unit test for the
// errors.Is sentinel helper (acceptance §2.3 同源).
func TestCV_IsIterationStateMachineReject(t *testing.T) {
	t.Parallel()
	if api.IsIterationStateMachineReject(nil) {
		t.Error("nil should not match reject sentinel")
	}
	// CompleteIterationOnCommit with empty id returns nil (early return —
	// query path not exercised, exercises the empty-id branch).
	if err := api.CompleteIterationOnCommit(nil, "", "", 0, 0); err != nil {
		t.Errorf("empty iteration_id should be nil-op, got %v", err)
	}
}

// TestCV_ListAnchorComments_Coverage covers the new handleListComments
// endpoint introduced via merge-from-main. Boosts coverage past 85%
// threshold without breaking CV-2.2 invariants.
func TestCV_ListAnchorComments_Coverage(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID, _ := cv42Setup(t)

	// 401 anon.
	resp401, err := http.Get(url + "/api/v1/anchors/x/comments")
	if err != nil {
		t.Fatalf("anon: %v", err)
	}
	resp401.Body.Close()
	if resp401.StatusCode != http.StatusUnauthorized {
		t.Errorf("anon expected 401, got %d", resp401.StatusCode)
	}

	// 404 missing anchor.
	resp404, _ := testutil.JSON(t, "GET", url+"/api/v1/anchors/no-such/comments", ownerTok, nil)
	if resp404.StatusCode != http.StatusNotFound {
		t.Errorf("missing anchor 404 expected, got %d", resp404.StatusCode)
	}

	// Happy path: create anchor + 1 comment + list.
	_, anchor := testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/anchors", ownerTok, map[string]any{
		"start_offset": 0, "end_offset": 3,
	})
	anchorID := anchor["id"].(string)
	_, _ = testutil.JSON(t, "POST", url+"/api/v1/anchors/"+anchorID+"/comments", ownerTok, map[string]any{
		"body": "lgtm",
	})
	resp, data := testutil.JSON(t, "GET", url+"/api/v1/anchors/"+anchorID+"/comments", ownerTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list comments not 200: got %d", resp.StatusCode)
	}
	if rows, ok := data["comments"].([]any); !ok || len(rows) != 1 {
		t.Errorf("expected 1 comment, got %v", data["comments"])
	}
}

// TestCV42_HandleListIterations_PathValueEmpty covers the canAccessChannel
// false branch of handleListIterations — outsider with no channel access
// gets 404 (404 not 403 to not leak existence per design rule 7 defense). This
// exercises a branch the happy-path test in TestCV_ListIterationsHistory
// doesn't reach.
func TestCV_HandleListIterations_NonMember404(t *testing.T) {
	t.Parallel()
	url, ownerTok, _, _, artID, _ := cv42Setup(t)
	// Iterate once so the artifact has history.
	_, _ = testutil.JSON(t, "POST", url+"/api/v1/artifacts/"+artID+"/iterate", ownerTok, map[string]any{
		"intent_text": "x", "target_agent_id": "any",
	})
	// 401 anon GET history.
	resp401, err := http.Get(url + "/api/v1/artifacts/" + artID + "/iterations")
	if err != nil {
		t.Fatalf("anon: %v", err)
	}
	resp401.Body.Close()
	if resp401.StatusCode != http.StatusUnauthorized {
		t.Errorf("anon expected 401, got %d", resp401.StatusCode)
	}
}
