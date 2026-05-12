// Package ws — permission_denied_frame.go: BPP-3.1 hub method for emitting
// PermissionDeniedFrame only from the server to the target agent's plugin
// connection.
//
// Blueprint reference: docs/blueprint/current/auth-permissions.md §2 invariant:
// permission denial is sent through BPP rather than HTTP error codes, and the
// protocol layer routes it to the owner DM. Also see §4.1 row listing the
// exact frame fields (`attempted_action`,
// `required_capability`, `current_scope`).
// Spec: docs/implementation/modules/bpp-3.1-spec.md.
//
// Behaviour contract — keep this aligned with the shared sequencing
// pattern used by PushArtifactUpdated, PushAnchorCommentAdded,
// PushMentionPushed, PushIterationStateChanged, and PushAgentConfigUpdate:
//
//   1. Cursor values come from hub.cursors.NextCursor(), the same monotonic
//      sequence used by RT-1/CV-2/DM-2/CV-4/AL-2b. Do not add a separate
//      plugin-only push sequence. BPP-3.1 is the sixth frame type in this
//      shared sequence.
//   2. Direction is server-to-plugin only; send only to the target agent's
//      PluginConn (h.plugins[agentID]), never broadcast.
//   3. Field order is fixed: type/cursor/agent_id/request_id/attempted_action/
//      required_capability/current_scope/denied_at. The BPP-1 #304 envelope
//      reflection lint covers this order in CI.
//   4. Drop the frame when the plugin is offline, matching
//      PushAgentConfigUpdate. Do not queue it; after reconnect, the plugin
//      must pull state with GET. This follows blueprint §1.5: the runtime
//      must not cache it.
//
// Constraints (spec §2):
//   - Admin callers must not invoke this method; ADM-0 §1.3 keeps admin flows
//     out of the business path. The AP-1 abac.go::HasCapability false path
//     must first confirm user.Role != "admin". This method does not perform
//     ACL checks, matching PushArtifactUpdated.
//   - Plugins must never send permission_denied. bppEnvelopeWhitelist and the
//     reflection lint both enforce the server-to-plugin direction.
//   - HTTP 403 is the fallback response; the BPP frame is the primary signal
//     per blueprint §2.

package ws

import (
	"borgee-server/internal/bpp"
)

// PermissionDeniedPusher is the boundary between the api package and ws.Hub
// for the BPP-3.1 permission_denied frame (mirrors AgentConfigPusher
// pattern in api/agent_config.go so the api package doesn't import
// internal/ws). AP-1 (#493) abac.go::HasCapability false path will wire
// this with a one-line follow-up after AP-1 and BPP-3.1 both merge.
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
//     test hook).
//   - sent: true iff plugin connection exists for agentID AND frame
//     enqueued to its send channel. false otherwise (plugin offline /
//     no allocator / channel buffer full).
//
// Frame field assignment must match bpp.PermissionDeniedFrame
// (spec §1 design ①, 8 fields); reordering arguments here without updating
// the frame struct is a CI failure caught by frame_schemas_test.go reflection
// lint.
//
// Caller responsibilities:
//   - requestID: AP-1 调用方生成的 trace UUID, plugin 端按此 key 关联
//     owner DM approval notification + retry (BPP-3.2 follow-up).
//   - attemptedAction: must be one of the seven BPP-2.1 operation allow-list
//     values (`bpp.SemanticOp*` const) or a REST endpoint name. Values outside
//     the v2+ enum must not reach this path.
//   - requiredCapability / currentScope: must remain in sync with the
//     AP-1 abac.go 403 body. Any change must update all three references:
//     blueprint §4.1, AP-1, and BPP-3.1.
//   - deniedAt: Unix-ms semantic timestamp. Cursor remains the ordering source.
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
