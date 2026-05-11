// Package ws — permission_denied_frame.go: BPP-3.1 hub method for emitting
// PermissionDeniedFrame to the target agent's plugin connection
// (server→plugin direction lock).
//
// Blueprint锚: docs/blueprint/current/auth-permissions.md §2 不变量 "Permission
// denied 走 BPP — 不靠 HTTP 错误码, 由协议层路由到 owner DM" + §4.1 row
// 字面 frame 字段 (`attempted_action`, `required_capability`, `current_scope`).
// Spec: docs/implementation/modules/bpp-3.1-spec.md.
//
// Behaviour contract — byte-identical 跟 PushArtifactUpdated /
// PushAnchorCommentAdded / PushMentionPushed / PushIterationStateChanged
// / PushAgentConfigUpdate 5-frame 共序模式:
//
//   1. Cursor 走 hub.cursors.NextCursor() 单调发号, 跟 RT-1/CV-2/DM-2/CV-4/
//      AL-2b 共一根 sequence (约束: 不另起 plugin-only 推送通道).
//      BPP-3.1 是第 6 个共序 frame.
//   2. Direction lock = server→plugin; 只发给目标 agent 的 PluginConn
//      (h.plugins[agentID]), 不 broadcast.
//   3. 字段顺序锁: type/cursor/agent_id/request_id/attempted_action/
//      required_capability/current_scope/denied_at — 跟 BPP-1 #304 envelope
//      CI lint reflect 自动覆盖.
//   4. plugin 离线 frame 丢弃 (跟 PushAgentConfigUpdate 同模式 — 约束:
//      不入队列, plugin 重连后 GET 主动拉; 蓝图 §1.5 字面 "runtime 不缓存"
//      同精神).
//
// 约束 (spec §2):
//   - admin god-mode 不调此方法 (ADM-0 §1.3 红线 — admin 不入业务路径).
//     调用方 (AP-1 abac.go::HasCapability false 路径) 必须先确认 user.Role
//     != "admin"; 此方法不做 ACL — 跟 PushArtifactUpdated 同模式.
//   - plugin 端永不发 permission_denied (direction lock by bppEnvelopeWhitelist
//     + reflect lint 双闸守).
//   - HTTP 403 是 fallback, BPP frame 是 primary (蓝图 §2 不变量).

package ws

import (
	"borgee-server/internal/bpp"
)

// PermissionDeniedPusher is the seam between the api package and ws.Hub
// for the BPP-3.1 permission_denied frame (mirrors AgentConfigPusher
// pattern in api/agent_config.go so the api package doesn't import
// internal/ws). AP-1 (#493) abac.go::HasCapability false path will wire
// this via 1-line follow-up after AP-1 + BPP-3.1 both merge.
//
// Implemented by *ws.Hub.PushPermissionDenied; injected as nil-safe
// optional field on relevant handlers.
type PermissionDeniedPusher interface {
	PushPermissionDenied(
		agentID string,
		requestID string,
		attemptedAction string,
		requiredCapability string,
		currentScope string,
		deniedAt int64,
	) (cursor int64, sent bool)
}

// PushPermissionDenied emits a PermissionDeniedFrame to the target agent's
// plugin connection. Returns (cursor, sent):
//
//   - cursor: hub.cursors monotonic sequence number (0 if no allocator,
//     test seam).
//   - sent: true iff plugin connection exists for agentID AND frame
//     enqueued to its send channel. false otherwise (plugin offline /
//     no allocator / channel buffer full).
//
// Frame field assignment is byte-identical with bpp.PermissionDeniedFrame
// (spec §1 设计 ① 8 字段); reordering arguments here without updating the
// frame struct is a CI red caught by frame_schemas_test.go reflect lint.
//
// Caller responsibilities:
//   - requestID: AP-1 调用方生成的 trace UUID, plugin 端按此 key 关联
//     owner DM 推审批通知 + retry (BPP-3.2 follow-up).
//   - attemptedAction: ∈ BPP-2.1 7 op 白名单 (`bpp.SemanticOp*` const)
//     或 REST endpoint 名; 约束: v2+ 枚举外值不入此路径.
//   - requiredCapability / currentScope: byte-identical 跟 AP-1 abac.go
//     403 body (改 = 改三处 — 蓝图 §4.1 + AP-1 + BPP-3.1, 双向 grep 守).
//   - deniedAt: Unix ms 语义戳 (约束: cursor 才是排序源).
func (h *Hub) PushPermissionDenied(
	agentID string,
	requestID string,
	attemptedAction string,
	requiredCapability string,
	currentScope string,
	deniedAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()

	frame := bpp.PermissionDeniedFrame{
		Type:               bpp.FrameTypeBPPPermissionDenied,
		Cursor:             cur,
		AgentID:            agentID,
		RequestID:          requestID,
		AttemptedAction:    attemptedAction,
		RequiredCapability: requiredCapability,
		CurrentScope:       currentScope,
		DeniedAt:           deniedAt,
	}

	pc := h.GetPlugin(agentID)
	if pc == nil {
		// Plugin offline — frame dropped. AP-1 caller still emits HTTP 403
		// fallback so the immediate request fails fast.
		return cur, false
	}

	pc.sendJSON(frame)
	return cur, true
}
