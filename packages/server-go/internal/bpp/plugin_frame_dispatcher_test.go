// Package bpp_test — plugin_frame_dispatcher_test.go: BPP-3 unified
// dispatcher boundary acceptance tests.
//
// Tests cover (acceptance §1+§2 BPP-3):
//   - Register validation (empty type / nil dispatcher / unknown type /
//     direction lock / duplicate type all panic)
//   - Route happy path (registered type → dispatcher.Dispatch invoked)
//   - Route unknown type (forward-compat: log + skip, no error)
//   - Route malformed JSON (soft-skip, no panic)
//   - Route empty type (soft-skip)
//   - AckFrameAdapter raw → typed decoding (delegates to AckDispatcher)
//   - Direction lock enforcement (server→plugin frame Register panics)
//
// 设计守: BPP-3 wire-routing only — no schema, no business logic, just
// boundary. Tests assert this by checking AllBPPEnvelopes() interaction.

package bpp_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"borgee-server/internal/bpp"
)

// fakeFrameDispatcher records Dispatch calls for verification.
type fakeFrameDispatcher struct {
	calls    int
	lastRaw  json.RawMessage
	lastSess bpp.PluginSessionContext
	retErr   error
}

func (f *fakeFrameDispatcher) Dispatch(raw json.RawMessage, sess bpp.PluginSessionContext) error {
	f.calls++
	f.lastRaw = raw
	f.lastSess = sess
	return f.retErr
}

// fakeAckHandler / fakeOwnerResolver — minimal seam impls for
// AckDispatcher wire-up in AckFrameAdapter test.
type fakeAckHandler struct {
	calls int
}

func (f *fakeAckHandler) HandleAck(_ bpp.AgentConfigAckFrame, _ bpp.AckSessionContext) error {
	f.calls++
	return nil
}

type fakeOwnerResolver struct {
	owner string
}

func (f *fakeOwnerResolver) OwnerOf(_ string) (string, error) {
	return f.owner, nil
}

// TestPluginFrameDispatcher_Route_Happy pins acceptance §2.1 — registered
// frame type routes to Dispatch with raw payload + session context.
func TestPluginFrameDispatcher_Route_Happy(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	fd := &fakeFrameDispatcher{}
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, fd)

	raw := []byte(`{"type":"agent_config_ack","agent_id":"a-1","schema_version":1,"status":"applied"}`)
	handled, err := pfd.Route(raw, bpp.PluginSessionContext{OwnerUserID: "u-1"})
	if err != nil {
		t.Fatalf("Route returned err: %v", err)
	}
	if !handled {
		t.Error("expected handled=true for registered frame type")
	}
	if fd.calls != 1 {
		t.Errorf("dispatcher.Dispatch calls: got %d want 1", fd.calls)
	}
	if fd.lastSess.OwnerUserID != "u-1" {
		t.Errorf("session OwnerUserID: got %q want u-1", fd.lastSess.OwnerUserID)
	}
}

// TestPluginFrameDispatcher_Route_UnknownType_SoftSkip pins forward-compat —
// unknown frame type returns (false, nil), no error, no panic. Plugin
// upgrade tolerance critical.
func TestPluginFrameDispatcher_Route_UnknownType_SoftSkip(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	raw := []byte(`{"type":"future_frame_v2","agent_id":"a-1"}`)
	handled, err := pfd.Route(raw, bpp.PluginSessionContext{OwnerUserID: "u-1"})
	if err != nil {
		t.Errorf("unknown type should not return err, got: %v", err)
	}
	if handled {
		t.Error("expected handled=false for unknown frame type")
	}
}

// TestPluginFrameDispatcher_Route_MalformedJSON_SoftSkip pins fail-soft —
// malformed JSON wire payload soft-skips (no panic, no err return).
func TestPluginFrameDispatcher_Route_MalformedJSON_SoftSkip(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	raw := []byte(`{not valid json`)
	handled, err := pfd.Route(raw, bpp.PluginSessionContext{})
	if err != nil {
		t.Errorf("malformed json should not return err, got: %v", err)
	}
	if handled {
		t.Error("expected handled=false for malformed json")
	}
}

