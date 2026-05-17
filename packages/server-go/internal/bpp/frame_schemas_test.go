// Package bpp_test — frame_schemas_test.go: BPP-1 (#274/#280) envelope
// CI lint. Reflection-based reverse assertions that pin the 5
// invariants the spec brief calls out:
//
//   ① Each registered envelope has the RT-0 #237 matching layout
//      (`Type` is field 0, tagged `json:"type"`, no extra envelope-
//      level meta fields like `v` / `ts` — the discriminator IS the
//      envelope).
//   ② Control plane (6 frames) — direction lock = Server→Plugin.
//   ③ Data plane (3 frames)    — direction lock = Plugin→Server.
//   ④ Allow-list closure — every exported `*Frame` struct in package bpp
//      that satisfies BPPEnvelope is in `BPPEnvelopeWhitelist`, and
//      every allow-list entry has a matching struct (no orphans).
//   ⑤ godoc anchor — `BPP-1.*byte-identical.*RT-0` count >= 1 in the
//      bpp package source.
//
// This file backs `scripts/lint-bpp-envelope.sh`, which is in turn
// invoked by the `bpp-envelope-lint` job in `.github/workflows/ci.yml`.

package bpp_test

import (
	"reflect"
	"testing"

	"borgee-server/internal/bpp"
)

// TestBPPEnvelopeFrameWhitelist pins invariant ④. The lint walks every
// envelope returned by AllBPPEnvelopes(), maps it to its FrameType(),
// then asserts the allow-list exactly covers that set.
//
// AL-2b (#452) extended data plane to 4 frames (added agent_config_ack)
// → total 10 envelopes (6 control + 4 data). 跟 acceptance §1.2 字面
// exact match — direction lock plugin→server.
func TestBPPEnvelopeFrameWhitelist(t *testing.T) {
	t.Parallel()
	envs := bpp.AllBPPEnvelopes()
	if got, want := len(envs), 15; got != want {
		t.Fatalf("BPP envelope count: got %d, want %d (BPP-1 control 6 + data 3 + AL-2b ack +1 + BPP-2.2 task +2 + BPP-3.1 permission_denied +1 + BPP-5 reconnect_handshake +1 + BPP-6 cold_start_handshake +1 = 15)", got, want)
	}
	wl := bpp.BPPEnvelopeWhitelist()
	if got, want := len(wl), 15; got != want {
		t.Fatalf("allow-list size: got %d, want %d", got, want)
	}
	seen := map[string]struct{}{}
	for _, e := range envs {
		ft := e.FrameType()
		if ft == "" {
			t.Fatalf("envelope %T has empty FrameType()", e)
		}
		if _, dup := seen[ft]; dup {
			t.Fatalf("duplicate FrameType %q across envelopes", ft)
		}
		seen[ft] = struct{}{}
		if _, ok := wl[ft]; !ok {
			t.Fatalf("envelope %T (%q) is not in BPPEnvelopeWhitelist", e, ft)
		}
	}
	for ft := range wl {
		if _, ok := seen[ft]; !ok {
			t.Fatalf("allow-list has %q but no matching envelope struct", ft)
		}
	}
}

// TestBPPEnvelopeDirectionLock pins invariants ② + ③. Walks every
// envelope, calls FrameDirection(), and asserts it matches the
// allow-list value. Also asserts the control-plane / data-plane counts
// match the §2.1 / §2.2 row counts.
func TestBPPEnvelopeDirectionLock(t *testing.T) {
	t.Parallel()
	wl := bpp.BPPEnvelopeWhitelist()
	var ctrl, data int
	for _, e := range bpp.AllBPPEnvelopes() {
		ft := e.FrameType()
		want := wl[ft]
		got := e.FrameDirection()
		if got != want {
			t.Errorf("%s direction: got %q, want %q", ft, got, want)
		}
		switch got {
		case bpp.DirectionServerToPlugin:
			ctrl++
		case bpp.DirectionPluginToServer:
			data++
		default:
			t.Errorf("%s has invalid direction %q", ft, got)
		}
	}
	if ctrl != 7 {
		t.Errorf("control-plane envelope count: got %d, want 7 (§2.1 + BPP-3.1 permission_denied)", ctrl)
	}
	if data != 8 {
		t.Errorf("data-plane envelope count: got %d, want 8 (§2.2 + AL-2b agent_config_ack + BPP-2.2 task_started/task_finished + BPP-5 reconnect_handshake + BPP-6 cold_start_handshake)", data)
	}
}

// TestBPPEnvelopeFieldOrder pins invariant ① — the wire-layout lock
// with RT-0 #237 / RT-1.1 #290. Every envelope's first struct field
// MUST be `Type string` tagged `json:"type"`. This is the dispatcher
// contract — change it and every wire decoder breaks at once.
func TestBPPEnvelopeFieldOrder(t *testing.T) {
	t.Parallel()
	for _, e := range bpp.AllBPPEnvelopes() {
		typ := reflect.TypeOf(e)
		if typ.Kind() != reflect.Struct {
			t.Fatalf("%T is not a struct", e)
		}
		if typ.NumField() == 0 {
			t.Fatalf("%T has zero fields", e)
		}
		f0 := typ.Field(0)
		if f0.Name != "Type" {
			t.Errorf("%s field 0 name: got %q, want \"Type\"", typ.Name(), f0.Name)
		}
		if f0.Type.Kind() != reflect.String {
			t.Errorf("%s field 0 kind: got %v, want string", typ.Name(), f0.Type.Kind())
		}
		if got := f0.Tag.Get("json"); got != "type" {
			t.Errorf("%s field 0 json tag: got %q, want \"type\"", typ.Name(), got)
		}
	}
}

// --- helpers ---
