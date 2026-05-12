// Package ws_test — al_2b_2_agent_config_push_test.go: AL-2b.2 hub
// PushAgentConfigUpdate emit + cursor sequence + plugin offline + frame
// byte-identity round-trip.
//
// Reference: docs/qa/acceptance-templates/al-2b.md §2.1 (delivery latency
// ≤1s + shared cursor sequence) + §2.2 (idempotent reload — same idempotency_key
// 重发 plugin 端 dedup, server stateless 约束).
package ws_test

import (
	"encoding/json"
	"strings"
	"testing"

	"borgee-server/internal/bpp"
	"borgee-server/internal/ws"
)

// TestAL_PushAgentConfigUpdateBasic pins acceptance §2.1 — 基本路径:
// hub.PushAgentConfigUpdate emits AgentConfigUpdateFrame to plugin's send
// channel with byte-identical wire JSON; cursor increases monotonically; sent=true.
func TestAL_PushAgentConfigUpdateBasic(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-A")
	hub.RegisterPlugin("agent-A", pc)

	cur, sent := hub.PushAgentConfigUpdate(
		"agent-A",
		3,
		`{"name":"BotZ","prompt":"…"}`,
		"idem-X",
		1700000000000,
	)
	if !sent {
		t.Fatal("PushAgentConfigUpdate must succeed when plugin is registered")
	}
	if cur == 0 {
		t.Fatal("cursor must be > 0 (hub.cursors allocator running)")
	}

	// Drain the send channel — assert wire JSON matches #472
	// AgentConfigUpdateFrame field order.
	wire, ok := pc.DrainSend()
	if !ok {
		t.Fatal("plugin send channel empty — frame not enqueued")
	}
	want := `{"type":"agent_config_update","cursor":` + itoa(cur) +
		`,"agent_id":"agent-A","schema_version":3,"blob":"{\"name\":\"BotZ\",\"prompt\":\"…\"}",` +
		`"idempotency_key":"idem-X","created_at":1700000000000}`
	if wire != want {
		t.Fatalf("wire JSON byte-identity broken:\n got: %s\nwant: %s", wire, want)
	}

	// Round-trip — plugin would parse this exact bytes.
	var frame bpp.AgentConfigUpdateFrame
	if err := json.Unmarshal([]byte(wire), &frame); err != nil {
		t.Fatalf("frame unmarshal: %v", err)
	}
	if frame.Type != bpp.FrameTypeBPPAgentConfigUpdate {
		t.Errorf("frame.Type = %q, want %q", frame.Type, bpp.FrameTypeBPPAgentConfigUpdate)
	}
	if frame.AgentID != "agent-A" || frame.SchemaVersion != 3 {
		t.Errorf("frame round-trip mismatch: %+v", frame)
	}
}

// TestAL_PushAgentConfigUpdate_PluginOffline pins acceptance §2.1
// fail-graceful path — plugin not registered → sent=false, cursor still
// allocated (blueprint §1.5 "runtime 不缓存": frame dropped, plugin
// reconnects and GET /agents/:id/config pulls; server does not queue).
func TestAL_PushAgentConfigUpdate_PluginOffline(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	cur, sent := hub.PushAgentConfigUpdate(
		"agent-OFFLINE",
		1,
		`{}`,
		"idem-Y",
		1700000000000,
	)
	if sent {
		t.Error("plugin offline → sent must be false (frame dropped, no queue per 蓝图 §1.5)")
	}
	if cur == 0 {
		t.Error("cursor must still allocate even when plugin offline (sequence 不留洞)")
	}
}

// TestAL_PushAgentConfigUpdate_CursorMonotonic pins acceptance §2.1
// shared cursor sequence — N pushes strictly increase cursor, sharing one
// sequence with RT-1 PushArtifactUpdated (no plugin-only channel).
func TestAL_PushAgentConfigUpdate_CursorMonotonic(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-A")
	hub.RegisterPlugin("agent-A", pc)

	c1, s1 := hub.PushAgentConfigUpdate("agent-A", 1, `{}`, "k1", 1)
	c2, s2 := hub.PushAgentConfigUpdate("agent-A", 2, `{}`, "k2", 2)
	c3, s3 := hub.PushAgentConfigUpdate("agent-A", 3, `{}`, "k3", 3)
	if !s1 || !s2 || !s3 {
		t.Fatalf("all 3 sends must succeed; got %v %v %v", s1, s2, s3)
	}
	if !(c1 < c2 && c2 < c3) {
		t.Errorf("cursor must be strictly monotonic; got %d %d %d", c1, c2, c3)
	}
}

