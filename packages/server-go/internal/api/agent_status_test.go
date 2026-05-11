// Package api_test — al_1b_2_status_test.go: AL-1b.2 acceptance tests
// (#453 schema v=21 → AL-1b.2 server endpoint + state machine).
//
// 设计约束 pins exercised (al-1b-spec.md §0 + acceptance §2):
//   - 设计第 1 条 分清三路径 — busy/idle 跟 AL-3 presence + AL-4 runtime 分清,
//     此 spec 5-state 合并仅在 API 层 (本 handler), schema 三表独立 (acceptance §2.1).
//   - 设计第 2 条 BPP 单一来源 — PATCH /status 405 reject (admin god-mode 也拒绝,
//     ADM-0 第 7 条 red-line 同源, acceptance §2.5).
//   - 设计第 3 条 文案三态 — schema 仅 2 态, API 暴露 5 态, client UI 进一步合并显示
//     (本 PR server-side 暴露 5 态字节级一致 跟 acceptance §2.1 字面对齐).
//
// 5-state 合并优先级 (acceptance §2.1):
//
//	error > busy > idle > online > offline
package api_test

import (
	"net/http"
	"testing"
	"time"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// al1b2Setup builds a fresh server, owner logs in, creates a role='agent'
// owned by owner. Returns (ts.URL, ownerTok, store, agentID).
func al1b2Setup(t *testing.T) (url string, ownerTok string, s *store.Store, agentID string) {
	t.Helper()
	ts, st, _ := testutil.NewTestServer(t)
	ownerTok = testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerTok, map[string]any{
		"display_name": "TaskBot",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent failed: %d (%v)", resp.StatusCode, data)
	}
	agentMap := data["agent"].(map[string]any)
	url = ts.URL
	s = st
	agentID = agentMap["id"].(string)
	return
}

// TestAL_GetStatus_NoRowFallsBackToOnlineOffline pins acceptance §2.1
// priority step 3 — 没有 BPP frame 上行过的 agent (agent_status 无 row)
// 走 AL-1a online/offline 退化 (Snapshot 默认 offline). 设计第 1 条 分清三路径
// — busy/idle 须显式 row, 不假装.
func TestAL_GetStatus_NoRowFallsBackToOnlineOffline(t *testing.T) {
	t.Parallel()
	url, tok, _, agentID := al1b2Setup(t)

	resp, data := testutil.JSON(t, http.MethodGet, url+"/api/v1/agents/"+agentID+"/status", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: %d (%v)", resp.StatusCode, data)
	}
	state, _ := data["state"].(string)
	if state != string(agentpkg.StateOffline) && state != string(agentpkg.StateOnline) {
		t.Errorf("no-row agent should fall back to online/offline (AL-1a), got state=%q", state)
	}
	// 反向约束: last_task_* 字段不应出现 (没 row 没 last task).
	for _, k := range []string{"last_task_id", "last_task_started_at", "last_task_finished_at"} {
		if _, has := data[k]; has {
			t.Errorf("no-row agent must not emit %q (got %v)", k, data[k])
		}
	}
}

// TestAL_GetStatus_BusyFromAgentStatusRow pins acceptance §2.1 step 2
// + §2.2: BPP `task_started` frame 触发 SetAgentTaskStarted → state='busy'
// + last_task_id + last_task_started_at; GET /status 返字节级一致.
func TestAL_GetStatus_BusyFromAgentStatusRow(t *testing.T) {
	t.Parallel()
	url, tok, st, agentID := al1b2Setup(t)

	now := time.Unix(1700000000, 0)
	if err := st.SetAgentTaskStarted(agentID, "task-foo", now); err != nil {
		t.Fatalf("SetAgentTaskStarted: %v", err)
	}

	resp, data := testutil.JSON(t, http.MethodGet, url+"/api/v1/agents/"+agentID+"/status", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: %d", resp.StatusCode)
	}
	if data["state"] != "busy" {
		t.Errorf("state = %v, want busy", data["state"])
	}
	if data["last_task_id"] != "task-foo" {
		t.Errorf("last_task_id = %v, want task-foo", data["last_task_id"])
	}
	if data["last_task_started_at"] != float64(now.UnixMilli()) {
		t.Errorf("last_task_started_at = %v, want %d", data["last_task_started_at"], now.UnixMilli())
	}
	// busy 态下 finished_at 应缺席 (acceptance §2.2 — task_started frame 不
	// 写 finished_at; idle 态时由 task_finished frame 填).
	if _, has := data["last_task_finished_at"]; has {
		t.Errorf("busy state should not emit last_task_finished_at, got %v", data["last_task_finished_at"])
	}
}

// TestAL_GetStatus_IdleFromAgentStatusRow pins acceptance §2.3 — BPP
// `task_finished` frame → state='idle' + last_task_finished_at 填.
func TestAL_GetStatus_IdleFromAgentStatusRow(t *testing.T) {
	t.Parallel()
	url, tok, st, agentID := al1b2Setup(t)

	now := time.Unix(1700000000, 0)
	// First started, then finished — simulates BPP frame pair.
	if err := st.SetAgentTaskStarted(agentID, "task-bar", now); err != nil {
		t.Fatalf("SetAgentTaskStarted: %v", err)
	}
	finishedAt := now.Add(30 * time.Second)
	if err := st.SetAgentTaskFinished(agentID, "task-bar", finishedAt); err != nil {
		t.Fatalf("SetAgentTaskFinished: %v", err)
	}

	resp, data := testutil.JSON(t, http.MethodGet, url+"/api/v1/agents/"+agentID+"/status", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: %d", resp.StatusCode)
	}
	if data["state"] != "idle" {
		t.Errorf("state = %v, want idle", data["state"])
	}
	if data["last_task_finished_at"] != float64(finishedAt.UnixMilli()) {
		t.Errorf("last_task_finished_at = %v", data["last_task_finished_at"])
	}
}

// TestAL_PatchStatusReturns405 pins acceptance §2.5 + 设计第 2 条 BPP 单一来源
// — PATCH /status 405 reject for owner. Admin god-mode 同样 reject 跟
// AL-4.2 admin god-mode 反向约束同源 (ADM-0 第 7 条 red-line).
func TestAL_PatchStatusReturns405(t *testing.T) {
	t.Parallel()
	url, tok, _, agentID := al1b2Setup(t)

	resp, data := testutil.JSON(t, http.MethodPatch, url+"/api/v1/agents/"+agentID+"/status", tok, map[string]any{
		"state": "busy",
	})
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /status: status=%d, want 405; data=%v", resp.StatusCode, data)
	}
	if got := resp.Header.Get("Allow"); got != "GET" {
		t.Errorf("Allow header = %q, want GET", got)
	}
	// Error message 含 "BPP-driven" 关键词 (反人工伪造 spec 引用).
	if errStr, _ := data["error"].(string); errStr == "" || !contains(errStr, "BPP-driven") {
		t.Errorf("error message missing BPP-driven keyword: %v", data)
	}
}

