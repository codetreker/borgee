// Package push — mention_notifier.go: DL-4.6 mention → push fan-out
// adapter. Wraps Gateway.Send(userID, payload) into the
// MentionPushNotifier interface that internal/api/mention_dispatch.go
// expects.
//
// Blueprint锚: docs/blueprint/current/client-shape.md L37 ("@你, agent 完成长任务
// — AI 团队异步协作的核心 UX") + DL-4 spec §1 DL-4.6 fan-out hook.
//
// What this adapter does:
//   1. Translate (sender_id, channel_name, body_preview) → push payload
//      JSON {kind: "mention", channel: ..., from: ..., body: ...}.
//   2. Invoke Gateway.Send(targetUserID, payload) — best-effort, returns
//      attempt count for observability.
//
// 反约束 (DL-4 spec §0 设计 ②③):
//   - fire-and-forget: 不返 error, 仅返 attempts count.
//   - payload 不带 secret / token (跟 蓝图 §1.4 隐私 + AL-2a #447 SSOT
//     设计沿用, push payload 是 metadata only).
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
// Payload shape (蓝图 client-shape.md L37 字面 "@你"):
//
//	{
//	  "kind": "mention",
//	  "from": <sender_id>,
//	  "channel": <channel_name>,  // 不是 channel_id, 直接给人看
//	  "body": <body_preview>,     // 已 80 rune 截断 (DM-2.2)
//	  "ts": <created_at>          // Unix ms
//	}
//
// Returns attempts count (跟 Gateway.Send 同语义, observability only).
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
// Fired when an agent transitions busy↔idle (蓝图 client-shape.md L37
// "agent 完成长任务"). Invoked from server-derive hook (RT-3.2 后续
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
//	  "subject": <subject>,    // busy 必带非空 (蓝图 §1.1 ⭐), idle 空
//	  "reason": <reason>,      // idle+failed 时 AL-1a 6 字典, 否则空
//	  "ts": <changed_at>
//	}
//
// 反约束: 跟 RT-3 frame 同设计 — busy 态 subject 必带非空 (蓝图 §1.1
// 字面 "沉默胜于假 loading"); 调用方有责任传非空 subject (validator
// 在 RT-3.2 派生 hook 已守, 此 notifier 只 forward).
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