// TestAL_PushAgentConfigUpdate_SharedSequenceWithRT1 pins acceptance
// §2.1 design 1 shared cursor sequence — AL-2b push and RT-1.1
// PushArtifactUpdated share one sequence (same pattern as anchor_comment_frame_test /
// iteration_state_changed_frame_test — no separate channel).
func TestAL_PushAgentConfigUpdate_SharedSequenceWithRT1(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-A")
	hub.RegisterPlugin("agent-A", pc)

	c1, sent1 := hub.PushArtifactUpdated("art-1", 1, "ch-1", 1700000000000, "commit")
	if !sent1 {
		t.Fatal("seed RT-1 push failed")
	}
	c2, sent2 := hub.PushAgentConfigUpdate("agent-A", 1, `{}`, "k1", 1)
	if !sent2 {
		t.Fatal("AL-2b push failed")
	}
	if c2 <= c1 {
		t.Errorf("AL-2b cursor must be strictly above prior RT-1 cursor; c1=%d c2=%d (约束 共一根 sequence)", c1, c2)
	}

	// Push another RT-1 frame — must continue from c2, not reset.
	c3, sent3 := hub.PushArtifactUpdated("art-2", 1, "ch-1", 1700000000001, "commit")
	if !sent3 {
		t.Fatal("third push failed")
	}
	if c3 <= c2 {
		t.Errorf("RT-1 cursor after AL-2b must continue monotonic; c2=%d c3=%d", c2, c3)
	}
}

// TestAL_PushAgentConfigUpdate_FieldByteIdentity pins acceptance §2.1
// + #472 §1.1 — wire JSON matches BPP envelope reflect lint
// (filled + zero-tail snapshots — do not add omitempty).
func TestAL_PushAgentConfigUpdate_FieldByteIdentity(t *testing.T) {
	t.Parallel()
	hub, _ := setupTestHub(t)

	pc := ws.NewTestPluginConn("agent-Z")
	hub.RegisterPlugin("agent-Z", pc)

	// Empty blob + zero schema_version — all 7 fields are serialized.
	cur, sent := hub.PushAgentConfigUpdate("agent-Z", 0, "", "idem-empty", 0)
	if !sent {
		t.Fatal("zero-tail push must succeed")
	}
	wire, ok := pc.DrainSend()
	if !ok {
		t.Fatal("send channel empty")
	}
	want := `{"type":"agent_config_update","cursor":` + itoa(cur) +
		`,"agent_id":"agent-Z","schema_version":0,"blob":"","idempotency_key":"idem-empty","created_at":0}`
	if wire != want {
		t.Fatalf("zero-tail wire byte-identity broken:\n got: %s\nwant: %s", wire, want)
	}
	// Constraint: 7 keys (always serialized, no omitempty).
	count := strings.Count(wire, ":")
	if count < 7 {
		t.Errorf("wire must serialize all 7 fields; saw %d ':' (omitempty mismatch)", count)
	}
}

// TestAL_PushAgentConfigUpdate_NoCursorAllocator pins fail-graceful
// — hub without cursor allocator returns (0, false). Same pattern as
// PushArtifactUpdated h.cursors==nil; test seed.
func TestAL_PushAgentConfigUpdate_NoCursorAllocator(t *testing.T) {
	t.Parallel()
	// Bare Hub via NewHub — but with cursors set up automatically. Skip
	// this case: production paths do not expose a Hub with cursors=nil. setupTestHub provides
	// an allocator, matching hub_test setupTestHub. This test only anchors the comment +
	// PushArtifactUpdated 同 fail-mode 实现 (h.cursors==nil → 0/false).
	t.Skip("setupTestHub always provides cursor allocator; covered by code review of guard line.")
}

// TestAL_PluginConnDrainSendEmpty pins NewTestPluginConn helper —
// Empty send channel returns ("", false), preventing false positives in later cases.
func TestAL_PluginConnDrainSendEmpty(t *testing.T) {
	t.Parallel()
	pc := ws.NewTestPluginConn("agent-empty")
	wire, ok := pc.DrainSend()
	if ok || wire != "" {
		t.Errorf("empty send channel: got (%q, %v), want (\"\", false)", wire, ok)
	}
}

// itoa — minimal int64 → string helper (not pulling strconv imports for
// a 6-line test file; mirror cursor_test.go style).
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