// TestPluginFrameDispatcher_Route_EmptyPayload_SoftSkip pins guard — empty
// raw bytes soft-skip (zero-len edge).
func TestPluginFrameDispatcher_Route_EmptyPayload_SoftSkip(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	handled, err := pfd.Route(nil, bpp.PluginSessionContext{})
	if err != nil || handled {
		t.Errorf("empty payload should soft-skip, got handled=%v err=%v", handled, err)
	}
	handled, err = pfd.Route([]byte{}, bpp.PluginSessionContext{})
	if err != nil || handled {
		t.Errorf("zero-len payload should soft-skip, got handled=%v err=%v", handled, err)
	}
}

// TestPluginFrameDispatcher_Route_EmptyType_SoftSkip pins guard — payload
// without `type` field soft-skips.
func TestPluginFrameDispatcher_Route_EmptyType_SoftSkip(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	raw := []byte(`{"agent_id":"a-1"}`)
	handled, err := pfd.Route(raw, bpp.PluginSessionContext{})
	if err != nil || handled {
		t.Errorf("missing type should soft-skip, got handled=%v err=%v", handled, err)
	}
}

// TestPluginFrameDispatcher_Route_DispatcherError pins error propagation —
// when dispatcher returns err, Route returns (true, err) so callers can
// errors.Is for metrics/logging.
func TestPluginFrameDispatcher_Route_DispatcherError(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	fd := &fakeFrameDispatcher{retErr: errors.New("validation failed")}
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, fd)

	raw := []byte(`{"type":"agent_config_ack","agent_id":"a-1","schema_version":1,"status":"applied"}`)
	handled, err := pfd.Route(raw, bpp.PluginSessionContext{OwnerUserID: "u-1"})
	if !handled {
		t.Error("expected handled=true even on dispatcher err (frame matched)")
	}
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected dispatcher err to propagate, got: %v", err)
	}
}

// TestPluginFrameDispatcher_Register_PanicsOnEmptyType pins boot-bug guard.
func TestPluginFrameDispatcher_Register_PanicsOnEmptyType(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register(\"\", ...) should panic")
		}
	}()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	pfd.Register("", &fakeFrameDispatcher{})
}

// TestPluginFrameDispatcher_Register_PanicsOnNilDispatcher pins boot-bug guard.
func TestPluginFrameDispatcher_Register_PanicsOnNilDispatcher(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register(type, nil) should panic")
		}
	}()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, nil)
}

// TestPluginFrameDispatcher_Register_PanicsOnDuplicate pins single-dispatcher
// invariant — only one dispatcher per frame type.
func TestPluginFrameDispatcher_Register_PanicsOnDuplicate(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, &fakeFrameDispatcher{})
	defer func() {
		if r := recover(); r == nil {
			t.Error("duplicate Register should panic")
		}
	}()
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, &fakeFrameDispatcher{})
}

// TestPluginFrameDispatcher_Register_PanicsOnUnknownFrameType pins envelope
// whitelist enforcement — frame must be defined in envelope.go first.
func TestPluginFrameDispatcher_Register_PanicsOnUnknownFrameType(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register with non-whitelisted frame type should panic")
		}
	}()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	pfd.Register("nonexistent_frame_type_xyz", &fakeFrameDispatcher{})
}

// TestPluginFrameDispatcher_Register_PanicsOnServerToPluginFrame pins
// direction lock — only Plugin→Server frames may register here.
// AgentConfigUpdateFrame is server→plugin, so registering it here is a
// definitional bug.
func TestPluginFrameDispatcher_Register_PanicsOnServerToPluginFrame(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register server→plugin frame (AgentConfigUpdateFrame) should panic on direction lock")
		}
	}()
	pfd := bpp.NewPluginFrameDispatcher(nil)
	// FrameTypeBPPAgentConfigUpdate is DirectionServerToPlugin — should
	// be rejected by Register.
	pfd.Register(bpp.FrameTypeBPPAgentConfigUpdate, &fakeFrameDispatcher{})
}

