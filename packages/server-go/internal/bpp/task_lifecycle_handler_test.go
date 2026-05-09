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

func newHandler(t *testing.T) (*bpp.TaskLifecycleHandler, *recPusher) {
	t.Helper()
	p := &recPusher{}
	return bpp.NewTaskLifecycleHandler(p, nil), p
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
		t.Errorf("设计 ② fail-closed broken — pusher got %d calls on subject empty (expected 0)", len(p.calls))
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
		t.Errorf("push args drift: %+v", c)
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
		t.Errorf("idle fanout drift: %+v", c)
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
		t.Errorf("failed reason fanout drift: %+v", p.calls)
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
		t.Errorf("StartedAdapter dispatch drift: %+v", p.calls)
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
		t.Errorf("FinishedAdapter dispatch drift: %+v", p.calls)
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
	bpp.NewTaskLifecycleHandler(nil, nil)
}

func TestRT_StartedAdapter_EmptySubject_PreservesSentinelChain(t *testing.T) {
	t.Parallel()
	h, _ := newHandler(t)
	raw := json.RawMessage(`{"type":"task_started","task_id":"t1","agent_id":"a1","channel_id":"c1","subject":"","started_at":1700000000000}`)
	err := h.StartedAdapter().Dispatch(raw, bpp.PluginSessionContext{})
	if err == nil {
		t.Fatalf("expected sentinel err, got nil")
	}
	// errors.Is sanity (跟 BPP-2.2 sentinel chain 同源).
	if !errors.Is(err, err) { // tautology to keep import
		t.Errorf("errors.Is sanity")
	}
	if !bpp.IsTaskSubjectEmpty(err) {
		t.Errorf("设计 ② sentinel chain broken: %v", err)
	}
}
