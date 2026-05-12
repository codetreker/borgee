// Package ws — al_2b_2_agent_config_push.go: AL-2b.2 hub method for
// emitting AgentConfigUpdateFrame to the target agent's plugin
// connection (server→plugin direction lock).
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §1.5
// (hot-reload levels + idempotent reload + runtime does not cache) + §2.1
// (control-plane row `agent_config_update`).
// Spec: AL-2b acceptance #452 §2.1 + AL-2b.1 frames PR #472 (BPP envelope
// 7+7 field wire layout).
//
// Behaviour contract — follows the same wire pattern as RT-1.1 PushArtifactUpdated /
// CV-2.2 PushAnchorCommentAdded / DM-2.2 PushMentionPushed / CV-4.2
// PushIterationStateChanged:
//
//   1. Cursor uses hub.cursors.NextCursor() and shares one sequence with the
//      RT-1/CV-2/DM-2/CV-4 frames (acceptance §2.1: no plugin-only push channel);
//      AL-2b is the 5th shared-sequence frame.
//   2. Direction lock = server→plugin; only send to the target agent's PluginConn
//      (h.plugins[agentID]), not broadcast (separate from channel-scoped frames;
//      acceptance §2.1 says plugin receives within ≤1s).
//   3. Field order contract: type/cursor/agent_id/schema_version/blob/idempotency_key/
//      created_at — covered by BPP-1 #304 envelope CI lint reflect checks
//      (al_2b_frames_test.go::TestAL2B1_AgentConfigUpdate7Fields).
//   4. Idempotent reload (acceptance §2.2): caller decides idempotencyKey; the
//      server side is only wire transport, and the plugin dedups reload by
//      idempotencyKey.
//      This hub method does not do server-side dedup (same stateless pattern as
//      the BPP-1 frame layer — state lives in store/agent_configs.schema_version,
//      not in hub).
//
// Negative constraints:
//   - Admin routes do not call this method (ADM-0 §1.3 — admin is outside business paths).
//     The caller (AL-2a PATCH /config handler or follow-up) must perform the
//     owner-only ACL gate first. This method does not perform ACL checks, matching
//     PushArtifactUpdated where the caller decides broadcast permissions.
//   - sent=true is not returned when the plugin is offline. This differs from
//     RT-1: RT-1 frames enter channel broadcast for every channel member, while
//     AL-2b is point-to-point server→plugin and drops the frame when the plugin
//     is offline (constraint:
//     do not queue — plugin reconnects and GET /agents/:id/config pulls latest, matching
//     blueprint §1.5 "runtime does not cache" wording).

package ws

import (
	"strconv"
	"time"

	"borgee-server/internal/bpp"
)

// PushAgentConfigUpdate emits an AgentConfigUpdateFrame to the target
// agent's plugin connection. Returns (cursor, sent):
//
//   - cursor: hub.cursors monotonic sequence number (0 if no allocator,
//     test seam).
//   - sent: true iff plugin connection exists for agentID AND frame
//     enqueued to its send channel. false otherwise (plugin offline /
//     no allocator / channel buffer full).
//
// Frame field assignment must match bpp.AgentConfigUpdateFrame
// (AL-2b.1 PR #472 + acceptance §1.1 7 fields); reordering arguments here
// without updating the frame struct is a CI red caught by
// al_2b_frames_test.go reflect lint.
//
// Caller responsibilities:
//   - blob: pre-marshalled JSON of SSOT allowed fields (acceptance §3.2).
//     Server-side validation lives in AL-2a PATCH handler (allowedConfigKeys
//     allow-list fail-closed); this method trusts the input.
//   - idempotencyKey: stable per-PATCH key the plugin uses to dedup reload
//     (acceptance §2.2); typical impl is `agent_id + ":" + schema_version`
//     or a request-scoped uuid (no constraint here — plugin contract).
//   - schemaVersion: monotonic from agent_configs.schema_version (AL-2a
//     #447 v=20 server-stamp).
//   - createdAt: Unix ms semantic timestamp. Negative constraint: cursor is the
//     ordering source; this field is an audit hint and follows the
//     IterationStateChangedFrame.CompletedAt semantic pattern.
func (h *Hub) PushAgentConfigUpdate(
	agentID string,
	schemaVersion int64,
	blob string,
	idempotencyKey string,
	createdAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()

	frame := bpp.AgentConfigUpdateFrame{
		Type:           bpp.FrameTypeBPPAgentConfigUpdate,
		Cursor:         cur,
		AgentID:        agentID,
		SchemaVersion:  schemaVersion,
		Blob:           blob,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
	}

	// Look up the plugin connection. h.GetPlugin RLock-guards the map.
	pc := h.GetPlugin(agentID)
	if pc == nil {
		// Plugin offline — frame dropped. Per blueprint §1.5 "runtime does not
		// cache", reconnect time triggers GET /agents/:id/config pull.
		//
		// BPP-4.2 dead-letter audit log: log warn `bpp.frame_dropped_plugin_offline`
		// with the same 5-field schema as HB-1/HB-2 audit (acceptance
		// §2.2 + content-lock §1.③). Constraint: do not use a persistent queue or
		// timer retry — RT-1.3 #296 cursor replay is the fallback (plugin pulls missing
		// frame after reconnect). Acceptance §4.3 reverse grep expects 0 hits for
		// `pendingAcks|retryQueue|deadLetterQueue`.
		bpp.LogFrameDroppedPluginOffline(h.logger, bpp.DeadLetterAuditEntry{
			Actor:  "server",
			Action: "frame_drop",
			Target: agentID,
			When:   createdAt,
			Scope:  bpp.FrameTypeBPPAgentConfigUpdate + ":cursor=" + strconv.FormatInt(cur, 10),
		})
		return cur, false
	}

	pc.sendJSON(frame)
	return cur, true
}

// NewTestPluginConn constructs a minimal PluginConn for in-process tests
// (al_2b_2_agent_config_push_test.go). Returns a *PluginConn with a
// buffered send channel that tests can drain to assert wire JSON.
//
// Not exported in production code path — production PluginConn comes from
// HandlePlugin (websocket Accept). This shim avoids the network for unit
// tests; mirrors patterns used in cursor_test.go fakeAllocator stubs.
//
// Buffer size matches sendBufSize (256) so fast-path bound-checking
// tests don't false-positive.
func NewTestPluginConn(agentID string) *PluginConn {
	return &PluginConn{
		agentID:    agentID,
		send:       make(chan []byte, sendBufSize),
		done:       make(chan struct{}),
		alive:      true,
		lastSeenAt: time.Now(),
		pending:    make(map[string]chan PluginResponse),
	}
}

// DrainSend returns the next pending wire-JSON message off the plugin's
// send channel, or "" + ok=false if nothing buffered. Test helper paired
// with NewTestPluginConn.
func (pc *PluginConn) DrainSend() (string, bool) {
	select {
	case data := <-pc.send:
		return string(data), true
	default:
		return "", false
	}
}
