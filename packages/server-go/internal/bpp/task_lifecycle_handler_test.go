// Package bpp — task_lifecycle_handler_test.go: RT-3 server派生 hook
// unit tests. Covers设计 ②+③:
//
//   - HandleStarted: empty subject → errSubjectEmpty (反 fallback push)
//   - HandleStarted: valid → pusher receives state='busy' + subject 透传
//   - HandleFinished: completed → pusher state='idle' subject="" reason=""
//   - HandleFinished: failed + AL-1a reason → pusher state='idle' reason 透传
//   - HandleFinished: invalid outcome → errOutcomeUnknown (反 push)
//   - HandleFinished: completed + reason → errOutcomeUnknown (字典污染防御)
//   - StartedAdapter / FinishedAdapter raw JSON decode + dispatch chain
//   - panic: NewTaskLifecycleHandler nil pusher
//
// Pusher seam (recPusher) records all calls for assertion — captures
// the 6 args of PushAgentTaskStateChanged byte-identical.

package bpp_test

import (
	"encoding/json"
	"errors"
	"testing"

	"borgee-server/internal/bpp"
)

type recPusherCall struct {
	AgentID, ChannelID, State, Subject, Reason string
	ChangedAt                                  int64
}

type recPusher struct {
	calls []recPusherCall
}

func (r *recPusher) PushAgentTaskStateChanged(agentID, channelID, state, subject, reason string, changedAt int64) (int64, bool) {
	r.calls = append(r.calls, recPusherCall{agentID, channelID, state, subject, reason, changedAt})
	return int64(len(r.calls)), true
}

// rtStubOwner is the OwnerResolver test-double for the typed/adapter unit
// tests. It maps every agent_id to ownerOf so same-owner sess (OwnerUserID ==
// ownerOf) passes the cross-owner gate; ownerOf defaults to "u1" matching the
// adapter dispatch tests' sess owner. A non-nil err is returned as a resolve
// failure (treated as a reject by the gate).
type rtStubOwner struct {
	ownerOf string
	err     error
}

func (r rtStubOwner) OwnerOf(string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.ownerOf, nil
}

func newHandler(t *testing.T) (*bpp.TaskLifecycleHandler, *recPusher) {
	t.Helper()
	p := &recPusher{}
	return bpp.NewTaskLifecycleHandler(p, rtStubOwner{ownerOf: "u1"}, nil), p
}

func TestRT_HandleStarted_EmptySubjectRejected(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	err := h.HandleStarted(bpp.TaskStartedFrame{
		Type: "task_started", TaskID: "t1", AgentID: "a1",
		ChannelID: "c1", Subject: "  ", StartedAt: 1700000000000,
	})
	if !bpp.IsTaskSubjectEmpty(err) {
		t.Fatalf("expected errSubjectEmpty, got %v", err)
	}
	if len(p.calls) != 0 {
		t.Errorf("规则 ② fail-closed broken — pusher got %d calls on subject empty (expected 0)", len(p.calls))
	}
}