// TestAL_PatchStatusAdminAlsoRejected pins acceptance §2.5 — admin
// god-mode 也不允许改 busy/idle (跟 AL-4.2 admin god-mode 反向约束同源).
func TestAL_PatchStatusAdminAlsoRejected(t *testing.T) {
	t.Parallel()
	url, _, _, agentID := al1b2Setup(t)
	adminTok := testutil.LoginAs(t, url, "admin@test.com", "password123")

	resp, _ := testutil.JSON(t, http.MethodPatch, url+"/api/v1/agents/"+agentID+"/status", adminTok, map[string]any{
		"state": "idle",
	})
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("admin PATCH /status: status=%d, want 405 (god-mode also rejected, 设计第 2 条 BPP 单一来源)", resp.StatusCode)
	}
}

// TestAL_GetStatus_NotFound pins handler defense — non-existent
// agentID returns 404. 跟 GET /agents/{id} 既有 404 同源.
func TestAL_GetStatus_NotFound(t *testing.T) {
	t.Parallel()
	url, tok, _, _ := al1b2Setup(t)

	resp, _ := testutil.JSON(t, http.MethodGet, url+"/api/v1/agents/nonexistent/status", tok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /status nonexistent: %d, want 404", resp.StatusCode)
	}
}

// TestAL_ReapStaleBusyToIdle pins acceptance §2.4 — 5min 无 frame 自动
// idle. ReapStaleBusyToIdle UPDATE WHERE last_task_started_at < cutoff.
// IdleThreshold const 单一来源.
func TestAL_ReapStaleBusyToIdle(t *testing.T) {
	t.Parallel()
	_, _, st, agentID := al1b2Setup(t)

	// busy frame at T=0
	t0 := time.Unix(1700000000, 0)
	if err := st.SetAgentTaskStarted(agentID, "task-stale", t0); err != nil {
		t.Fatalf("SetAgentTaskStarted: %v", err)
	}

	// Reap at T+1min — should NOT reap (under 5min).
	tEarly := t0.Add(1 * time.Minute)
	n, err := st.ReapStaleBusyToIdle(tEarly, 5*time.Minute)
	if err != nil {
		t.Fatalf("reap early: %v", err)
	}
	if n != 0 {
		t.Errorf("reap at T+1min: %d rows, want 0 (under 5min threshold)", n)
	}
	row, err := st.GetAgentStatus(agentID)
	if err != nil || row.State != "busy" {
		t.Errorf("after early reap: state=%v err=%v, want busy", row, err)
	}

	// Reap at T+6min — should reap (over 5min).
	tLate := t0.Add(6 * time.Minute)
	n, err = st.ReapStaleBusyToIdle(tLate, 5*time.Minute)
	if err != nil {
		t.Fatalf("reap late: %v", err)
	}
	if n != 1 {
		t.Errorf("reap at T+6min: %d rows, want 1 (over threshold)", n)
	}
	row, err = st.GetAgentStatus(agentID)
	if err != nil {
		t.Fatalf("post-reap GetAgentStatus: %v", err)
	}
	if row.State != "idle" {
		t.Errorf("post-reap state = %q, want idle (acceptance §2.4 5min 无 frame → idle)", row.State)
	}
}

