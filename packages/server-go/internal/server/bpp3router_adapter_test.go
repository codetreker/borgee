// Package server — bpp_3_router_adapter_test.go: BPP-3 server-boot
// pluginFrameRouterAdapter (ws.PluginFrameRouter ↔ bpp.PluginFrameDispatcher)
// bridge coverage. Adapter is the only ws→bpp boundary at boot;
// without this test it stays 0% covered because cross-package bridge code is a
// typical cold path.
package server

import (
	"encoding/json"
	"log/slog"
	"testing"

	"borgee-server/internal/bpp"
	"borgee-server/internal/ws"
)

func TestBpp3routerAdapter_Route_Happy(t *testing.T) {
	t.Parallel()
	pfd := bpp.NewPluginFrameDispatcher(slog.Default())
	adapter := &pluginFrameRouterAdapter{pfd: pfd}

	// Empty payload → soft-skip (false, nil).
	handled, err := adapter.Route([]byte{}, ws.PluginSessionContext{OwnerUserID: "owner-1"})
	if handled || err != nil {
		t.Errorf("empty payload: expected (false, nil), got (%v, %v)", handled, err)
	}

	// Unknown frame type → soft-skip (forward-compat).
	raw, _ := json.Marshal(map[string]string{"type": "totally_unknown_frame"})
	handled, err = adapter.Route(raw, ws.PluginSessionContext{OwnerUserID: "owner-1"})
	if handled || err != nil {
		t.Errorf("unknown type: expected (false, nil), got (%v, %v)", handled, err)
	}
}

func TestBpp3routerAdapter_OwnerUserIDBridge(t *testing.T) {
	t.Parallel()
	// Verify ws.PluginSessionContext.OwnerUserID is byte-identical bridged
	// into bpp.PluginSessionContext.OwnerUserID via the adapter. Use a
	// recording dispatcher to capture the inbound session.
	rec := &recordingFrameDispatcher{}
	pfd := bpp.NewPluginFrameDispatcher(slog.Default())
	pfd.Register(bpp.FrameTypeBPPAgentConfigAck, rec)
	adapter := &pluginFrameRouterAdapter{pfd: pfd}

	raw, _ := json.Marshal(map[string]any{
		"type":           bpp.FrameTypeBPPAgentConfigAck,
		"agent_id":       "agent-x",
		"schema_version": 1,
		"status":         "applied",
	})
	_, _ = adapter.Route(raw, ws.PluginSessionContext{OwnerUserID: "owner-bridge-test"})

	if rec.lastSess.OwnerUserID != "owner-bridge-test" {
		t.Errorf("expected OwnerUserID bridged byte-identical, got %q", rec.lastSess.OwnerUserID)
	}
}

// recordingFrameDispatcher captures the bpp.PluginSessionContext seen on
// Dispatch. Used to assert the ws→bpp adapter passes OwnerUserID
// untransformed (the BPP-3 boundary contract).
type recordingFrameDispatcher struct {
	lastSess bpp.PluginSessionContext
	lastRaw  json.RawMessage
}

func (r *recordingFrameDispatcher) Dispatch(raw json.RawMessage, sess bpp.PluginSessionContext) error {
	r.lastSess = sess
	r.lastRaw = raw
	return nil
}