func TestRT_HandleStarted_HappyPath_BusyFanout(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	if err := h.HandleStarted(bpp.TaskStartedFrame{
		Type: "task_started", TaskID: "t1", AgentID: "a1",
		ChannelID: "c1", Subject: "正在分析订单数据", StartedAt: 1700000000000,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(p.calls) != 1 {
		t.Fatalf("expected 1 push call, got %d", len(p.calls))
	}
	c := p.calls[0]
	if c.AgentID != "a1" || c.ChannelID != "c1" || c.State != "busy" ||
		c.Subject != "正在分析订单数据" || c.Reason != "" || c.ChangedAt != 1700000000000 {
		t.Errorf("push args mismatch: %+v", c)
	}
}

func TestRT_HandleFinished_Completed_IdleFanout(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	if err := h.HandleFinished(bpp.TaskFinishedFrame{
		Type: "task_finished", TaskID: "t1", AgentID: "a1",
		ChannelID: "c1", Outcome: "completed", Reason: "",
		FinishedAt: 1700000001000,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(p.calls) != 1 {
		t.Fatalf("expected 1 push call, got %d", len(p.calls))
	}
	c := p.calls[0]
	if c.State != "idle" || c.Subject != "" || c.Reason != "" {
		t.Errorf("idle fanout mismatch: %+v", c)
	}
}

func TestRT_HandleFinished_Failed_ReasonTransparent(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	if err := h.HandleFinished(bpp.TaskFinishedFrame{
		Type: "task_finished", TaskID: "t1", AgentID: "a1",
		ChannelID: "c1", Outcome: "failed", Reason: "runtime_crashed",
		FinishedAt: 1700000002000,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "idle" || p.calls[0].Reason != "runtime_crashed" {
		t.Errorf("failed reason fanout mismatch: %+v", p.calls)
	}
}

func TestRT_HandleFinished_InvalidOutcome_Rejected(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	err := h.HandleFinished(bpp.TaskFinishedFrame{
		Type: "task_finished", TaskID: "t1", AgentID: "a1",
		ChannelID: "c1", Outcome: "partial", FinishedAt: 1700000000000,
	})
	if !bpp.IsTaskOutcomeUnknown(err) {
		t.Fatalf("expected errOutcomeUnknown, got %v", err)
	}
	if len(p.calls) != 0 {
		t.Errorf("中间态 fail-closed broken — pusher got %d calls", len(p.calls))
	}
}

func TestRT_HandleFinished_CompletedWithReason_RejectedDictPollution(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	err := h.HandleFinished(bpp.TaskFinishedFrame{
		Type: "task_finished", TaskID: "t1", AgentID: "a1",
		ChannelID: "c1", Outcome: "completed", Reason: "runtime_crashed",
		FinishedAt: 1700000000000,
	})
	if err == nil {
		t.Fatalf("expected字典污染 reject, got nil")
	}
	if len(p.calls) != 0 {
		t.Errorf("字典污染 fail-closed broken — got %d calls", len(p.calls))
	}
}

func TestRT_StartedAdapter_RawDecode_Dispatch(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c1","subject":"分析中","started_at":1700000000000}`)
	if err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"}); err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "busy" || p.calls[0].Subject != "分析中" {
		t.Errorf("StartedAdapter dispatch mismatch: %+v", p.calls)
	}
}

func TestRT_FinishedAdapter_RawDecode_Dispatch(t *testing.T) {
	t.Parallel()
	h, p := newHandler(t)
	raw := json.RawMessage(`{"type":"task_finished","task_id":"t1","agent_id":"a1","channel_id":"c1","outcome":"completed","reason":"","finished_at":1700000001000}`)
	if err := h.FinishedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"}); err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "idle" {
		t.Errorf("FinishedAdapter dispatch mismatch: %+v", p.calls)
	}
}

func TestRT_StartedAdapter_BadJSON_DecodeErr(t *testing.T) {
	t.Parallel()
	h, _ := newHandler(t)
	err := h.StartedAdapter().Dispatch(json.RawMessage(`{not json}`), bpp.PluginSessionContext{})
	if err == nil {
		t.Errorf("expected decode err, got nil")
	}
}

func TestRT_NewTaskLifecycleHandler_NilPusherPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil pusher, got none")
		}
	}()
	bpp.NewTaskLifecycleHandler(nil, rtStubOwner{ownerOf: "u1"}, nil)
}

func TestRT_NewTaskLifecycleHandler_NilResolverPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil resolver, got none")
		}
	}()
	bpp.NewTaskLifecycleHandler(&recPusher{}, nil, nil)
}

func TestRT_StartedAdapter_EmptySubject_PreservesSentinelChain(t *testing.T) {
	t.Parallel()
	h, _ := newHandler(t)
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c1","subject":"","started_at":1700000000000}`)
	err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"})
	if err == nil {
		t.Fatalf("expected sentinel err, got nil")
	}
	// errors.Is sanity (跟 BPP-2.2 sentinel chain 同源).
	if !errors.Is(err, err) { // tautology to keep import
		t.Errorf("errors.Is sanity")
	}
	if !bpp.IsTaskSubjectEmpty(err) {
		t.Errorf("规则 ② sentinel chain broken: %v", err)
	}
}

// --- #1029 cross-owner gate regression (red→green) ---------------------------
//
// recNotifier / recMembers are the external-package push-fanout doubles for the
// same-owner positive case: SetPushFanout wires them so 0-fanout-on-reject is a
// real contrast (otherwise 0-fanout is trivially true with no notifier set).

type recNotifyCall struct {
	targetUserID, agentID, state, subject, reason string
	ts                                            int64
}

type recNotifier struct{ calls []recNotifyCall }

func (n *recNotifier) NotifyAgentTask(targetUserID, agentID, state, subject, reason string, changedAt int64) int {
	n.calls = append(n.calls, recNotifyCall{targetUserID, agentID, state, subject, reason, changedAt})
	return 1
}

type recMembers struct{ userIDs []string }

func (m *recMembers) ListChannelMemberUserIDs(string) ([]string, error) { return m.userIDs, nil }

// newHandlerWithOwner builds a handler whose resolver maps every agent to
// ownerOf, with push fanout wired via SetPushFanout so the same-owner positive
// asserts both push (recPusher) and fanout (recNotifier) fire.
func newHandlerWithOwner(t *testing.T, ownerOf string) (*bpp.TaskLifecycleHandler, *recPusher, *recNotifier) {
	t.Helper()
	p := &recPusher{}
	n := &recNotifier{}
	h := bpp.NewTaskLifecycleHandler(p, rtStubOwner{ownerOf: ownerOf}, nil)
	h.SetPushFanout(&recMembers{userIDs: []string{"member-1", "member-2"}}, n)
	return h, p, n
}

