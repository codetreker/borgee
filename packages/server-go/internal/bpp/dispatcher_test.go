// Package bpp_test — dispatcher_test.go: BPP-2.1 acceptance tests
// (战马E #460 4 件套 v0 lock → BPP-2.1 dispatch 实施第一段).
//
// Stance pins exercised (bpp-2-spec.md §0 + acceptance §1 + content-lock §1):
//   - ① 7 v1 operation allow-list matches the blueprint §1.3 text
//     (all 7 ops pass; 'list_users' / 'delete_org' / v2+ ops reject)
//   - ① dispatcher 不开 raw REST 旁路 (plugin 不下穿协议红线)
//   - ① permission 走 AP-0 RequirePermission (跟既有 REST 同闸 — 此层不
//     bypass)
//   - Error code strings match expected literals (`bpp.semantic_op_unknown`
//     anchor.create_owner_only #360 / dm.workspace_not_supported #407 /
//     iteration.target_not_in_channel #409 naming pattern)
package bpp_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"borgee-server/internal/bpp"
)

// fakeHandler is a recording ActionHandler used to assert dispatcher
// routing decisions in unit tests. It records the frame + session it
// was called with and returns the configured result/error.
type fakeHandler struct {
	calls   int
	last    bpp.SemanticActionFrame
	lastSes bpp.SessionContext
	result  []byte
	err     error
}

func (f *fakeHandler) HandleAction(frame bpp.SemanticActionFrame, sess bpp.SessionContext) ([]byte, error) {
	f.calls++
	f.last = frame
	f.lastSes = sess
	return f.result, f.err
}

// TestBPP_OpWhitelist pins acceptance §1.2: v1 ops match the
// blueprint plugin-protocol.md §1.3 text. Order matters — content-lock
// §1 ① bytes the op list directly.
//
// BPP-3.2.1 (#494 follow-up) extends 7→8 with `request_capability_grant`
// (蓝图 auth-permissions.md §1.3 主入口字面承袭).
func TestBPP_OpWhitelist(t *testing.T) {
	t.Parallel()
	want := []string{
		"create_artifact",
		"update_artifact",
		"reply_in_thread",
		"mention_user",
		"request_agent_join",
		"read_channel_history",
		"read_artifact",
		"request_capability_grant", // BPP-3.2.1
	}
	for _, op := range want {
		if !bpp.ValidSemanticOps[op] {
			t.Errorf("v1 allow-list missing op %q (must match blueprint §1.3 text)", op)
		}
	}
	if len(bpp.ValidSemanticOps) != len(want) {
		t.Errorf("v1 allow-list length mismatch: got %d, want %d (negative assertion: v2+ ops stay out of v1)",
			len(bpp.ValidSemanticOps), len(want))
	}
}

// TestBPP_RejectsUnknownOp pins acceptance §1.2 反断 + content-lock
// §2 ⑦ — 'list_users' / 'delete_org' / v2+ 列表 reject (蓝图 §1.3
// v2+ 字面禁 v1 进).
func TestBPP_RejectsUnknownOp(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	// Register a handler so the test specifically exercises the op-
	// validation branch (not the no-handler branch).
	for op := range bpp.ValidSemanticOps {
		_ = d.RegisterHandler(op, &fakeHandler{})
	}

	for _, bad := range []string{
		// v2+ 列表 (蓝图 §1.3 字面禁 v1 进).
		"propose_artifact_change",
		"request_owner_review",
		"request_clarification",
		// 显式权限漂 (admin god-mode 不入此 rail).
		"list_users", "delete_org", "grant_admin",
		// 大小写 / 空 / 同义词漂.
		"Create_Artifact", "createArtifact", "", "create-artifact",
	} {
		_, err := d.Dispatch(bpp.SemanticActionFrame{
			Type:    bpp.FrameTypeBPPSemanticAction,
			AgentID: "agent-1",
			Action:  bad,
			Payload: "{}",
			Nonce:   "n-1",
		}, bpp.SessionContext{AgentUserID: "u-1", PluginID: "p-1"})
		if err == nil {
			t.Errorf("op=%q accepted — should reject (v1 allow-list is fail-closed)", bad)
			continue
		}
		if !bpp.IsSemanticOpUnknown(err) {
			t.Errorf("op=%q rejected with wrong sentinel: got %v, want errSemanticOpUnknown", bad, err)
		}
	}
}

