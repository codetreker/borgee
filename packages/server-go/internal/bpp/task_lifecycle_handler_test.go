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

// stubMembership is the ChannelMembershipChecker test-double for the #1110
// cross-channel gate. Unlike recMembers (which only models the fanout target
// list, not authz), this models the STRICT membership authz decision:
// IsChannelMember returns `member` for every (channelID, agentID) pair so a
// test can pin the member-positive path (true) or the non-member reject path
// (false). lastChannelID / lastAgentID capture the most recent call args so a
// test can assert the gate passes frame.ChannelID + frame.AgentID through.
type stubMembership struct {
	member        bool
	lastChannelID string
	lastAgentID   string
}

func (s *stubMembership) IsChannelMember(channelID, agentID string) bool {
	s.lastChannelID = channelID
	s.lastAgentID = agentID
	return s.member
}

// newHandlerWithOwner builds a handler whose resolver maps every agent to
// ownerOf, with push fanout wired via SetPushFanout AND a member-positive
// channel-membership stub (member=true) wired via SetChannelMembership so the
// same-owner positive asserts both push (recPusher) and fanout (recNotifier)
// fire and the #1110 gate is satisfied for these positive paths.
func newHandlerWithOwner(t *testing.T, ownerOf string) (*bpp.TaskLifecycleHandler, *recPusher, *recNotifier) {
	t.Helper()
	h, p, n, _ := newHandlerWithOwnerAndMembership(t, ownerOf, true)
	return h, p, n
}

