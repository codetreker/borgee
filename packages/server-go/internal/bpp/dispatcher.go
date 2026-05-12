// Package bpp — dispatcher.go: BPP-2.1 source-of-truth for the
// semantic_action dispatch layer (plugin → server → existing REST handler).
//
// Blueprint: docs/blueprint/current/plugin-protocol.md §1.3. Plugins call
// Borgee through the semantic-action layer, not direct REST. Spec brief:
// docs/implementation/modules/bpp-2-spec.md §0 + §1 BPP-2.1. Stance and
// content locks: docs/qa/bpp-2-stance-checklist.md §1 and
// docs/qa/bpp-2-content-lock.md §1 ①.
//
// What this dispatcher does:
//
//  1. Plugin upstream emits a `SemanticActionFrame` (BPP-1 envelope §2.2,
//     already in `bppEnvelopeWhitelist` since #304). BPP-2.1 ADDS the
//     server-side `Dispatch(frame)` routing layer — no envelope wire
//     change: BPP-1 envelope and the frame whitelist remain unchanged.
//  2. Validate `Action` ∈ 7 v1 whitelist (blueprint §1.3); enum-out
//     values reject with `bpp.semantic_op_unknown` error code.
//  3. Resolve `(action, agent_id, payload)` → an `ActionHandler`
//     registered by the api package (interface seam, similar to
//     AgentInvitationPusher / ArtifactPusher pattern — bpp pkg never
//     imports internal/api).
//  4. Permission check via AP-0 RequirePermission is the responsibility
//     of the registered handler — the dispatcher only routes, does
//     not bypass permission checks. The dispatcher does not accept a raw
//     HTTP client, call `http.Post`, or build URLs to REST endpoints.
//
// Negative constraints (bpp-2-spec.md §0 + acceptance §4.1 reverse grep):
//   - Dispatcher has no raw HTTP / REST bypass; CI reverse grep must return 0
//     hits across this package, excluding _test.go.
//   - The 7-op whitelist is closed: enum-out values such as 'list_users' /
//     'delete_org' reject with `bpp.semantic_op_unknown`.
//   - v2+ collaborative intent actions are not in the v1 whitelist.
package bpp

import (
	"errors"
	"fmt"
)

// SemanticOp values pin the v1 whitelist byte-identical with
// plugin-protocol.md §1.3. Changes must be coordinated with the blueprint,
// bpp-2-spec.md §0, and this implementation enum.
const (
	SemanticOpCreateArtifact     = "create_artifact"
	SemanticOpUpdateArtifact     = "update_artifact"
	SemanticOpReplyInThread      = "reply_in_thread"
	SemanticOpMentionUser        = "mention_user"
	SemanticOpRequestAgentJoin   = "request_agent_join"
	SemanticOpReadChannelHistory = "read_channel_history"
	SemanticOpReadArtifact       = "read_artifact"
	// BPP-3.2.1 — agent requests owner approval through the capability grant
	// flow. After the plugin SDK receives a BPP-3.1 permission_denied frame,
	// this op asks the server to write a system DM to the owner. It reuses the
	// existing DM-2 path and does not add a channel type.
	SemanticOpRequestCapabilityGrant = "request_capability_grant"
)

// ValidSemanticOps is the v1 whitelist set. Membership is the only gate
// at the dispatcher boundary — the registered handler then enforces
// permission via AP-0 RequirePermission and parses the payload.
//
// Order matches the blueprint table (§1.3) for byte-identical review.
// 反向约束: do NOT add v2+ ops here without first updating the blueprint;
// CI grep 反向断言 count==0 for v2+ literals (acceptance §4 反向约束).
//
// BPP-3.2.1 (#494 后续): 7→8 加 request_capability_grant; 蓝图
// §1.3 字面跟随 + bpp-3.2-spec.md §1 原则 ① + bpp-3.2-stance §1.
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
// 错误码字面 (跟 出处 anchor.create_owner_only #360 / dm.workspace_not_supported
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
// 反向约束 (acceptance §4.1): Dispatcher 不接 raw HTTP / REST endpoint,
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
//  1. frame.Action ∈ ValidSemanticOps (蓝图 §1.3 v1 whitelist) →
//     returns errSemanticOpUnknown if not.
//  2. handler registered for op → returns ErrNoHandler if not.
//  3. Delegate to handler.HandleAction(frame, sess) — handler enforces
//     permission via AP-0 + parses Payload.
//
// 反向约束: Dispatch does not call out to raw HTTP / REST. The handler
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