// TestAL_NoDomainBleed_Response pins acceptance §1.5 + spec §0 设计第 1 条
// 反向约束 — 5-state 合并响应不泄漏 schema 内列名 (反向断言 server 不返
// is_online / endpoint_url / process_kind / last_error_reason raw 文本).
// 跟 al_4_2 admin god-mode reason raw 反向约束同源.
func TestAL_NoDomainBleed_Response(t *testing.T) {
	t.Parallel()
	url, tok, st, agentID := al1b2Setup(t)
	now := time.Unix(1700000000, 0)
	_ = st.SetAgentTaskStarted(agentID, "task-bleed", now)

	resp, data := testutil.JSON(t, http.MethodGet, url+"/api/v1/agents/"+agentID+"/status", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: %d", resp.StatusCode)
	}
	for _, forbidden := range []string{
		// AL-3 presence 列不应出现 (真分清三路径).
		"is_online", "presence",
		// AL-4 runtime 列不应出现 (process-level vs task-level 真分清).
		"endpoint_url", "process_kind", "last_error_reason",
		// schema 内部列名直泄露反向约束 (resp 用 reason, 不用 raw 列名).
		"source", "set_by",
	} {
		if _, has := data[forbidden]; has {
			t.Errorf("response leaks forbidden field %q (反向约束 broken, acceptance §1.5 + spec §0 设计第 1+2 条)", forbidden)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