// newHandlerWithOwnerAndMembership is newHandlerWithOwner plus a configurable
// channel-membership decision (member). It returns the stub so #1110 tests can
// assert the gate's call args. member=true → frame's agent is a strict member
// (positive path); member=false → non-member (cross-channel reject path).
func newHandlerWithOwnerAndMembership(t *testing.T, ownerOf string, member bool) (*bpp.TaskLifecycleHandler, *recPusher, *recNotifier, *stubMembership) {
	t.Helper()
	p := &recPusher{}
	n := &recNotifier{}
	mem := &stubMembership{member: member}
	h := bpp.NewTaskLifecycleHandler(p, rtStubOwner{ownerOf: ownerOf}, nil)
	h.SetPushFanout(&recMembers{userIDs: []string{"member-1", "member-2"}}, n)
	h.SetChannelMembership(mem)
	return h, p, n, mem
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

// --- #1110 cross-channel membership gate regression (red→green) --------------
//
// Sibling of the #1029 cross-owner gate above. The attack: a same-owner agent
// (passes the owner gate) targets a channel it is NOT a strict member of. The
// gate must reject with errTaskCrossChannelReject BEFORE push / fanout — 0
// push, 0 fanout. stubMembership models the STRICT authz decision (member=false
// → non-member); recMembers above only models the fanout target list, not authz.

// TestRT_StartedAdapter_NonMemberChannel_Reject pins #1110 for task_started: a
// frame whose agent passes the owner gate (same owner) but is NOT a member of
// frame.ChannelID is rejected with errTaskCrossChannelReject — 0 push, 0 fanout.
func TestRT_StartedAdapter_NonMemberChannel_Reject(t *testing.T) {
	t.Parallel()
	// agent "a1" is owned by "u1" (same as sess owner → owner gate passes) but
	// is NOT a member of channel "c-foreign" (member=false).
	h, p, n, mem := newHandlerWithOwnerAndMembership(t, "u1", false)
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c-foreign","subject":"分析中","started_at":1700000000000}`)
	err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"})
	if !bpp.IsTaskCrossChannelReject(err) {
		t.Fatalf("expected errTaskCrossChannelReject, got %v", err)
	}
	if len(p.calls) != 0 {
		t.Errorf("cross-channel gate broken — pusher got %d calls (want 0)", len(p.calls))
	}
	if len(n.calls) != 0 {
		t.Errorf("cross-channel gate broken — notifier got %d fanout calls (want 0)", len(n.calls))
	}
	// gate must pass frame.ChannelID + frame.AgentID through to IsChannelMember.
	if mem.lastChannelID != "c-foreign" || mem.lastAgentID != "a1" {
		t.Errorf("gate called IsChannelMember(%q, %q), want (c-foreign, a1)", mem.lastChannelID, mem.lastAgentID)
	}
}

// TestRT_FinishedAdapter_NonMemberChannel_Reject pins #1110 for task_finished:
// same-owner agent + non-member channel → cross-channel reject, 0 push, 0 fanout.
func TestRT_FinishedAdapter_NonMemberChannel_Reject(t *testing.T) {
	t.Parallel()
	h, p, n, mem := newHandlerWithOwnerAndMembership(t, "u1", false)
	raw := json.RawMessage(`{"type":"task_finished","task_id":"t1","agent_id":"a1","channel_id":"c-foreign","outcome":"completed","reason":"","finished_at":1700000001000}`)
	err := h.FinishedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"})
	if !bpp.IsTaskCrossChannelReject(err) {
		t.Fatalf("expected errTaskCrossChannelReject, got %v", err)
	}
	if len(p.calls) != 0 {
		t.Errorf("cross-channel gate broken — pusher got %d calls (want 0)", len(p.calls))
	}
	if len(n.calls) != 0 {
		t.Errorf("cross-channel gate broken — notifier got %d fanout calls (want 0)", len(n.calls))
	}
	if mem.lastChannelID != "c-foreign" || mem.lastAgentID != "a1" {
		t.Errorf("gate called IsChannelMember(%q, %q), want (c-foreign, a1)", mem.lastChannelID, mem.lastAgentID)
	}
}

// TestRT_StartedAdapter_MemberChannel_PushAndFanout pins the #1110 positive
// contrast for task_started: same-owner agent + a channel it IS a member of
// (member=true) passes both gates and fires push + fanout.
func TestRT_StartedAdapter_MemberChannel_PushAndFanout(t *testing.T) {
	t.Parallel()
	h, p, n, mem := newHandlerWithOwnerAndMembership(t, "u1", true)
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c1","subject":"分析中","started_at":1700000000000}`)
	if err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"}); err != nil {
		t.Fatalf("member dispatch rejected unexpectedly: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "busy" || p.calls[0].Subject != "分析中" {
		t.Errorf("member push mismatch: %+v", p.calls)
	}
	if len(n.calls) != 2 {
		t.Fatalf("member fanout = %d, want 2 (both members)", len(n.calls))
	}
	if mem.lastChannelID != "c1" || mem.lastAgentID != "a1" {
		t.Errorf("gate called IsChannelMember(%q, %q), want (c1, a1)", mem.lastChannelID, mem.lastAgentID)
	}
}

// TestRT_FinishedAdapter_MemberChannel_PushAndFanout pins the #1110 positive
// contrast for task_finished: member channel → idle push + fanout fire.
func TestRT_FinishedAdapter_MemberChannel_PushAndFanout(t *testing.T) {
	t.Parallel()
	h, p, n, _ := newHandlerWithOwnerAndMembership(t, "u1", true)
	raw := json.RawMessage(`{"type":"task_finished","task_id":"t1","agent_id":"a1","channel_id":"c1","outcome":"failed","reason":"runtime_crashed","finished_at":1700000002000}`)
	if err := h.FinishedAdapter().Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u1"}); err != nil {
		t.Fatalf("member dispatch rejected unexpectedly: %v", err)
	}
	if len(p.calls) != 1 || p.calls[0].State != "idle" || p.calls[0].Reason != "runtime_crashed" {
		t.Errorf("member idle push mismatch: %+v", p.calls)
	}
	if len(n.calls) != 2 {
		t.Fatalf("member fanout = %d, want 2", len(n.calls))
	}
}
