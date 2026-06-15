// Package bpp — task_lifecycle_wire_test.go: WIRE-1 wire-3 production
// wire-up 真测 (RT-3 server-derive hook → DL-4 push gateway fanout).
//
// Spec: docs/implementation/modules/wire-1-spec.md §1 W1.2 wire-3.
//
// Pin: handler.SetPushFanout 后 task_started → notifier.NotifyAgentTask
// 调用 ≥1 hit per channel member (反 self-push agent 自己 + 空 user_id).

package bpp

import (
	"errors"
	"testing"
)

type stubMembers struct {
	userIDs []string
	err     error
}

func (s *stubMembers) ListChannelMemberUserIDs(_ string) ([]string, error) {
	return s.userIDs, s.err
}

type stubPushNotifier struct {
	calls []notifyCall
}

type notifyCall struct {
	targetUserID, agentID, state, subject, reason string
	ts                                            int64
}

func (s *stubPushNotifier) NotifyAgentTask(targetUserID, agentID, state, subject, reason string, changedAt int64) int {
	s.calls = append(s.calls, notifyCall{targetUserID, agentID, state, subject, reason, changedAt})
	return 1
}

type stubPusher struct{}

func (stubPusher) PushAgentTaskStateChanged(_ string, _ string, _ string, _ string, _ string, _ int64) (int64, bool) {
	return 1, true
}

// wireStubOwner satisfies OwnerResolver for the wire tests. These tests drive
// the typed HandleStarted / HandleFinished entries (session-free), so the
// resolver is never consulted; it only satisfies the non-nil ctor contract.
type wireStubOwner struct{}

func (wireStubOwner) OwnerOf(agentID string) (string, error) { return agentID, nil }

// TestWire3_TaskStarted_PushFanoutPerMember covers the happy path:
// 3 channel members → 3 NotifyAgentTask calls (minus agent self).
func TestWire3_TaskStarted_PushFanoutPerMember(t *testing.T) {
	t.Parallel()
	h := NewTaskLifecycleHandler(stubPusher{}, wireStubOwner{}, nil)
	notifier := &stubPushNotifier{}
	members := &stubMembers{userIDs: []string{"u-1", "u-2", "agent-A", ""}}
	h.SetPushFanout(members, notifier)

	frame := TaskStartedFrame{
		Type:      FrameTypeBPPTaskStarted,
		TaskID:    "t-1",
		AgentID:   "agent-A",
		ChannelID: "ch-1",
		Subject:   "doing the thing",
		StartedAt: 1700000000000,
	}
	if err := h.HandleStarted(frame); err != nil {
		t.Fatalf("HandleStarted: %v", err)
	}
	// agent-A 自身 + 空 user_id 跳, 仅 u-1 + u-2 收 notify.
	if len(notifier.calls) != 2 {
		t.Fatalf("notify calls = %d, want 2 (agent self + empty 跳过)", len(notifier.calls))
	}
	for _, c := range notifier.calls {
		if c.state != "busy" {
			t.Errorf("state = %q, want busy", c.state)
		}
		if c.subject != "doing the thing" {
			t.Errorf("subject drift: %q", c.subject)
		}
	}
}

// TestWire3_TaskFinished_IdleFanout pins task_finished → idle state.
func TestWire3_TaskFinished_IdleFanout(t *testing.T) {
	t.Parallel()
	h := NewTaskLifecycleHandler(stubPusher{}, wireStubOwner{}, nil)
	notifier := &stubPushNotifier{}
	h.SetPushFanout(&stubMembers{userIDs: []string{"u-1"}}, notifier)

	frame := TaskFinishedFrame{
		Type:       FrameTypeBPPTaskFinished,
		TaskID:     "t-1",
		AgentID:    "agent-A",
		ChannelID:  "ch-1",
		Outcome:    "completed",
		Reason:     "",
		FinishedAt: 1700000001000,
	}
	if err := h.HandleFinished(frame); err != nil {
		t.Fatal(err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(notifier.calls))
	}
	c := notifier.calls[0]
	if c.state != "idle" || c.subject != "" {
		t.Errorf("idle frame: state=%q subject=%q", c.state, c.subject)
	}
}

// TestWire3_NilFanout_NoOp pins SetPushFanout 未调 → fanout 跳 (反 panic).
func TestWire3_NilFanout_NoOp(t *testing.T) {
	t.Parallel()
	h := NewTaskLifecycleHandler(stubPusher{}, wireStubOwner{}, nil)
	// 不 SetPushFanout → members + notifier 均 nil.
	frame := TaskStartedFrame{
		Type: FrameTypeBPPTaskStarted, TaskID: "t", AgentID: "a", ChannelID: "c",
		Subject: "x", StartedAt: 1,
	}
	if err := h.HandleStarted(frame); err != nil {
		t.Errorf("HandleStarted nil-fanout panic? err=%v", err)
	}
}

// TestWire3_MembersErr_Skipped pins fetch err → fanout 跳, 不 panic.
func TestWire3_MembersErr_Skipped(t *testing.T) {
	t.Parallel()
	h := NewTaskLifecycleHandler(stubPusher{}, wireStubOwner{}, nil)
	notifier := &stubPushNotifier{}
	h.SetPushFanout(&stubMembers{err: errors.New("db closed")}, notifier)

	frame := TaskStartedFrame{
		Type: FrameTypeBPPTaskStarted, TaskID: "t", AgentID: "a", ChannelID: "c",
		Subject: "x", StartedAt: 1,
	}
	if err := h.HandleStarted(frame); err != nil {
		t.Errorf("err on members fetch should be swallowed (push 是 best-effort): %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("notify calls = %d, want 0 on member err", len(notifier.calls))
	}
}