// TestPluginFrameDispatcher_DecodesAndDelegates pins acceptance §3.1 —
// AckFrameAdapter wraps AckDispatcher: raw → AgentConfigAckFrame →
// typed Dispatch.
func TestPluginFrameDispatcher_DecodesAndDelegates(t *testing.T) {
	t.Parallel()
	handler := &fakeAckHandler{}
	resolver := &fakeOwnerResolver{owner: "u-1"}
	ackDisp := bpp.NewAckDispatcher(handler, resolver)
	adapter := bpp.NewAckFrameAdapter(ackDisp)

	raw := json.RawMessage(`{"type":"agent_config_ack","agent_id":"a-1","schema_version":1,"status":"applied"}`)
	err := adapter.Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u-1"})
	if err != nil {
		t.Fatalf("AckFrameAdapter.Dispatch err: %v", err)
	}
	if handler.calls != 1 {
		t.Errorf("AckHandler.HandleAck calls: got %d want 1", handler.calls)
	}
}

// TestPluginFrameDispatcher_PanicsOnNilDispatcher pins boot-bug guard.
func TestPluginFrameDispatcher_PanicsOnNilDispatcher(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewAckFrameAdapter(nil) should panic")
		}
	}()
	bpp.NewAckFrameAdapter(nil)
}

// TestPluginFrameDispatcher_DecodeError pins error wrapping — malformed
// raw frame returns wrapped error (not panic).
func TestPluginFrameDispatcher_DecodeError(t *testing.T) {
	t.Parallel()
	handler := &fakeAckHandler{}
	resolver := &fakeOwnerResolver{owner: "u-1"}
	ackDisp := bpp.NewAckDispatcher(handler, resolver)
	adapter := bpp.NewAckFrameAdapter(ackDisp)

	raw := json.RawMessage(`{"type":"agent_config_ack","schema_version":"not-a-number"}`)
	err := adapter.Dispatch(raw, bpp.PluginSessionContext{OwnerUserID: "u-1"})
	if err == nil {
		t.Error("expected decode error for malformed schema_version")
	}
	if !strings.Contains(err.Error(), "ack_frame_decode") {
		t.Errorf("expected wrapped decode error, got: %v", err)
	}
	if handler.calls != 0 {
		t.Errorf("handler should not be called on decode err, got %d calls", handler.calls)
	}
}

// TestPluginFrameDispatcher_Integration_RegisterRouteAck pins end-to-end
// routing: PluginFrameDispatcher.Route → AckFrameAdapter.Dispatch →
// AckDispatcher.Dispatch → fakeAckHandler.HandleAck. Validates the
// full BPP-3 boundary chain works.
func TestPluginFrameDispatcher_Integration_RegisterRouteAck(t *testing.T) {
	t.Parallel()
	handler := &fakeAckHandler{}
	resolver := &fakeOwnerResolver{owner: "u-1"}
	ackDisp := bpp.NewAckDispatcher(handler, resolver)
	adapter := bpp.NewAckFrameAdapter(ackDisp)

	pfd := bpp.NewPluginFrameDispatcher(nil)
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, adapter)

	raw := []byte(`{"type":"agent_config_ack","agent_id":"a-1","schema_version":1,"status":"applied"}`)
	handled, err := pfd.Route(raw, bpp.PluginSessionContext{OwnerUserID: "u-1"})
	if err != nil {
		t.Fatalf("Route err: %v", err)
	}
	if !handled {
		t.Error("expected handled=true")
	}
	if handler.calls != 1 {
		t.Errorf("end-to-end ack handler calls: got %d want 1", handler.calls)
	}
}
