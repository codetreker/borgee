// Package push_test — mention_notifier_test.go: DL-4.6 mention/agent_task
// → push fan-out adapter unit tests.
//
// Pins:
//   - NewMentionNotifier nil-safe (Gateway==nil → returns nil)
//   - NotifyMention payload shape byte-identical
//     {kind, from, channel, body, ts}
//   - NewAgentTaskNotifier nil-safe
//   - NotifyAgentTask payload shape byte-identical
//     {kind, agent_id, state, subject, reason, ts}
//   - Gateway recorded calls — observability (attempts count returned)
package push_test

import (
	"context"
	"sync"
	"testing"

	"borgee-server/internal/push"
)

// recordingGateway is a Gateway test-double that captures Send calls
// for assertion, matching the fakePusher / fakeNoopGateway pattern.
type recordingGateway struct {
	mu    sync.Mutex
	calls []recordedCall
}

type recordedCall struct {
	UserID  string
	Payload map[string]any
}

func (g *recordingGateway) Send(ctx context.Context, userID string, payload map[string]any) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, recordedCall{UserID: userID, Payload: payload})
	return 1 // simulate 1 subscription per user (observability hint)
}

func (g *recordingGateway) Calls() []recordedCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]recordedCall, len(g.calls))
	copy(out, g.calls)
	return out
}

// TestDL_NewMentionNotifier_NilSafe pins seam — nil Gateway → returns
// nil notifier (caller passes directly to MentionDispatcher.PushNotifier
// nil-safe field).
func TestDL_NewMentionNotifier_NilSafe(t *testing.T) {
	t.Parallel()
	n := push.NewMentionNotifier(nil)
	if n != nil {
		t.Errorf("NewMentionNotifier(nil) = %v, want nil", n)
	}
}

// TestDL_NotifyMention_PayloadShape pins payload byte-identical:
// {kind: "mention", from, channel, body, ts}.
func TestDL_NotifyMention_PayloadShape(t *testing.T) {
	t.Parallel()
	g := &recordingGateway{}
	n := push.NewMentionNotifier(g)
	if n == nil {
		t.Fatal("NewMentionNotifier with non-nil gateway returned nil")
	}

	got := n.NotifyMention("user-target", "user-sender", "general", "hey @target", 1700000000000)
	if got != 1 {
		t.Errorf("NotifyMention returned %d attempts, want 1 (recordingGateway always 1)", got)
	}

	calls := g.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 gateway call, got %d", len(calls))
	}
	c := calls[0]
	if c.UserID != "user-target" {
		t.Errorf("Send userID = %q, want user-target", c.UserID)
	}

	// Payload field byte-identity check (blueprint client-shape.md L37 wording).
	want := map[string]any{
		"kind":    "mention",
		"from":    "user-sender",
		"channel": "general",
		"body":    "hey @target",
		"ts":      int64(1700000000000),
	}
	for k, v := range want {
		if c.Payload[k] != v {
			t.Errorf("payload[%q] = %v (%T), want %v (%T)",
				k, c.Payload[k], c.Payload[k], v, v)
		}
	}
	if len(c.Payload) != len(want) {
		t.Errorf("payload key count = %d, want %d (extra keys: %v)",
			len(c.Payload), len(want), c.Payload)
	}
}

// TestDL_NewAgentTaskNotifier_NilSafe pins seam.
func TestDL_NewAgentTaskNotifier_NilSafe(t *testing.T) {
	t.Parallel()
	n := push.NewAgentTaskNotifier(nil)
	if n != nil {
		t.Errorf("NewAgentTaskNotifier(nil) = %v, want nil", n)
	}
}

// TestDL_NotifyAgentTask_PayloadShape pins payload {kind, agent_id,
// state, subject, reason, ts}.
func TestDL_NotifyAgentTask_PayloadShape(t *testing.T) {
	t.Parallel()
	g := &recordingGateway{}
	n := push.NewAgentTaskNotifier(g)
	if n == nil {
		t.Fatal("NewAgentTaskNotifier with non-nil gateway returned nil")
	}

	// busy state
	got := n.NotifyAgentTask("user-recipient", "agent-A", "busy", "writing section 3", "", 1700000000000)
	if got != 1 {
		t.Errorf("NotifyAgentTask returned %d, want 1", got)
	}

	// idle state with reason
	_ = n.NotifyAgentTask("user-recipient", "agent-A", "idle", "", "runtime_timeout", 1700000000005)

	calls := g.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	wantBusy := map[string]any{
		"kind":     "agent_task",
		"agent_id": "agent-A",
		"state":    "busy",
		"subject":  "writing section 3",
		"reason":   "",
		"ts":       int64(1700000000000),
	}
	for k, v := range wantBusy {
		if calls[0].Payload[k] != v {
			t.Errorf("busy payload[%q] = %v, want %v", k, calls[0].Payload[k], v)
		}
	}
	if len(calls[0].Payload) != len(wantBusy) {
		t.Errorf("busy payload key count = %d, want %d", len(calls[0].Payload), len(wantBusy))
	}

	// idle payload includes reason field non-empty
	if calls[1].Payload["state"] != "idle" {
		t.Errorf("idle state payload[state] = %v", calls[1].Payload["state"])
	}
	if calls[1].Payload["reason"] != "runtime_timeout" {
		t.Errorf("idle payload[reason] = %v, want runtime_timeout", calls[1].Payload["reason"])
	}
}

// TestDL_Notifiers_NilNotifier_NoOp pins fire-and-forget — invoking
// Notify* on nil notifier returns 0 attempts without panic (caller-side
// nil-safe pattern allowing legacy code to inject nil gateway).
func TestDL_Notifiers_NilNotifier_NoOp(t *testing.T) {
	t.Parallel()
	var n *push.MentionNotifier
	if got := n.NotifyMention("u", "s", "c", "b", 1); got != 0 {
		t.Errorf("nil mention notifier returned %d attempts, want 0", got)
	}

	var an *push.AgentTaskNotifier
	if got := an.NotifyAgentTask("u", "a", "busy", "s", "", 1); got != 0 {
		t.Errorf("nil agent-task notifier returned %d attempts, want 0", got)
	}
}
