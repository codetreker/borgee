// Package bpp — envelope.go: BPP-1 (#274/#280) source-of-truth for the
// 9 envelope frames defined in docs/blueprint/current/plugin-protocol.md §2.1
// (control plane, Borgee→Plugin) + §2.2 (data plane, Plugin→Borgee).
//
// Layout contract — BPP-1 envelope is byte-identical with RT-0 (#237)
// envelope on the discriminator + payload-first-field convention:
//
//   - Field 0 is `Type` tagged `json:"type"` — the wire dispatcher
//     matches on this exactly like RT-0 (`AgentInvitationPendingFrame`)
//     and RT-1.1 (`ArtifactUpdatedFrame`) do.
//   - Subsequent fields are payload, ordered by semantic weight (IDs
//     first, then timestamps / counters). No `version` field on the
//     frame itself — protocol version is negotiated on `connect` once.
//   - There is NO `timestamp` ordering field; the cursor (or, for
//     control-plane fan-out, the server's monotonic seq) IS the order.
//
// Direction lock — per §2.1 / §2.2 headings, every frame in this file
// has a hard direction lock enforced by FrameDirection() below + the
// reflection lint in frame_schemas_test.go. A mismatch fails CI.
//
// Whitelist — only the 9 OpName constants enumerated in
// `bppEnvelopeWhitelist` are permitted. Adding a frame here without a
// matching blueprint row fails CI (TestBPPEnvelopeFrameWhitelist).
//
// Negative constraint — this file MUST NOT contain any `replay_mode = "full"`
// default, `defaultReplayMode` symbol, or `default.*ResumeModeFull`
// branch (RT-1.3 fail-closed replay behavior). The reverse grep step in
// the bpp-envelope-lint workflow enforces 0 hits across this package
// (excluding _test.go).

package bpp

// Frame `type` discriminator strings on the BPP-1 wire. These are the
// only OpName constants the envelope lint accepts; the lint asserts
// each control-plane / data-plane registry below has exactly the
// expected length and that `bppEnvelopeWhitelist` covers them all.
const (
	// Control plane (Server → Plugin) — §2.1.
	FrameTypeBPPConnect                = "connect"
	FrameTypeBPPAgentRegister          = "agent_register"
	FrameTypeBPPRuntimeSchemaAdvertise = "runtime_schema_advertise"
	FrameTypeBPPAgentConfigUpdate      = "agent_config_update"
	FrameTypeBPPAgentToggle            = "agent_toggle" // disable/enable: one frame, action field
	FrameTypeBPPInboundMessage         = "inbound_message"
	// BPP-3.1 permission_denied — server notifies plugin of authz failure.
	// Server-to-plugin only; plugin never sends it. Payload fields are
	// byte-identical with the AP-1 abac.go 403 body.
	FrameTypeBPPPermissionDenied = "permission_denied"

	// Data plane (Plugin → Server) — §2.2.
	// Data plane (Plugin → Server) — §2.2.
	FrameTypeBPPHeartbeat      = "heartbeat"
	FrameTypeBPPSemanticAction = "semantic_action"
	FrameTypeBPPErrorReport    = "error_report"
	FrameTypeBPPAgentConfigAck = "agent_config_ack" // AL-2b #481 §1.2 ack 路径
	// BPP-2.2 task lifecycle reverse-channel — plugin upstream signals
	// agent busy/idle. The source must be the plugin upstream frame, not a stub.
	// online is session-level WebSocket lifecycle; busy is task-level state.
	FrameTypeBPPTaskStarted  = "task_started"
	FrameTypeBPPTaskFinished = "task_finished"

	// BPP-5 plugin reconnect handshake — plugin upstream signals
	// reconnect-with-cursor. Reconnect is distinct from connect: connect carries
	// initial identity + capabilities, while reconnect carries last_known_cursor.
	// Cursor resume reuses RT-1.3 #296 (bpp.ResolveResume + Mode=incremental,
	// AfterCursor=last_known_cursor). State uses the existing AL-1 error→online
	// valid edge; there is no persisted "connecting" intermediate state.
	FrameTypeBPPReconnectHandshake = "reconnect_handshake"

	// BPP-6 plugin cold-start handshake — plugin upstream signals process
	// restart, with state lost and no cursor. Cold-start is distinct from
	// reconnect: reconnect has last_known_cursor and uses ResolveResume, while
	// cold-start clears agent.Tracker and appends any→online through AL-1 #492.
	// The reason reuses `runtime_crashed` from the byte-identical reason set.
	// Cold-start intentionally omits LastKnownCursor / DisconnectAt / ReconnectAt.
	FrameTypeBPPColdStartHandshake = "cold_start_handshake"
)