// TestBPP_DispatchRoutesToHandler pins acceptance §1.3: a registered
// handler is invoked for a valid op + the frame + session context are
// passed through unchanged (handler is the AP-0 perm gate, not
// dispatcher).
func TestBPP_DispatchRoutesToHandler(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	want := []byte(`{"artifact_id":"art-1"}`)
	h := &fakeHandler{result: want}
	if err := d.RegisterHandler(bpp.SemanticOpCreateArtifact, h); err != nil {
		t.Fatalf("register: %v", err)
	}

	frame := bpp.SemanticActionFrame{
		Type:    bpp.FrameTypeBPPSemanticAction,
		AgentID: "agent-X",
		Action:  bpp.SemanticOpCreateArtifact,
		Payload: `{"channel_id":"ch-Y","title":"P"}`,
		Nonce:   "n-42",
	}
	sess := bpp.SessionContext{AgentUserID: "u-agent-X", PluginID: "plugin-host"}
	result, err := d.Dispatch(frame, sess)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if string(result) != string(want) {
		t.Errorf("dispatch result mismatch: got %q, want %q", result, want)
	}
	if h.calls != 1 {
		t.Errorf("handler calls=%d, want 1", h.calls)
	}
	if h.last.Action != frame.Action || h.last.AgentID != frame.AgentID || h.last.Nonce != frame.Nonce {
		t.Errorf("handler frame mismatch: got %+v, want %+v", h.last, frame)
	}
	if h.lastSes != sess {
		t.Errorf("handler session mismatch: got %+v, want %+v", h.lastSes, sess)
	}
}

// TestBPP_DispatchAllSevenOps pins acceptance §1.2 happy path: each
// of the 7 v1 ops can be registered + dispatched independently. Drift
// between the spec list and the dispatcher would surface here as a
// missed op or duplicate routing.
func TestBPP_DispatchAllSevenOps(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	handlers := map[string]*fakeHandler{}
	for op := range bpp.ValidSemanticOps {
		h := &fakeHandler{result: []byte(op)}
		if err := d.RegisterHandler(op, h); err != nil {
			t.Fatalf("register %q: %v", op, err)
		}
		handlers[op] = h
	}

	for op := range bpp.ValidSemanticOps {
		frame := bpp.SemanticActionFrame{
			Type:    bpp.FrameTypeBPPSemanticAction,
			AgentID: "agent-z",
			Action:  op,
			Payload: "{}",
			Nonce:   "n-" + op,
		}
		result, err := d.Dispatch(frame, bpp.SessionContext{AgentUserID: "u-z"})
		if err != nil {
			t.Errorf("op=%q dispatch failed: %v", op, err)
			continue
		}
		if string(result) != op {
			t.Errorf("op=%q routed to wrong handler: result=%q", op, result)
		}
		// Per-op handler call count must be exactly 1 (no cross-op leak).
		if handlers[op].calls != 1 {
			t.Errorf("op=%q handler calls=%d, want 1 (cross-op leak suspected)",
				op, handlers[op].calls)
		}
	}
}