// TestRT_StartedAdapter_CrossOwner_Reject pins #1029: a task_started frame
// whose agent is owned by someone other than the authenticated plugin session
// owner is rejected with errTaskCrossOwnerReject BEFORE HandleStarted — 0 push,
// 0 fanout.
func TestRT_StartedAdapter_CrossOwner_Reject(t *testing.T) {
	t.Parallel()
	// agent "a1" is owned by "victim"; the session owner is "attacker".
	h, p, n := newHandlerWithOwner(t, "victim")
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c1","subject":"分析中","started_at":1700000000000}`)
	err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "attacker"})
	if !bpp.IsTaskCrossOwnerReject(err) {
		t.Fatalf("expected errTaskCrossOwnerReject, got %v", err)
	}
	if len(p.calls) != 0 {
		t.Errorf("cross-owner gate broken — pusher got %d calls (want 0)", len(p.calls))
	}
	if len(n.calls) != 0 {
		t.Errorf("cross-owner gate broken — notifier got %d fanout calls (want 0)", len(n.calls))
	}
}

// TestRT_FinishedAdapter_CrossOwner_Reject pins #1029 for the task_finished
// adapter: cross-owner → reject sentinel, 0 push, 0 fanout.
func TestRT_FinishedAdapter_CrossOwner_Reject(t *testing.T) {
	t.Parallel()
	h, p, n := newHandlerWithOwner(t, "victim")
	raw := json.RawMessage(`{"type":"task_finished","task_id":"t1","agent_id":"a1","channel_id":"c1","outcome":"completed","reason":"","finished_at":1700000001000}`)
	err := h.FinishedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "attacker"})
	if !bpp.IsTaskCrossOwnerReject(err) {
		t.Fatalf("expected errTaskCrossOwnerReject, got %v", err)
	}
	if len(p.calls) != 0 {
		t.Errorf("cross-owner gate broken — pusher got %d calls (want 0)", len(p.calls))
	}
	if len(n.calls) != 0 {
		t.Errorf("cross-owner gate broken — notifier got %d fanout calls (want 0)", len(n.calls))
	}
}

// TestRT_Adapter_CrossOwner_ResolveErr_Reject pins that an OwnerOf error (agent
// unknown / disconnected) is treated as a reject — mirrors AckDispatcher:176.
func TestRT_Adapter_CrossOwner_ResolveErr_Reject(t *testing.T) {
	t.Parallel()
	p := &recPusher{}
	n := &recNotifier{}
	h := bpp.NewTaskLifecycleHandler(p, rtStubOwner{err: errors.New("agent not found")}, nil)
	h.SetPushFanout(&recMembers{userIDs: []string{"member-1"}}, n)
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"ghost","channel_id":"c1","subject":"分析中","started_at":1700000000000}`)
	err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "attacker"})
	if !bpp.IsTaskCrossOwnerReject(err) {
		t.Fatalf("expected errTaskCrossOwnerReject on resolve err, got %v", err)
	}
	if len(p.calls) != 0 || len(n.calls) != 0 {
		t.Errorf("resolve-err gate broken — push=%d fanout=%d (want 0/0)", len(p.calls), len(n.calls))
	}
}

// TestRT_StartedAdapter_SameOwner_PushAndFanout pins the same-owner positive
// contrast: a frame whose agent owner == session owner passes the gate and
// fires BOTH push (recPusher) and fanout (recNotifier, wired via SetPushFanout
// so 0-fanout-on-reject above is a real contrast, not trivially true).
func TestRT_StartedAdapter_SameOwner_PushAndFanout(t *testing.T) {
	t.Parallel()
	h, p, n := newHandlerWithOwner(t, "u1")
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c1","subject":"分析中","started_at":1700000000000}`)
	if err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"}); err != nil {
		t.Fatalf("same-owner dispatch rejected unexpectedly: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "busy" || p.calls[0].Subject != "分析中" {
		t.Errorf("same-owner push mismatch: %+v", p.calls)
	}
	if len(n.calls) != 2 {
		t.Fatalf("same-owner fanout = %d, want 2 (both members)", len(n.calls))
	}
	for _, c := range n.calls {
		if c.state != "busy" || c.subject != "分析中" {
			t.Errorf("fanout drift: %+v", c)
		}
	}
}

// TestRT_FinishedAdapter_SameOwner_PushAndFanout pins the task_finished
// same-owner positive: idle push + fanout fire.
func TestRT_FinishedAdapter_SameOwner_PushAndFanout(t *testing.T) {
	t.Parallel()
	h, p, n := newHandlerWithOwner(t, "u1")
	raw := json.RawMessage(`{"type":"task_finished","task_id":"t1","agent_id":"a1","channel_id":"c1","outcome":"failed","reason":"runtime_crashed","finished_at":1700000002000}`)
	if err := h.FinishedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"}); err != nil {
		t.Fatalf("same-owner dispatch rejected unexpectedly: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "idle" || p.calls[0].Reason != "runtime_crashed" {
		t.Errorf("same-owner idle push mismatch: %+v", p.calls)
	}
	if len(n.calls) != 2 {
		t.Fatalf("same-owner fanout = %d, want 2", len(n.calls))
	}
}
