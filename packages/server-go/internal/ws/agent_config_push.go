// Package ws — al_2b_2_agent_config_push.go: AL-2b.2 hub method for
// emitting AgentConfigUpdateFrame to the target agent's plugin
// connection (server→plugin direction lock).
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.5 (热更新分级 + 幂等
// reload + runtime 不缓存) + §2.1 (control-plane row `agent_config_update`).
// Spec: AL-2b acceptance #452 §2.1 + AL-2b.1 frames PR #472 (BPP envelope
// 7+7 字段 byte-identical).
//
// Behaviour contract — byte-identical 跟 RT-1.1 PushArtifactUpdated /
// CV-2.2 PushAnchorCommentAdded / DM-2.2 PushMentionPushed / CV-4.2
// PushIterationStateChanged 同模式:
//
//   1. Cursor 走 hub.cursors.NextCursor() 单调发号, 跟 RT-1/CV-2/DM-2/CV-4
//      4-frame 共一根 sequence (acceptance §2.1 反约束: 不另起 plugin-only
//      推送通道); AL-2b 是第 5 个共序 frame.
//   2. Direction lock = server→plugin; 只发给目标 agent 的 PluginConn
//      (h.plugins[agentID]), 不 broadcast (跟 channel-scoped frames 拆死;
//      acceptance §2.1 字面 plugin 收到 ≤1s).
//   3. 字段顺序锁: type/cursor/agent_id/schema_version/blob/idempotency_key/
//      created_at — 跟 BPP-1 #304 envelope CI lint reflect 自动覆盖
//      (al_2b_frames_test.go::TestAL2B1_AgentConfigUpdate7Fields 守).
//   4. 幂等 reload (acceptance §2.2): caller 决定 idempotencyKey, server
//      端只是 wire transport — plugin 端按 idempotencyKey 去重 reload.
//      此 hub 方法不做 server-side dedup (反约束: 跟 BPP-1 frame layer
//      stateless 同模式 — state 在 store/agent_configs.schema_version,
//      不在 hub).
//
// 反约束:
//   - admin god-mode 不调此方法 (ADM-0 §1.3 红线 — admin 不入业务路径).
//     调用方 (AL-2a PATCH /config handler 或 follow-up) 必须先做 owner-only
//     ACL gate. 此方法不做 ACL — 跟 PushArtifactUpdated 同模式 (broadcast
//     由调用方决定权限).
//   - 不返 sent=true 当 plugin 离线 — 这是 AL-2b 跟 RT-1 不同的语义:
//     RT-1 frame 进 channel broadcast 任何 channel member 都收, AL-2b
//     frame 是点对点 server→plugin, plugin 离线时 frame 丢弃 (反约束:
//     不入队列 — plugin 重连后 GET /agents/:id/config 主动拉最新, 跟
//     蓝图 §1.5 字面 "runtime 不缓存" 同源).

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
// Frame field assignment is byte-identical with bpp.AgentConfigUpdateFrame
// (AL-2b.1 PR #472 + acceptance §1.1 7 字段); reordering arguments here
// without updating the frame struct is a CI red caught by
// al_2b_frames_test.go reflect lint.
//
// Caller responsibilities:
//   - blob: pre-marshalled JSON of SSOT-whitelisted fields (acceptance §3.2).
//     Server-side validation lives in AL-2a PATCH handler (allowedConfigKeys
//     whitelist fail-closed); this method trusts the input.
//   - idempotencyKey: stable per-PATCH key the plugin uses to dedup reload
//     (acceptance §2.2); typical impl is `agent_id + ":" + schema_version`
//     or a request-scoped uuid (no constraint here — plugin contract).
//   - schemaVersion: monotonic from agent_configs.schema_version (AL-2a
//     #447 v=20 server-stamp).
//   - createdAt: Unix ms semantic timestamp (反约束: cursor 才是排序源,
//     此字段是 audit hint; 跟 IterationStateChangedFrame.CompletedAt 同
//     语义模式).
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
		// Plugin offline — frame dropped. Per 蓝图 §1.5 字面 "runtime 不
		// 缓存", reconnect time triggers GET /agents/:id/config pull.
		//
		// BPP-4.2 dead-letter audit log: log warn `bpp.frame_dropped_plugin_offline`
		// with 5-field schema byte-identical 跟 HB-1/HB-2 audit (acceptance
		// §2.2 + content-lock §1.③). 反约束: **不**入持久队列, 不挂 timer
		// 重发 — RT-1.3 #296 cursor replay 兜底 (plugin 重连后主动拉缺失
		// frame). Acceptance §4.3 反向 grep `pendingAcks|retryQueue|
		// deadLetterQueue` 0 hit 守门.
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
