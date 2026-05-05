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
// reflection lint in frame_schemas_test.go. A drift is a CI red.
//
// Whitelist — only the 9 OpName constants enumerated in
// `bppEnvelopeWhitelist` are permitted. Adding a frame here without a
// matching blueprint row is a CI red (TestBPPEnvelopeFrameWhitelist).
//
// 反约束 — this file MUST NOT contain any `replay_mode = "full"`
// default, `defaultReplayMode` symbol, or `default.*ResumeModeFull`
// branch (RT-1.3 hardline carried forward). The reverse grep step in
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
	// BPP-3.1 permission_denied — server 通知 plugin authz 失败
	// (蓝图 auth-permissions.md §2 不变量 + §4.1 row 字面). Server-rail
	// only; plugin 永不发. payload 字段 byte-identical 跟 AP-1 abac.go
	// 403 body (跨 PR drift 守 — 改 = 改三处).
	FrameTypeBPPPermissionDenied = "permission_denied"

	// Data plane (Plugin → Server) — §2.2.
	// Data plane (Plugin → Server) — §2.2.
	FrameTypeBPPHeartbeat      = "heartbeat"
	FrameTypeBPPSemanticAction = "semantic_action"
	FrameTypeBPPErrorReport    = "error_report"
	FrameTypeBPPAgentConfigAck = "agent_config_ack" // AL-2b #481 §1.2 ack 路径
	// BPP-2.2 task lifecycle reverse-channel — plugin upstream signals
	// agent busy/idle (蓝图 §1.6 + agent-lifecycle §2.3 字面: source 必须
	// plugin 上行 frame, 不准 stub). 跟 AL-1b #482 BPP single source
	// 立场同源 (蓝图 §2.3 R3). online = session-level 走 WS conn
	// lifecycle, 跟 task-level (busy) 正交.
	FrameTypeBPPTaskStarted  = "task_started"
	FrameTypeBPPTaskFinished = "task_finished"

	// BPP-5 plugin reconnect handshake — plugin upstream signals
	// reconnect-with-cursor (蓝图 §1.6 重连恢复 + §2.1 connect 路径承袭;
	// reconnect ≠ connect — connect 是首次身份+capabilities, reconnect
	// 携带 last_known_cursor 恢复, 字段集不交). cursor resume **复用
	// RT-1.3 #296 既有 mechanism** (bpp.ResolveResume + Mode=incremental,
	// AfterCursor=last_known_cursor); state 翻转走 AL-1 5-state graph
	// 既有 error→online valid edge (无 persisted "connecting" 中间态,
	// connecting 仅 spec 概念名, 跟 #492 valid edge byte-identical).
	FrameTypeBPPReconnectHandshake = "reconnect_handshake"

	// BPP-6 plugin cold-start handshake — plugin upstream signals process
	// restart (state 全丢, 无 cursor) (蓝图 §1.6 进程死亡 vs 网络重连;
	// cold-start ≠ reconnect — reconnect 持 last_known_cursor 走
	// ResolveResume 增量恢复 (BPP-5); cold-start 不持 cursor 走 fresh
	// start — agent.Tracker.Clear + AL-1 #492 single-gate
	// AppendAgentStateTransition any→online byte-identical, reason 复用
	// `runtime_crashed` 6-dict 不扩 (#496 SSOT, 锁链第 11 处). 字段集
	// 跟 ReconnectHandshakeFrame **互斥反断** — cold-start 不含
	// LastKnownCursor / DisconnectAt / ReconnectAt 字段.
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
//   {Type, Cursor, AgentID, SchemaVersion, Blob, IdempotencyKey, CreatedAt}
//
// Field order is the contract. Do NOT reorder without updating
// schema_equivalence_test.go + acceptance al-2b.md §1.1 simultaneously.
//
// Field semantics:
//   - Type: discriminator 头位, byte-identical 跟 BPP-1 envelope (#280)
//   - Cursor: hub.cursors atomic int64 单调发号, 跟 RT-1 #290 + CV-2.2
//     #360 + DM-2.2 #372 + CV-4.2 #416 + AL-2b 5 source frame 共一根
//     sequence (RT-1 spec §1.1, 反约束: 不另起 plugin-only 通道; 立场
//     "不另起 channel" 跟 acceptance §2.1 字面同源)
//   - AgentID: target agent UUID
//   - SchemaVersion: 单调跟 agent_configs.schema_version (AL-2a v=20 #447)
//     字面 byte-identical; plugin 收到 < 当前 server 值 → ack `status=stale`
//     (acceptance §2.3)
//   - Blob: 序列化后的 SSOT 字段 (name/avatar/prompt/model/能力开关/启用
//     状态/memory_ref); 反约束 不含 api_key/temperature/token_limit/
//     retry_policy runtime-only 字段 (acceptance §3.2 + AL-2a #447 SSOT)
//   - IdempotencyKey: server 生成的稳定 key, 同 key 重发 plugin reload
//     仅触发 1 次 (acceptance §2.2 + 蓝图 §1.5 字面 "幂等 reload")
//   - CreatedAt: Unix ms 语义戳 (反约束: 不用作排序源, cursor 才是; 跟
//     IterationStateChangedFrame.CompletedAt 同语义模式)
//
// Plugin MUST reload idempotently; same payload pushed twice is a no-op.
type AgentConfigUpdateFrame struct {
	Type           string `json:"type"`
	Cursor         int64  `json:"cursor"`
	AgentID        string `json:"agent_id"`
	SchemaVersion  int64  `json:"schema_version"`
	Blob           string `json:"blob"` // JSON-encoded SSOT delta; opaque on the wire
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

// PermissionDeniedFrame — BPP-3.1 server 通知 plugin authz 失败 (蓝图
// auth-permissions.md §2 不变量字面 "Permission denied 走 BPP — 不靠
// HTTP 错误码, 由协议层路由到 owner DM" + §4.1 row 字面 frame 字段:
// `attempted_action`, `required_capability`, `current_scope`, `reason`).
//
// 8 字段 byte-identical 跟 spec bpp-3.1 §1 立场 ③:
//
//   {Type, Cursor, AgentID, RequestID, AttemptedAction, RequiredCapability, CurrentScope, DeniedAt}
//
// Field semantics:
//   - Type: discriminator 头位 byte-identical 跟 BPP envelope (#280)
//   - Cursor: hub.cursors 单调发号, 跟 RT-1/CV-2/DM-2/CV-4/AL-2b 共一根
//     sequence (反约束: 不另起 plugin-only 推送通道)
//   - AgentID: target agent UUID (deny 路径 plugin 端按 agent 分流)
//   - RequestID: AP-1 调用方生成的 trace UUID, plugin 按此 key 关联
//     owner DM 推审批通知 + retry 流 (BPP-3.2 follow-up)
//   - AttemptedAction: ∈ BPP-2.1 7 op 白名单 (`SemanticOp*` const) 或
//     REST endpoint 名 (e.g. "POST /artifacts/:id/commits"); 反约束:
//     'list_users' 等 v2+ 枚举外值 reject
//   - RequiredCapability: byte-identical 跟 AP-1 abac.go 403 body 字段
//     (e.g. "commit_artifact" 跟 AP-1 capabilities.go const 同源 — drift =
//     双向 grep CI lint red)
//   - CurrentScope: byte-identical 跟 AP-1 abac.go 403 body 字段
//     (e.g. "artifact:art-1" 跟 AP-1 ArtifactScopeStr 同源)
//   - DeniedAt: Unix ms 语义戳 (反约束: 不用作排序源, cursor 才是; 跟
//     IterationStateChangedFrame.CompletedAt 同语义模式)
//
// 反约束 (spec bpp-3.1 §2):
//   - direction = server→plugin hard-locked; plugin 永不发
//     (bppEnvelopeWhitelist + reflect lint 双闸守)
//   - admin god-mode 不消费此 frame (admin 走 /admin-api/* 不入业务路径,
//     ADM-0 §1.3 红线)
//   - HTTP 403 是 fallback, BPP frame 是 primary (蓝图 §2 不变量字面)
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
// AgentConfigUpdateFrame (AL-2b acceptance #452 §1.2 + 蓝图 §1.5 幂等
// reload). Direction is hard-locked plugin→server (反向断言:
// DirectionServerToPlugin 不在此 frame, 跟 BPP-1 #304 direction 锁同模式).
//
// 7 字段 byte-identical 跟 acceptance §1.2:
//
//   {Type, Cursor, AgentID, SchemaVersion, Status, Reason, AppliedAt}
//
// Field semantics:
//   - Type: discriminator 头位 byte-identical 跟 BPP envelope #280
//   - Cursor: plugin echo update.Cursor 做配对 (server 端按 cursor
//     配 ack ↔ AgentConfigUpdateFrame; ack 自身不走 hub.cursors 单调
//     发号 — ack 是 plugin → server 回执, 跟 update 走的 server →
//     plugin push cursor 不同根 sequence)
//   - AgentID: target agent UUID, 跟 update frame byte-identical
//   - SchemaVersion: plugin 实际 apply 的 schema_version (acceptance §2.3
//     stale 路径: plugin 收到 < server 当前 → ack 携带 plugin 已知值,
//     server 据此判 stale 触发 plugin 主动拉)
//   - Status: 'applied' | 'rejected' | 'stale' (acceptance §1.2 CHECK
//     enum byte-identical; 反约束 reject 'unknown' 等枚举外值, server 端
//     校验 fail-closed)
//   - Reason: stale/rejected 时填 (跟 AL-1a #249 6 reason 枚举 byte-
//     identical 同源 — api_key_invalid/quota_exceeded/network_unreachable/
//     runtime_crashed/runtime_timeout/unknown); applied 态时空 string
//     (反约束: 不挂 omitempty, 跟 IterationStateChangedFrame.ErrorReason
//     同模式 — 始终序列化)
//   - AppliedAt: Unix ms plugin 实际 reload 完成戳 (acceptance §2.2 幂等
//     reload — applied 态填真值, stale/rejected 填 0)
//
// 反约束 (acceptance §3.2 + §4.2):
//   - 不挂 cursor 之外的排序字段 — sort.AgentConfigAck.time / timestamp
//     反向 grep 0 hit (跟 RT-1 立场反约束同源, cursor 唯一可信序)
//   - 不下发 admin god-mode (admin 不入业务路径, ADM-0 §1.3 红线 + 反向
//     grep `admin.*AgentConfig.*ack` 0 hit)
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

// AgentConfigAck status enum byte-identical 跟 acceptance §1.2 CHECK
// + server-side fail-closed 校验 (枚举外值 reject).
const (
	AgentConfigAckStatusApplied  = "applied"
	AgentConfigAckStatusRejected = "rejected"
	AgentConfigAckStatusStale    = "stale"
)

// TaskStartedFrame — BPP-2.2 plugin signals agent has started a task
// (§1.6 + agent-lifecycle.md §2.3 字面: busy/idle source 必须 plugin
// 上行 frame, stub 一旦上 v1 拆掉 = 白写). The `Subject` field is the
// human-readable description ("agent 在做什么") — server REJECTS empty
// or whitespace-only Subject + log warn `bpp.task_subject_empty`
// (野马 §11 文案守 + spec §0 立场 ② 字面禁默认值 fallback).
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

// TaskFinishedFrame — BPP-2.2 plugin signals task termination. `Outcome`
// ∈ 3 enum ('completed' / 'failed' / 'cancelled'); when 'failed', `Reason`
// MUST be one of AL-1a #249 6 字典 (api_key_invalid / quota_exceeded /
// network_unreachable / runtime_crashed / runtime_timeout / unknown) —
// 跟 AL-3 #305 + AL-4 #321 + #427 三处单测锁同源 (改 = 改四处, BPP-2.2
// 是第四). 反约束: 'partial' / 'paused' / 'pending' / 'starting' 中间
// 态 reject.
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
// 6 字段 byte-identical 跟 spec brief §1 BPP-5.1:
//   {Type, PluginID, AgentID, LastKnownCursor, DisconnectAt, ReconnectAt}
//
// 反约束 (跟 spec §0 + stance §1 立场承袭):
//   - **不复用 ConnectFrame** — connect 携 Token + Capabilities (首次身份);
//     reconnect 携 last_known_cursor (恢复). 字段集不交.
//   - **不另开 channel/sub_protocol** — 单 BPP envelope frame, 跟 BPP-3
//     dispatcher 复用 (PluginFrameDispatcher 注册).
//   - **cursor resume 复用 RT-1.3** — server handler 调
//     bpp.ResolveResume(SessionResumeRequest{Mode: incremental,
//     AfterCursor: LastKnownCursor}, …). 不另起 sequence.
//   - 字段顺序锁: type/plugin_id/agent_id/last_known_cursor/disconnect_at/
//     reconnect_at — 跟 BPP-1 #304 envelope CI lint reflect 自动覆盖.
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
//   1. agent.Tracker.Clear(agentID) — drop in-memory state
//   2. Store.AppendAgentStateTransition(agentID, fromState, online,
//      runtime_crashed, "") — AL-1 #492 single-gate writes state-log row
//   3. NO history replay (反向 BPP-5 — cold-start 是 fresh start)
//
// 5 字段 byte-identical 跟 spec brief §1 BPP-6.1:
//   {Type, PluginID, AgentID, RestartAt, RestartReason}
//
// 反约束 (跟 spec §0 + stance §1 立场承袭):
//   - **字段集与 ReconnectHandshakeFrame 互斥** — 不含 LastKnownCursor /
//     DisconnectAt / ReconnectAt 字段. spec §0.1 立场守门.
//   - **不另开 channel/sub_protocol** — 单 BPP envelope frame, 跟 BPP-3
//     dispatcher 复用 (PluginFrameDispatcher 注册).
//   - **不重放历史 frame** — handler 不调 ResolveResume, 不携 cursor.
//     spec §0.2 立场守门 (AST scan 守).
//   - **不另开 plugin_restart_count 列** — restart 计数走 state-log
//     COUNT(WHERE to_state='online' AND reason='runtime_crashed') 反向
//     derive. spec §0.3 立场守门.
//   - reason 复用 `runtime_crashed` 6-dict byte-identical (反映上次
//     error → 此次复活语义). reasons SSOT #496 不扩第 7 字面.
//   - 字段顺序锁: type/plugin_id/agent_id/restart_at/restart_reason.
type ColdStartHandshakeFrame struct {
	Type          string `json:"type"`
	PluginID      string `json:"plugin_id"`
	AgentID       string `json:"agent_id"`
	RestartAt     int64  `json:"restart_at"`     // Unix ms
	RestartReason string `json:"restart_reason"` // e.g. "sigkill", "panic", "oom"; opaque to server, audit-only
}

func (ColdStartHandshakeFrame) FrameType() string         { return FrameTypeBPPColdStartHandshake }
func (ColdStartHandshakeFrame) FrameDirection() Direction { return DirectionPluginToServer }

// bppEnvelopeWhitelist — single-source-of-truth list of permitted
// BPP-1 envelope OpNames. The reflection lint asserts every exported
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
	FrameTypeBPPAgentConfigAck: DirectionPluginToServer, // AL-2b #481
	FrameTypeBPPTaskStarted:    DirectionPluginToServer, // BPP-2.2 #485
	FrameTypeBPPTaskFinished:   DirectionPluginToServer, // BPP-2.2 #485
	FrameTypeBPPReconnectHandshake: DirectionPluginToServer, // BPP-5
	FrameTypeBPPColdStartHandshake: DirectionPluginToServer, // BPP-6
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
		AgentConfigAckFrame{}, // AL-2b #481 §1.2
		TaskStartedFrame{},    // BPP-2.2 #485
		TaskFinishedFrame{},   // BPP-2.2 #485
		ReconnectHandshakeFrame{}, // BPP-5
		ColdStartHandshakeFrame{}, // BPP-6
	}
}