// Direction is the hard direction lock the lint enforces.
type Direction string

const (
	// DirectionServerToPlugin — every control-plane envelope.
	DirectionServerToPlugin Direction = "server_to_plugin"
	// DirectionPluginToServer — every data-plane envelope.
	DirectionPluginToServer Direction = "plugin_to_server"
)

// BPPEnvelope is the marker every BPP-1 envelope struct implements. The
// reflection lint walks all exported structs in this file and asserts
// each one returns a non-empty FrameType + a valid Direction.
type BPPEnvelope interface {
	FrameType() string
	FrameDirection() Direction
}

// ----- Control plane (Server → Plugin) — 6 envelopes -----

// ConnectFrame — handshake. Sent first when a plugin opens the BPP
// socket. Carries the auth token + the protocol version the plugin
// supports; server replies with its version on the same frame type
// (per §2.1, the row is one frame, the direction tag below pins the
// outbound side).
type ConnectFrame struct {
	Type         string `json:"type"`
	PluginID     string `json:"plugin_id"`
	Token        string `json:"token"`
	Version      string `json:"version"` // protocol version, e.g. "bpp-1"
	Capabilities string `json:"capabilities"`
}

func (ConnectFrame) FrameType() string         { return FrameTypeBPPConnect }
func (ConnectFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// AgentRegisterFrame — multi-agent registration (§1.1). One plugin
// connection hosts N agents; this frame carries the list.
type AgentRegisterFrame struct {
	Type     string   `json:"type"`
	PluginID string   `json:"plugin_id"`
	AgentIDs []string `json:"agent_ids"`
}

func (AgentRegisterFrame) FrameType() string         { return FrameTypeBPPAgentRegister }
func (AgentRegisterFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// RuntimeSchemaAdvertiseFrame — runtime declares its model list +
// opaque blob keys (§1.4 "Runtime 上报 model schema"). The server
// stores the schema verbatim; UI renders generic select widgets.
type RuntimeSchemaAdvertiseFrame struct {
	Type      string `json:"type"`
	PluginID  string `json:"plugin_id"`
	Models    string `json:"models"`    // JSON-encoded list; opaque to server
	BlobKeys  string `json:"blob_keys"` // JSON-encoded list of allowed keys
	SchemaVer int    `json:"schema_ver"`
}

func (RuntimeSchemaAdvertiseFrame) FrameType() string {
	return FrameTypeBPPRuntimeSchemaAdvertise
}
func (RuntimeSchemaAdvertiseFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// AgentConfigUpdateFrame — server pushes a config delta (§1.5).
//
// AL-2b (#460 BPP-2 base + AL-2b acceptance #452 §1.1) extended this from
// the BPP-1 4-field stub (Type / AgentID / ConfigRev / Payload) to the
// 7-field byte-identical envelope per acceptance §1.1:
//
//	{Type, Cursor, AgentID, SchemaVersion, Blob, IdempotencyKey, CreatedAt}
//
// Field order is the contract. Do NOT reorder without updating
// schema_equivalence_test.go + acceptance al-2b.md §1.1 simultaneously.
//
// Field semantics:
//   - Type: discriminator first field, byte-identical with BPP-1 envelope (#280)
//   - Cursor: hub.cursors atomic int64 monotonic allocation, sharing one
//     sequence with RT-1 #290 + CV-2.2 #360 + DM-2.2 #372 + CV-4.2 #416 +
//     AL-2b's 5 source frames. RT-1 spec §1.1 negative constraint: do not add a
//     plugin-only channel; principle "不另起 channel" remains byte-identical
//     with acceptance §2.1.
//   - AgentID: target agent UUID
//   - SchemaVersion: monotonic with agent_configs.schema_version (AL-2a v=20
//     #447), byte-identical; plugin receives < current server value → ack
//     `status=stale`
//     (acceptance §2.3)
//   - Blob: serialized single-source fields (name/avatar/prompt/model/
//     capability switches/enabled state/memory_ref). Negative constraint: do
//     not include runtime-only fields api_key/temperature/token_limit/
//     retry_policy (acceptance §3.2 + AL-2a #447 single source).
//   - IdempotencyKey: stable server-generated key; resending the same key
//     triggers plugin reload only once (acceptance §2.2 + blueprint §1.5
//     literal "幂等 reload")
//   - CreatedAt: Unix-ms semantic timestamp. Negative constraint: do not use
//     it as the ordering source; cursor is the ordering source, matching
//     IterationStateChangedFrame.CompletedAt semantics.
//
// Plugin MUST reload idempotently; same payload pushed twice is a no-op.
type AgentConfigUpdateFrame struct {
	Type           string `json:"type"`
	Cursor         int64  `json:"cursor"`
	AgentID        string `json:"agent_id"`
	SchemaVersion  int64  `json:"schema_version"`
	Blob           string `json:"blob"` // JSON-encoded single-source delta; opaque on the wire
	IdempotencyKey string `json:"idempotency_key"`
	CreatedAt      int64  `json:"created_at"` // Unix ms; semantic only — cursor IS the order
}

func (AgentConfigUpdateFrame) FrameType() string         { return FrameTypeBPPAgentConfigUpdate }
func (AgentConfigUpdateFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// AgentToggleFrame — pause / resume an agent's inbound (§2.1
// `agent_disable / enable` row). One frame; `Action` is "disable" or
// "enable".
type AgentToggleFrame struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	Action  string `json:"action"` // "disable" | "enable"
	Reason  string `json:"reason"`
}

func (AgentToggleFrame) FrameType() string         { return FrameTypeBPPAgentToggle }
func (AgentToggleFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// InboundMessageFrame — server pushes a new channel message at the
// agent (§2.1 `inbound_message`).
type InboundMessageFrame struct {
	Type      string `json:"type"`
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id"`
	MessageID string `json:"message_id"`
	AuthorID  string `json:"author_id"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"` // Unix ms
}

func (InboundMessageFrame) FrameType() string         { return FrameTypeBPPInboundMessage }
func (InboundMessageFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// PermissionDeniedFrame — BPP-3.1 server notifies plugin of authz failure.
// Blueprint auth-permissions.md §2 invariant: permission denial is sent through
// BPP rather than HTTP error codes, and the protocol layer routes it to the
// owner DM. Also see §4.1 row listing the exact frame fields:
// `attempted_action`, `required_capability`, `current_scope`, `reason`).
//
// 8 fields, byte-identical with spec bpp-3.1 §1 principle ③:
//
//	{Type, Cursor, AgentID, RequestID, AttemptedAction, RequiredCapability, CurrentScope, DeniedAt}
//
// Field semantics:
//   - Type: discriminator first field, byte-identical with BPP envelope (#280)
//   - Cursor: hub.cursors monotonic allocation, sharing one sequence with
//     RT-1/CV-2/DM-2/CV-4/AL-2b. Negative constraint: do not add a plugin-only
//     push channel.
//   - AgentID: target agent UUID (deny 路径 plugin 端按 agent 分流)
//   - RequestID: AP-1 caller-generated trace UUID; plugin uses this key to
//     link owner DM approval notification + retry flow (BPP-3.2 follow-up)
//   - AttemptedAction: one of the BPP-2.1 7 op allow-list values
//     (`SemanticOp*` const) or a REST endpoint name (e.g.
//     "POST /artifacts/:id/commits"). Negative constraint: reject v2+ enum-out
//     values such as 'list_users'.
//   - RequiredCapability: byte-identical with the AP-1 abac.go 403 body field
//     (e.g. "commit_artifact" shares the AP-1 capabilities.go const; drift
//     fails the bidirectional grep CI lint)
//   - CurrentScope: byte-identical with the AP-1 abac.go 403 body field (e.g.
//     "artifact:art-1" shares AP-1 ArtifactScopeStr)
//   - DeniedAt: Unix-ms semantic timestamp. Negative constraint: do not use it
//     as the ordering source; cursor is the ordering source, matching
//     IterationStateChangedFrame.CompletedAt semantics.
//
// Negative constraints (spec bpp-3.1 §2):
//   - direction = server→plugin hard-locked; plugins must never send this
//     frame. bppEnvelopeWhitelist and reflection lint both enforce it.
//   - Admin users do not consume this frame. Admin flows use /admin-api/* and
//     do not enter the business path, per ADM-0 §1.3.
//   - HTTP 403 is the fallback; the BPP frame is the primary signal, per
//     blueprint §2 invariant.
type PermissionDeniedFrame struct {
	Type               string `json:"type"`
	Cursor             int64  `json:"cursor"`
	AgentID            string `json:"agent_id"`
	RequestID          string `json:"request_id"`
	AttemptedAction    string `json:"attempted_action"`
	RequiredCapability string `json:"required_capability"`
	CurrentScope       string `json:"current_scope"`
	DeniedAt           int64  `json:"denied_at"` // Unix ms; semantic only — cursor IS the order
}

func (PermissionDeniedFrame) FrameType() string         { return FrameTypeBPPPermissionDenied }
func (PermissionDeniedFrame) FrameDirection() Direction { return DirectionServerToPlugin }

// ----- Data plane (Plugin → Server) — 3 envelopes -----

// HeartbeatFrame — plugin liveness + per-agent state (§1.6 + §2.2).
// `Status` is one of "online" / "working" / "offline" — matches the
// AL-1a three-state runtime registry (PR #249).
type HeartbeatFrame struct {
	Type      string `json:"type"`
	PluginID  string `json:"plugin_id"`
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`    // empty when status==online; reason code per AL-1a
	Timestamp int64  `json:"timestamp"` // Unix ms (semantic only — server cursor IS the order)
}

func (HeartbeatFrame) FrameType() string         { return FrameTypeBPPHeartbeat }
func (HeartbeatFrame) FrameDirection() Direction { return DirectionPluginToServer }

// SemanticActionFrame — collaborative-intent action (§1.3). The
// `Action` field carries one of the v1 whitelisted verbs
// (create_artifact / update_artifact / reply_in_thread / mention_user /
// request_agent_join / read_channel_history / read_artifact). Server
// dispatches to the matching REST handler with permission checks.
type SemanticActionFrame struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	Action  string `json:"action"`
	Payload string `json:"payload"` // JSON-encoded action args; opaque on the wire
	Nonce   string `json:"nonce"`   // idempotency key
}

func (SemanticActionFrame) FrameType() string         { return FrameTypeBPPSemanticAction }
func (SemanticActionFrame) FrameDirection() Direction { return DirectionPluginToServer }

// ErrorReportFrame — plugin proactively reports an agent fault
// (§1.6 "故障 UX 区分"). `Kind` is "runtime_disconnected" or
// "agent_misconfigured" so the UI can route the user to the right
// remediation path.
type ErrorReportFrame struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
}

func (ErrorReportFrame) FrameType() string         { return FrameTypeBPPErrorReport }
func (ErrorReportFrame) FrameDirection() Direction { return DirectionPluginToServer }

// AgentConfigAckFrame — plugin acknowledges receipt + apply outcome of an
// AgentConfigUpdateFrame (AL-2b acceptance #452 §1.2 + blueprint §1.5
// idempotent reload). Direction is hard-locked plugin→server: this frame must
// not use DirectionServerToPlugin, matching the BPP-1 #304 direction-lock
// pattern.
//
// 7 字段 byte-identical 跟 acceptance §1.2:
//
//	{Type, Cursor, AgentID, SchemaVersion, Status, Reason, AppliedAt}
//
// Field semantics:
//   - Type: discriminator first field, byte-identical with BPP envelope #280
//   - Cursor: plugin echoes update.Cursor for pairing. The server pairs ack ↔
//     AgentConfigUpdateFrame by cursor; the ack itself does not use
//     hub.cursors monotonic allocation because it is a plugin → server receipt,
//     not the same sequence as the server → plugin push cursor.
//   - AgentID: target agent UUID, byte-identical with the update frame
//   - SchemaVersion: schema_version actually applied by the plugin. In the
//     acceptance §2.3 stale path, plugin receives < current server value → ack
//     carries the plugin-known value, and the server treats it as stale so the
//     plugin actively pulls.
//   - Status: 'applied' | 'rejected' | 'stale' (acceptance §1.2 CHECK
//     enum byte-identical; negative constraint: reject enum-out values such as
//     'unknown', with server-side validation fail-closed)
//   - Reason: set for stale/rejected. It is byte-identical with the AL-1a #249
//     6-reason enum — api_key_invalid/quota_exceeded/network_unreachable/
//     runtime_crashed/runtime_timeout/unknown. For applied, it is empty string.
//     Negative constraint: do not add omitempty; always serialize it, matching
//     IterationStateChangedFrame.ErrorReason.
//   - AppliedAt: Unix-ms plugin actual reload completion timestamp
//     (acceptance §2.2 idempotent reload). applied uses the actual value;
//     stale/rejected use 0.
//
// Negative constraints (acceptance §3.2 + §4.2):
//   - Do not add ordering fields beyond cursor. Reverse grep for
//     sort.AgentConfigAck.time / timestamp must return 0 hits. This matches
//     the RT-1 principle: cursor is the only trusted order.
//   - Do not send admin override acknowledgements. Admins do not enter the
//     business path per ADM-0 §1.3, and reverse grep
//     `admin.*AgentConfig.*ack` must return 0 hits.
type AgentConfigAckFrame struct {
	Type          string `json:"type"`
	Cursor        int64  `json:"cursor"`
	AgentID       string `json:"agent_id"`
	SchemaVersion int64  `json:"schema_version"`
	Status        string `json:"status"` // 'applied'|'rejected'|'stale'
	Reason        string `json:"reason"`
	AppliedAt     int64  `json:"applied_at"` // Unix ms; 0 when stale/rejected
}

func (AgentConfigAckFrame) FrameType() string         { return FrameTypeBPPAgentConfigAck }
func (AgentConfigAckFrame) FrameDirection() Direction { return DirectionPluginToServer }

// AgentConfigAck status enum is byte-identical with acceptance §1.2 CHECK plus
// server-side fail-closed validation; enum-out values reject.
const (
	AgentConfigAckStatusApplied  = "applied"
	AgentConfigAckStatusRejected = "rejected"
	AgentConfigAckStatusStale    = "stale"
)

// TaskStartedFrame — BPP-2.2 plugin signals agent has started a task. §1.6 and
// agent-lifecycle.md §2.3 require busy/idle to come from a plugin upstream
// frame; stubs must be removed before v1. The `Subject` field is the
// human-readable description ("agent 在做什么"). The server rejects empty or
// whitespace-only Subject and logs `bpp.task_subject_empty` (spec §0 stance ②:
// default-value fallback is forbidden).
type TaskStartedFrame struct {
	Type      string `json:"type"`
	TaskID    string `json:"task_id"`
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id"`
	Subject   string `json:"subject"`
	StartedAt int64  `json:"started_at"` // Unix ms (semantic only — server cursor IS the order)
}

func (TaskStartedFrame) FrameType() string         { return FrameTypeBPPTaskStarted }
func (TaskStartedFrame) FrameDirection() Direction { return DirectionPluginToServer }

// TaskFinishedFrame — BPP-2.2 plugin signals task termination. `Outcome` is one
// of the 3 enum values ('completed' / 'failed' / 'cancelled'). When 'failed',
// `Reason` MUST be one of the AL-1a #249 6-dict reasons (api_key_invalid /
// quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout /
// unknown). This shares the AL-3 #305, AL-4 #321, and #427 test locks; BPP-2.2
// is the fourth lock. Negative constraint: reject intermediate states such as
// 'partial' / 'paused' / 'pending' / 'starting'.
type TaskFinishedFrame struct {
	Type       string `json:"type"`
	TaskID     string `json:"task_id"`
	AgentID    string `json:"agent_id"`
	ChannelID  string `json:"channel_id"`
	Outcome    string `json:"outcome"`
	Reason     string `json:"reason"`      // empty unless outcome=='failed'
	FinishedAt int64  `json:"finished_at"` // Unix ms
}

func (TaskFinishedFrame) FrameType() string         { return FrameTypeBPPTaskFinished }
func (TaskFinishedFrame) FrameDirection() Direction { return DirectionPluginToServer }

// ReconnectHandshakeFrame — BPP-5 plugin reconnect handshake.
//
// Direction lock plugin→server. Sent when a plugin reopens the BPP
// socket AFTER an earlier connect (BPP-1 #304) was disconnected. The
// frame carries `last_known_cursor` so the server can resume the
// shared event sequence (RT-1.3 #296 ResolveResume incremental mode);
// agents are scoped from the plugin connection's authenticated user
// (BPP-1 connect handshake), so this frame doesn't re-authenticate.
//
// 6 fields, byte-identical with spec brief §1 BPP-5.1:
//
//	{Type, PluginID, AgentID, LastKnownCursor, DisconnectAt, ReconnectAt}
//
// Negative constraints (matching spec §0 + principle §1):
//   - **Do not reuse ConnectFrame**. connect carries Token + Capabilities for
//     initial identity; reconnect carries last_known_cursor for recovery. Their
//     field sets are disjoint.
//   - **Do not add another channel/sub_protocol**. This is a single BPP envelope
//     frame and reuses the BPP-3 dispatcher via PluginFrameDispatcher.
//   - **cursor resume reuses RT-1.3**. The server handler calls
//     bpp.ResolveResume(SessionResumeRequest{Mode: incremental,
//     AfterCursor: LastKnownCursor}, …). Do not add another sequence.
//   - Field order is fixed: type/plugin_id/agent_id/last_known_cursor/
//     disconnect_at/reconnect_at. The BPP-1 #304 envelope CI reflection lint
//     covers this.
type ReconnectHandshakeFrame struct {
	Type            string `json:"type"`
	PluginID        string `json:"plugin_id"`
	AgentID         string `json:"agent_id"`
	LastKnownCursor int64  `json:"last_known_cursor"`
	DisconnectAt    int64  `json:"disconnect_at"` // Unix ms
	ReconnectAt     int64  `json:"reconnect_at"`  // Unix ms
}

func (ReconnectHandshakeFrame) FrameType() string         { return FrameTypeBPPReconnectHandshake }
func (ReconnectHandshakeFrame) FrameDirection() Direction { return DirectionPluginToServer }

// ColdStartHandshakeFrame — BPP-6 plugin cold-start handshake.
//
// Direction lock plugin→server. Sent when a plugin process is RESTARTED
// (e.g. after SIGKILL/crash) — state is fully lost, no last_known_cursor
// available. Reverse of BPP-5 ReconnectHandshakeFrame:
//   - reconnect (BPP-5): socket dropped, plugin process alive, holds cursor
//   - cold-start (BPP-6): process died, fresh start, no cursor
//
// Server handler reaction (cold_start_handler.go):
//  1. agent.Tracker.Clear(agentID) — drop in-memory state
//  2. Store.AppendAgentStateTransition(agentID, fromState, online,
//     runtime_crashed, "") — AL-1 #492 single-gate writes state-log row
//  3. NO history replay. This is the opposite of BPP-5: cold-start is a fresh start.
//
// 5 fields, byte-identical with spec brief §1 BPP-6.1:
//
//	{Type, PluginID, AgentID, RestartAt, RestartReason}
//
// Negative constraints (matching spec §0 + principle §1):
//   - **Field set is disjoint from ReconnectHandshakeFrame**. It does not carry
//     LastKnownCursor / DisconnectAt / ReconnectAt. spec §0.1.
//   - **Do not add another channel/sub_protocol**. This is a single BPP envelope
//     frame and reuses the BPP-3 dispatcher via PluginFrameDispatcher.
//   - **Do not replay historical frames**. The handler does not call
//     ResolveResume and does not carry cursor. spec §0.2 (AST scan).
//   - **Do not add a plugin_restart_count column**. Restart count is derived
//     from state-log COUNT(WHERE to_state='online' AND reason='runtime_crashed').
//     spec §0.3.
//   - reason reuses `runtime_crashed` from the byte-identical 6-dict to express
//     previous error → current recovery. reasons single source #496 does not
//     add a 7th literal.
//   - Field order is fixed: type/plugin_id/agent_id/restart_at/restart_reason.
type ColdStartHandshakeFrame struct {
	Type          string `json:"type"`
	PluginID      string `json:"plugin_id"`
	AgentID       string `json:"agent_id"`
	RestartAt     int64  `json:"restart_at"`     // Unix ms
	RestartReason string `json:"restart_reason"` // e.g. "sigkill", "panic", "oom"; opaque to server, audit-only
}

func (ColdStartHandshakeFrame) FrameType() string         { return FrameTypeBPPColdStartHandshake }
func (ColdStartHandshakeFrame) FrameDirection() Direction { return DirectionPluginToServer }

// bppEnvelopeWhitelist is the single source of truth for permitted BPP-1
// envelope OpNames. The reflection lint asserts every exported
// frame struct in this file maps to exactly one entry here and
// vice-versa (no orphans, no extras). Adding a row here without a
// matching blueprint §2 entry is a CI red.
var bppEnvelopeWhitelist = map[string]Direction{
	FrameTypeBPPConnect:                DirectionServerToPlugin,
	FrameTypeBPPAgentRegister:          DirectionServerToPlugin,
	FrameTypeBPPRuntimeSchemaAdvertise: DirectionServerToPlugin,
	FrameTypeBPPAgentConfigUpdate:      DirectionServerToPlugin,
	FrameTypeBPPAgentToggle:            DirectionServerToPlugin,
	FrameTypeBPPInboundMessage:         DirectionServerToPlugin,
	FrameTypeBPPPermissionDenied:       DirectionServerToPlugin, // BPP-3.1
	FrameTypeBPPHeartbeat:              DirectionPluginToServer,
	FrameTypeBPPSemanticAction:         DirectionPluginToServer,
	FrameTypeBPPErrorReport:            DirectionPluginToServer,
	FrameTypeBPPAgentConfigAck:         DirectionPluginToServer, // AL-2b #481
	FrameTypeBPPTaskStarted:            DirectionPluginToServer, // BPP-2.2 #485
	FrameTypeBPPTaskFinished:           DirectionPluginToServer, // BPP-2.2 #485
	FrameTypeBPPReconnectHandshake:     DirectionPluginToServer, // BPP-5
	FrameTypeBPPColdStartHandshake:     DirectionPluginToServer, // BPP-6
}

// BPPEnvelopeWhitelist exposes the registry to tests in other packages
// (and to future BPP-1 wire dispatcher impls). Returns a fresh copy so
// callers can't mutate the source-of-truth.
func BPPEnvelopeWhitelist() map[string]Direction {
	out := make(map[string]Direction, len(bppEnvelopeWhitelist))
	for k, v := range bppEnvelopeWhitelist {
		out[k] = v
	}
	return out
}

// AllBPPEnvelopes returns one zero-valued instance of each registered
// envelope, in stable order. Used by the lint to drive reflection
// without needing build-time tag scanning.
func AllBPPEnvelopes() []BPPEnvelope {
	return []BPPEnvelope{
		ConnectFrame{},
		AgentRegisterFrame{},
		RuntimeSchemaAdvertiseFrame{},
		AgentConfigUpdateFrame{},
		AgentToggleFrame{},
		InboundMessageFrame{},
		PermissionDeniedFrame{}, // BPP-3.1
		HeartbeatFrame{},
		SemanticActionFrame{},
		ErrorReportFrame{},
		AgentConfigAckFrame{},     // AL-2b #481 §1.2
		TaskStartedFrame{},        // BPP-2.2 #485
		TaskFinishedFrame{},       // BPP-2.2 #485
		ReconnectHandshakeFrame{}, // BPP-5
		ColdStartHandshakeFrame{}, // BPP-6
	}
}
