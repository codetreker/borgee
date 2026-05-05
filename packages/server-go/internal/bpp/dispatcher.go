// Package bpp — dispatcher.go: BPP-2.1 source-of-truth for the
// semantic_action dispatch layer (plugin → server → existing REST handler).
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.3 (Plugin 调 Borgee
// 抽象语义层 C, 不直对 REST + 协议红线 "不允许 plugin 下穿语义层直调
// REST" + 7 v1 必须语义动作字面). Spec brief: docs/implementation/modules/
// bpp-2-spec.md (战马E #460 v0) §0 立场 ① + §1 拆段 BPP-2.1.
// Stance: docs/qa/bpp-2-stance-checklist.md §1 立场 ① 8 反约束 checkbox.
// Content lock: docs/qa/bpp-2-content-lock.md §1 ① 7 op 白名单字面.
//
// What this dispatcher does:
//
//   1. Plugin upstream emits a `SemanticActionFrame` (BPP-1 envelope §2.2,
//      already in `bppEnvelopeWhitelist` since #304). BPP-2.1 ADDS the
//      server-side `Dispatch(frame)` routing layer — no envelope wire
//      change (反约束: BPP-1 envelope 不裂, 9 frame whitelist 不动).
//   2. Validate `Action` ∈ 7 v1 whitelist (蓝图 §1.3 字面); enum-out
//      values reject with `bpp.semantic_op_unknown` error code.
//   3. Resolve `(action, agent_id, payload)` → an `ActionHandler`
//      registered by the api package (interface seam, similar to
//      AgentInvitationPusher / ArtifactPusher pattern — bpp pkg never
//      imports internal/api).
//   4. Permission check via AP-0 RequirePermission is the responsibility
//      of the registered handler — the dispatcher only routes, does
//      not bypass perm. 反约束: dispatcher 不接 raw HTTP client
//      / `http.Post`, 不拼 URL 调 REST endpoint (蓝图 §1.3 协议红线
//      字面 "不允许 plugin 下穿语义层直调 REST").
//
// 反约束 (bpp-2-spec.md §0 立场 ① + acceptance §4.1 反向 grep):
//   - Dispatcher 不开 raw HTTP / REST 旁路 — 反向 grep
//     reverse grep CI lint count==0 — see acceptance §4.1.
//     across this package (excluding _test.go).
//   - 7 op 白名单严闭, 'list_users' / 'delete_org' 等枚举外值 reject +
//     log warn `bpp.semantic_op_unknown` (跟 dm.workspace_not_supported
//     #407 / iteration.target_not_in_channel #409 / anchor.create_owner_only
//     #360 错码命名同模式).
//   - v2+ 列表 (蓝图 §1.3 v2+ 协作意图动作) 不在 v1 白名单, reject —
//     列表字面禁 v1 进.
package bpp

import (
	"errors"
	"fmt"
)

// SemanticOp values pin the v1 whitelist byte-identical 跟蓝图
// plugin-protocol.md §1.3 字面 "v1 必须的语义动作" 7 项. Drift here
// breaks reverse grep `bpp-2-content-lock.md §1 ①` byte-identical 锁.
//
// 改 = 改三处: 蓝图 plugin-protocol.md §1.3 + spec bpp-2-spec.md §0
// 立场 ① + this enum (实施代码 source-of-truth).
const (
	SemanticOpCreateArtifact     = "create_artifact"
	SemanticOpUpdateArtifact     = "update_artifact"
	SemanticOpReplyInThread      = "reply_in_thread"
	SemanticOpMentionUser        = "mention_user"
	SemanticOpRequestAgentJoin   = "request_agent_join"
	SemanticOpReadChannelHistory = "read_channel_history"
	SemanticOpReadArtifact       = "read_artifact"
	// BPP-3.2.1 — agent 触发 owner DM 走 capability 审批流 (蓝图
	// auth-permissions.md §1.3 主入口字面). plugin 端 SDK 收 BPP-3.1
	// permission_denied frame 后, 通过此 op 触发 server 给 owner 写
	// system DM (复用 DM-2 既有 path, 反约束: 不开新 channel 类型).
	// 文案锁见 docs/qa/bpp-3.2-content-lock.md §1; quick_action JSON
	// shape 见 §2 (action ∈ {grant, reject, snooze}).
	SemanticOpRequestCapabilityGrant = "request_capability_grant"
)

// ValidSemanticOps is the v1 whitelist set. Membership is the only gate
// at the dispatcher boundary — the registered handler then enforces
// permission via AP-0 RequirePermission and parses the payload.
//
// Order matches the blueprint table (§1.3) for byte-identical review.
// 反约束: do NOT add v2+ ops here without first updating the blueprint;
// CI grep 反向断言 count==0 for v2+ literals (acceptance §4 反约束).
//
// BPP-3.2.1 (#494 follow-up): 7→8 加 request_capability_grant; 蓝图
// §1.3 字面承袭 + bpp-3.2-spec.md §1 立场 ① + bpp-3.2-stance §1.
var ValidSemanticOps = map[string]bool{
	SemanticOpCreateArtifact:         true,
	SemanticOpUpdateArtifact:         true,
	SemanticOpReplyInThread:          true,
	SemanticOpMentionUser:            true,
	SemanticOpRequestAgentJoin:       true,
	SemanticOpReadChannelHistory:     true,
	SemanticOpReadArtifact:           true,
	SemanticOpRequestCapabilityGrant: true,
}

// DispatchErrCodeOpUnknown is the error code returned when a plugin
// upstream SemanticActionFrame carries an Action outside the v1
// whitelist. byte-identical literal 跟 bpp-2-content-lock.md §1 ⑥
// 错误码字面 (跟 anchor.create_owner_only #360 / dm.workspace_not_supported
// #407 / iteration.target_not_in_channel #409 命名同模式).
const DispatchErrCodeOpUnknown = "bpp.semantic_op_unknown"

// DispatchErrCodeNoRawREST is the error code reserved for plugin
// attempts to bypass the dispatch layer (e.g. raw HTTP request through
// the BPP socket). v0 implementation does not currently emit this code
// — the protocol envelope itself enforces frame-only ingress (BPP-1
// #304 envelope whitelist). The constant is reserved as a defense-in-
// depth witness for acceptance §4.1 反向 grep + future runtime patches.
const DispatchErrCodeNoRawREST = "bpp.plugin_no_raw_rest"

// errSemanticOpUnknown is the sentinel returned by Dispatch when the
// SemanticActionFrame.Action is not in the v1 whitelist. Callers should
// surface this to the plugin via an error frame carrying
// DispatchErrCodeOpUnknown.
var errSemanticOpUnknown = errors.New("bpp: semantic op unknown")

// IsSemanticOpUnknown lets callers map the package-private sentinel to
// the wire-level error code without exporting the var directly (跟
// errArtifactConflict / errIterationStateMachineReject 同模式).
func IsSemanticOpUnknown(err error) bool {
	return errors.Is(err, errSemanticOpUnknown)
}

// ActionHandler is the seam between the bpp package and the api package
// for routing a validated SemanticActionFrame to the matching REST
// handler. The api package implements one ActionHandler per v1 op and
// registers it via Dispatcher.RegisterHandler at server boot. The bpp
// package never imports internal/api — this is the same pattern as
// ArtifactPusher / AgentInvitationPusher / IterationStatePusher.
//
// AP-0 RequirePermission is the handler's responsibility (handler is
// itself the existing REST handler wrapped to consume frame + session
// context). Dispatcher does NOT bypass permission checks.
type ActionHandler interface {
	// HandleAction is invoked once a SemanticActionFrame is validated
	// and routed by op. The implementation must:
	//   - Parse SemanticActionFrame.Payload as JSON args (op-specific
	//     shape; see plugin-protocol.md §1.3 v1 args table).
	//   - Resolve the agent's user permissions via AP-0 (跟既有 REST
	//     handler 同闸).
	//   - Execute the side effect (artifact create / message send / ...).
	//   - Return a result blob the bpp.SemanticActionAck frame can carry.
	HandleAction(frame SemanticActionFrame, sess SessionContext) (result []byte, err error)
}

// SessionContext is the per-plugin-connection context the Dispatcher
// passes to ActionHandler. Carries the resolved agent user (BPP-1
// connect frame token already authenticated the agent at handshake)
// + the plugin id (for audit trail).
//
// AP-0 RequirePermission is invoked using sess.AgentUserID — the
// permission scope is per-channel where applicable (跟 既有 REST
// handler 模式同 — `auth.RequirePermission(s, "message.send", channelID)`).
type SessionContext struct {
	AgentUserID string // resolved via BPP-1 connect handshake
	PluginID    string // for audit / log only
}

// Dispatcher routes validated SemanticActionFrame instances to the
// registered ActionHandler for the op.
//
// 反约束 (acceptance §4.1): Dispatcher 不接 raw HTTP / REST endpoint,
// 不在内部 import internal/api 包 (依赖反转 via ActionHandler interface).
// Plugin 不下穿走 raw REST — protocol red line (蓝图 §1.3).
type Dispatcher struct {
	handlers map[string]ActionHandler
}

// NewDispatcher creates an empty dispatcher. The api package registers
// one handler per v1 op at server boot (server.go) before the BPP
// listener accepts plugin connections.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]ActionHandler),
	}
}

