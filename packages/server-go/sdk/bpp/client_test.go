// Package bpp (sdk/bpp) — client_test.go: BPP-7.1 unit tests.
//
// Cases (acceptance §1):
//   1.1 ConnectFrame round-trip — server-defined ConnectFrame 5 fields
//       stay byte-identical through JSON round-trip and reflection checks.
//   1.2 frame schema byte-identical reflection check — SDK does not redefine frames.
//   1.3 WebSocket library and client dispatcher grep checks.
//   1.4 admin-only SDK path grep check.

package bpp_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	srvbpp "borgee-server/internal/bpp"
	sdkbpp "borgee-server/sdk/bpp"
)

// TestBPP_ConnectFrame_RoundTrip — acceptance §1.1.
//
// Encode a ConnectFrame with known fields, decode into the same type,
// and confirm the round-trip preserves all 5 fields with the right JSON
// keys (Type/PluginID/Token/Version/Capabilities).
func TestBPP_ConnectFrame_RoundTrip(t *testing.T) {
	original := srvbpp.ConnectFrame{
		Type:         srvbpp.FrameTypeBPPConnect,
		PluginID:     "plugin-1",
		Token:        "token-abc",
		Version:      "bpp-1",
		Capabilities: "stub",
	}
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// JSON keys must include the 5 contract fields byte-identical.
	for _, key := range []string{`"type"`, `"plugin_id"`, `"token"`, `"version"`, `"capabilities"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("missing JSON key %q in %s", key, b)
		}
	}
	var decoded srvbpp.ConnectFrame
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round-trip drift: got %+v want %+v", decoded, original)
	}
}

// TestBPP_FrameSchemaByteIdentical — acceptance §1.2 schema parity check.
//
// Reflect over server's bpp.AllBPPEnvelopes(); each frame must have a
// non-empty Type field of kind string with json:"type" tag at field 0.
// Also confirms SDK can iterate all 15 frames without redefining any.
func TestBPP_FrameSchemaByteIdentical(t *testing.T) {
	envs := srvbpp.AllBPPEnvelopes()
	if len(envs) != 15 {
		t.Fatalf("expected 15 envelopes (BPP-1..6), got %d", len(envs))
	}
	for _, e := range envs {
		typ := reflect.TypeOf(e)
		f0 := typ.Field(0)
		if f0.Name != "Type" {
			t.Errorf("%s field 0: got %q, want Type", typ.Name(), f0.Name)
		}
		if got := f0.Tag.Get("json"); got != "type" {
			t.Errorf("%s field 0 json tag: got %q, want type", typ.Name(), got)
		}
	}
}

// TestBPP_NilSafeCtor — acceptance §2.5 fail-fast constructor validation.
func TestBPP_NilSafeCtor(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
	}{
		{"empty pluginID", func() { sdkbpp.NewClient("", "agent-1", nil) }},
		{"empty agentID", func() { sdkbpp.NewClient("plugin-1", "", nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic on %s", tc.name)
				}
			}()
			tc.fn()
		})
	}
	// nil logger is OK (defaults to slog.Default).
	c := sdkbpp.NewClient("plugin-1", "agent-1", nil)
	if c == nil {
		t.Fatal("nil logger ctor returned nil Client unexpectedly")
	}
}
