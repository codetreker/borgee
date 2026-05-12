// Package push — mention_notifier.go: DL-4.6 mention → push fan-out
// adapter. Wraps Gateway.Send(userID, payload) into the
// MentionPushNotifier interface that internal/api/mention_dispatch.go
// expects.
//
// Blueprint anchor: docs/blueprint/current/client-shape.md L37 ("@ you, agent
// finished a long task — core UX for async AI-team collaboration") + DL-4 spec
// §1 DL-4.6 fan-out hook.
//
// What this adapter does:
//   1. Translate (sender_id, channel_name, body_preview) → push payload
//      JSON {kind: "mention", channel: ..., from: ..., body: ...}.
//   2. Invoke Gateway.Send(targetUserID, payload) — best-effort, returns
//      attempt count for observability.
//
// Negative constraints (DL-4 spec §0 principles ②③):
//   - fire-and-forget: no error return, only an attempts count.
//   - payload carries no secret or token. It follows blueprint §1.4 privacy
//     and AL-2a #447 SSOT constraints: push payloads are metadata only.
package push

import "context"

// MentionNotifier is a Gateway-backed adapter satisfying the
// internal/api MentionPushNotifier interface (declared there to keep
// the api package importing push as a leaf dep).
type MentionNotifier struct {
	gateway Gateway
}

// NewMentionNotifier wraps a Gateway. Nil Gateway → returns nil (caller
// can pass directly to MentionDispatcher.PushNotifier which is nil-safe).
func NewMentionNotifier(g Gateway) *MentionNotifier {
	if g == nil {
		return nil
	}
	return &MentionNotifier{gateway: g}
}

// NotifyMention implements MentionPushNotifier — fires push to the
// mention target (cross-device, best-effort).
//
// Payload shape (blueprint client-shape.md L37 "@ you" wording):
//
//	{
//	  "kind": "mention",
//	  "from": <sender_id>,
//	  "channel": <channel_name>,  // channel name, not channel_id, for display
//	  "body": <body_preview>,     // already truncated to 80 runes (DM-2.2)
//	  "ts": <created_at>          // Unix ms
//	}
//
// Returns attempts count with the same observability-only semantics as Gateway.Send.
func (n *MentionNotifier) NotifyMention(targetUserID, senderID, channelName, bodyPreview string, createdAt int64) int {
	if n == nil || n.gateway == nil {
		return 0
	}
	payload := map[string]any{
		"kind":    "mention",
		"from":    senderID,
		"channel": channelName,
		"body":    bodyPreview,
		"ts":      createdAt,
	}
	return n.gateway.Send(context.Background(), targetUserID, payload)
}

// AgentTaskNotifier is the RT-3 agent_task_state_changed → push adapter.
// Fired when an agent transitions busy↔idle (blueprint client-shape.md L37
// "agent completed a long task"). Invoked from server-derive hook (RT-3.2 follow-up
// commit) for each channel member's user_id.
//
// Multi-recipient fan-out: caller iterates channel members + invokes
// per user_id.
type AgentTaskNotifier struct {
	gateway Gateway
}

// NewAgentTaskNotifier wraps a Gateway. Nil Gateway → returns nil
// (caller pre-checks).
func NewAgentTaskNotifier(g Gateway) *AgentTaskNotifier {
	if g == nil {
		return nil
	}
	return &AgentTaskNotifier{gateway: g}
}

// NotifyAgentTask fires push to recipient when agent state changes.
//
// Payload shape:
//
//	{
//	  "kind": "agent_task",
//	  "agent_id": <agent_id>,
//	  "state": "busy"|"idle",
//	  "subject": <subject>,    // non-empty when busy (blueprint §1.1 ⭐); empty when idle
//	  "reason": <reason>,      // AL-1a 6-reason dictionary for idle+failed; otherwise empty
//	  "ts": <changed_at>
//	}
//
// Negative constraint: same as the RT-3 frame. A busy state must include a
// non-empty subject (blueprint §1.1: "silence is better than fake loading").
// The caller must pass that subject; the RT-3.2 derive hook validates it, and
// this notifier only forwards the payload.
func (n *AgentTaskNotifier) NotifyAgentTask(targetUserID, agentID, state, subject, reason string, changedAt int64) int {
	if n == nil || n.gateway == nil {
		return 0
	}
	payload := map[string]any{
		"kind":     "agent_task",
		"agent_id": agentID,
		"state":    state,
		"subject":  subject,
		"reason":   reason,
		"ts":       changedAt,
	}
	return n.gateway.Send(context.Background(), targetUserID, payload)
}