// RegisterHandler associates an ActionHandler with one v1 op. The op
// MUST be in ValidSemanticOps; registering an unknown op is a server
// boot bug and panics (defense-in-depth: prevents typo-driven 0-coverage
// op routes from silently entering production).
//
// Registration is idempotent on (op, handler) but rejects re-registration
// of a different handler for the same op — this would silently break
// invariant tests (one op, one handler) and is a programming bug.
func (d *Dispatcher) RegisterHandler(op string, h ActionHandler) error {
	if _, ok := ValidSemanticOps[op]; !ok {
		return fmt.Errorf("bpp: cannot register handler for unknown op %q (not in v1 whitelist)", op)
	}
	if existing, ok := d.handlers[op]; ok && existing != h {
		return fmt.Errorf("bpp: handler for op %q already registered", op)
	}
	d.handlers[op] = h
	return nil
}

// HandlerFor returns the registered ActionHandler for op, or nil if no
// handler is registered. Callers should treat nil as a transient boot-
// order issue (handler not yet wired) and reject the frame with a
// service-unavailable response — not as a permanent op-unknown error.
func (d *Dispatcher) HandlerFor(op string) ActionHandler {
	return d.handlers[op]
}

// Dispatch validates a plugin-upstream SemanticActionFrame and routes
// it to the registered handler.
//
// Validation (in order):
//   1. frame.Action ∈ ValidSemanticOps (蓝图 §1.3 v1 whitelist) →
//      returns errSemanticOpUnknown if not.
//   2. handler registered for op → returns ErrNoHandler if not.
//   3. Delegate to handler.HandleAction(frame, sess) — handler enforces
//      permission via AP-0 + parses Payload.
//
// 反约束: Dispatch does not call out to raw HTTP / REST. The handler
// is a pre-resolved ActionHandler interface, not a URL or http.Client.
// Reverse grep CI lint count==0 across
// internal/bpp/ (acceptance §4.1).
func (d *Dispatcher) Dispatch(frame SemanticActionFrame, sess SessionContext) ([]byte, error) {
	if _, ok := ValidSemanticOps[frame.Action]; !ok {
		return nil, fmt.Errorf("%w: action=%q (v1 whitelist: 7 ops)", errSemanticOpUnknown, frame.Action)
	}
	h := d.HandlerFor(frame.Action)
	if h == nil {
		return nil, fmt.Errorf("bpp: no handler registered for op %q", frame.Action)
	}
	return h.HandleAction(frame, sess)
}
