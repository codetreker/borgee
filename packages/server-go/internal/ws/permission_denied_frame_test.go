// Package ws_test — permission_denied_frame_test.go: BPP-3.1 hub
// PushPermissionDenied 5 unit pins.
//
// 锚: docs/qa/acceptance-templates/bpp-3.1.md + spec
// docs/implementation/modules/bpp-3.1-spec.md §1 三立场.
package ws_test

import (
	"encoding/json"
	"strings"
	"testing"

	"borgee-server/internal/bpp"
	"borgee-server/internal/ws"
)

// REG-BPP31-001 — basic path: hub.PushPermissionDenied emits
// PermissionDeniedFrame with exact wire JSON; cursor is monotonic;
// sent=true.
func TestBPP_PushPermissionDenied_Basic(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-A")
	hub.RegisterPlugin("agent-A", pc)

	cur, sent := hub.PushPermissionDenied(
		"agent-A",
		"req-trace-1",
		"artifact.commit",
		"artifact.commit",
		"artifact:art-1",
		1700000000000,
	)
	if !sent {
		t.Fatal("PushPermissionDenied must succeed when plugin registered")
	}
	if cur == 0 {
		t.Fatal("cursor must be > 0 (hub.cursors allocator running)")
	}

	wire, ok := pc.DrainSend()
	if !ok {
		t.Fatal("plugin send channel empty — frame not enqueued")
	}
	want := `{"type":"permission_denied","cursor":` + itoa(cur) +
		`,"agent_id":"agent-A","request_id":"req-trace-1",` +
		`"attempted_action":"artifact.commit","required_capability":"artifact.commit",` +
		`"current_scope":"artifact:art-1","denied_at":1700000000000}`
	if wire != want {
		t.Fatalf("wire JSON mismatch:\n got: %s\nwant: %s", wire, want)
	}

	// Round-trip validation.
	var frame bpp.PermissionDeniedFrame
	if err := json.Unmarshal([]byte(wire), &frame); err != nil {
		t.Fatalf("frame unmarshal: %v", err)
	}
	if frame.Type != bpp.FrameTypeBPPPermissionDenied {
		t.Errorf("frame.Type = %q, want %q", frame.Type, bpp.FrameTypeBPPPermissionDenied)
	}
	if frame.RequiredCapability != "artifact.commit" || frame.CurrentScope != "artifact:art-1" {
		t.Errorf("payload mismatch: %+v", frame)
	}
}

// REG-BPP31-002 — direction lock server→plugin (frame schema invariant
// enforced by bppEnvelopeWhitelist). plugin 永不发 permission_denied.
func TestBPP_DirectionLock_ServerToPlugin(t *testing.T) {
	t.Parallel()
	wl := bpp.BPPEnvelopeWhitelist()
	dir, ok := wl[bpp.FrameTypeBPPPermissionDenied]
	if !ok {
		t.Fatalf("permission_denied not in BPPEnvelopeWhitelist")
	}
	if dir != bpp.DirectionServerToPlugin {
		t.Errorf("permission_denied direction = %q, want %q (negative assertion: plugin never sends it)",
			dir, bpp.DirectionServerToPlugin)
	}
	// 反向: ensure FrameDirection() instance method 同源.
	if got := (bpp.PermissionDeniedFrame{}).FrameDirection(); got != bpp.DirectionServerToPlugin {
		t.Errorf("FrameDirection() = %q, want %q", got, bpp.DirectionServerToPlugin)
	}
}

// REG-BPP31-003 — plugin offline fail-graceful: sent=false, cursor still
// allocated (sequence 不留洞, 跟 PushAgentConfigUpdate 同模式).
func TestBPP_PushPermissionDenied_PluginOffline(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	cur, sent := hub.PushPermissionDenied(
		"agent-OFFLINE", "req-2", "artifact.commit",
		"artifact.commit", "artifact:art-9", 1700000000000,
	)
	if sent {
		t.Error("plugin offline → sent must be false (frame dropped, no queue)")
	}
	if cur == 0 {
		t.Error("cursor must still allocate even when plugin offline (sequence 不留洞)")
	}
}

// REG-BPP31-004 — shared cursor sequence: BPP-3.1 push shares the same
// sequence as RT-1 PushArtifactUpdated and AL-2b PushAgentConfigUpdate
// (negative assertion §1: no plugin-only channel).
func TestBPP_PushPermissionDenied_SharedSequence(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-A")
	hub.RegisterPlugin("agent-A", pc)

	c1, ok1 := hub.PushArtifactUpdated("art-1", 1, "ch-1", 1700000000000, "commit")
	if !ok1 {
		t.Fatal("seed RT-1 push failed")
	}
	c2, ok2 := hub.PushPermissionDenied("agent-A", "req", "artifact.commit",
		"artifact.commit", "artifact:art-1", 1700000000001)
	if !ok2 {
		t.Fatal("BPP-3.1 push failed")
	}
	if c2 <= c1 {
		t.Errorf("BPP-3.1 cursor must be strictly above prior RT-1; c1=%d c2=%d", c1, c2)
	}
	c3, ok3 := hub.PushAgentConfigUpdate("agent-A", 1, `{}`, "k", 1700000000002)
	if !ok3 {
		t.Fatal("AL-2b push failed")
	}
	if c3 <= c2 {
		t.Errorf("AL-2b cursor after BPP-3.1 must continue monotonic; c2=%d c3=%d", c2, c3)
	}
}

// REG-BPP31-005 — field-level wire shape (filled + zero-tail). Negative
// assertion: all 8 fields serialize; no omitempty.
func TestBPP_PushPermissionDenied_FieldByteIdentity(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-Z")
	hub.RegisterPlugin("agent-Z", pc)

	cur, sent := hub.PushPermissionDenied("agent-Z", "", "", "", "", 0)
	if !sent {
		t.Fatal("zero-tail push must succeed")
	}
	wire, ok := pc.DrainSend()
	if !ok {
		t.Fatal("send channel empty")
	}
	want := `{"type":"permission_denied","cursor":` + itoa(cur) +
		`,"agent_id":"agent-Z","request_id":"","attempted_action":"",` +
		`"required_capability":"","current_scope":"","denied_at":0}`
	if wire != want {
		t.Fatalf("zero-tail wire JSON mismatch:\n got: %s\nwant: %s", wire, want)
	}
	// All 8 fields serialize (no omitempty drift).
	if count := strings.Count(wire, ":"); count < 8 {
		t.Errorf("wire must serialize all 8 fields; saw %d ':' (omitempty drift)", count)
	}
}

// REG-BPP31-006 — interface seam: PermissionDeniedPusher 可由 *Hub 实现
// (api 包将通过此接口注入, AP-1 #493 follow-up wiring).
func TestBPP_HubImplementsPermissionDeniedPusher(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)
	var _ ws.PermissionDeniedPusher = hub
}