// TestBPP_NoHandlerRegistered pins the boot-order edge case: an op
// in the v1 allow-list with no registered handler returns an error
// distinct from the unknown-op error (so the api package can surface
// 503 service-unavailable, not 400 bad-request).
func TestBPP_NoHandlerRegistered(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	// Register only one op so the test specifically targets the
	// no-handler branch on a different allowed op.
	_ = d.RegisterHandler(bpp.SemanticOpCreateArtifact, &fakeHandler{})

	_, err := d.Dispatch(bpp.SemanticActionFrame{
		Type:    bpp.FrameTypeBPPSemanticAction,
		AgentID: "agent-z",
		Action:  bpp.SemanticOpReadArtifact, // valid op but no handler
		Payload: "{}",
		Nonce:   "n-no-handler",
	}, bpp.SessionContext{AgentUserID: "u-z"})
	if err == nil {
		t.Fatal("dispatch with no handler accepted — should error")
	}
	// Distinct from op-unknown sentinel (boot-order issue, not protocol violation).
	if bpp.IsSemanticOpUnknown(err) {
		t.Errorf("no-handler error masquerading as op-unknown sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), "no handler") {
		t.Errorf("no-handler error message doesn't say so: %v", err)
	}
}

// TestBPP_RegisterHandlerRejectsUnknownOp pins boot-time invariant:
// registering a handler for an op outside the v1 allow-list is a
// programming bug (typo or stale op name) and must fail loud.
func TestBPP_RegisterHandlerRejectsUnknownOp(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	for _, bad := range []string{
		"unknown_op",
		"propose_artifact_change", // v2+ list — must reject at boot too
		"",
		"Create_Artifact", // case漂
	} {
		err := d.RegisterHandler(bad, &fakeHandler{})
		if err == nil {
			t.Errorf("RegisterHandler(%q) accepted — should reject (v1 allow-list is fail-closed)", bad)
		}
	}
}

// TestBPP_RegisterHandlerIdempotent pins the (op, handler) duplicate
// registration semantic — same handler instance can be re-registered
// for the same op (idempotent boot path), but registering a different
// handler for the same op must fail loud (boot programming bug).
func TestBPP_RegisterHandlerIdempotent(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	h1 := &fakeHandler{}
	h2 := &fakeHandler{}

	if err := d.RegisterHandler(bpp.SemanticOpMentionUser, h1); err != nil {
		t.Fatalf("first register: %v", err)
	}
	// Same instance — idempotent.
	if err := d.RegisterHandler(bpp.SemanticOpMentionUser, h1); err != nil {
		t.Errorf("idempotent re-register failed: %v", err)
	}
	// Different instance — must fail.
	if err := d.RegisterHandler(bpp.SemanticOpMentionUser, h2); err == nil {
		t.Error("RegisterHandler with different handler accepted — should reject (one op, one handler)")
	}
}

// TestBPP_HandlerForUnregistered pins HandlerFor returns nil (not
// panic) for an unregistered op — so callers can treat nil as a
// transient boot-order signal.
func TestBPP_HandlerForUnregistered(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	if h := d.HandlerFor(bpp.SemanticOpCreateArtifact); h != nil {
		t.Errorf("HandlerFor unregistered op returned non-nil: %v", h)
	}
}

// TestBPP_DispatchErrorCodeLiteral pins acceptance + content-lock
// §1 ⑥ — the error code surfaced to the plugin must match
// `bpp.semantic_op_unknown` (跟 anchor.create_owner_only #360 /
// dm.workspace_not_supported #407 / iteration.target_not_in_channel
// #409 命名同模式).
func TestBPP_DispatchErrorCodeLiteral(t *testing.T) {
	t.Parallel()
	if bpp.DispatchErrCodeOpUnknown != "bpp.semantic_op_unknown" {
		t.Errorf("DispatchErrCodeOpUnknown literal drift: got %q, want %q",
			bpp.DispatchErrCodeOpUnknown, "bpp.semantic_op_unknown")
	}
	// Reserved for §4.1 反向 grep — defense-in-depth witness.
	if bpp.DispatchErrCodeNoRawREST != "bpp.plugin_no_raw_rest" {
		t.Errorf("DispatchErrCodeNoRawREST literal drift: got %q, want %q",
			bpp.DispatchErrCodeNoRawREST, "bpp.plugin_no_raw_rest")
	}
}

// TestBPP_HandlerErrorPropagated pins handler error pass-through —
// dispatcher does not swallow handler errors (which carry AP-0
// permission denials, payload parse errors, etc).
func TestBPP_HandlerErrorPropagated(t *testing.T) {
	t.Parallel()
	d := bpp.NewDispatcher()
	wantErr := errors.New("handler perm denied (AP-0)")
	_ = d.RegisterHandler(bpp.SemanticOpCreateArtifact, &fakeHandler{err: wantErr})

	_, err := d.Dispatch(bpp.SemanticActionFrame{
		Type:    bpp.FrameTypeBPPSemanticAction,
		AgentID: "agent-A",
		Action:  bpp.SemanticOpCreateArtifact,
		Payload: "{}",
	}, bpp.SessionContext{AgentUserID: "u-A"})
	if err == nil {
		t.Fatal("dispatcher swallowed handler error — should propagate")
	}
	if !errors.Is(err, wantErr) && !strings.Contains(err.Error(), wantErr.Error()) {
		t.Errorf("handler error not propagated: got %v, want %v", err, wantErr)
	}
}

// Compile-time guard: make sure fmt import doesn't drift.
var _ = fmt.Sprintf
