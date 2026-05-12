// Package bpp_test — agent_config_update_test.go: BPP-2.3 acceptance
// tests.
//
// Stance pins exercised (bpp-2-spec.md §0 立场 ③ + acceptance §3 +
// content-lock §1 ②):
//   - 6 allowed fields match the blueprint §1.4 table text
//   - runtime 调优字段 (api_key/temperature) reject
//   - config 单源 server→plugin (plugin 不上行 config — frame 方向锁)
//   - 幂等 reload (同 (agent_id, config_rev) 重复推送 no-op)
package bpp_test

import (
	"testing"

	"borgee-server/internal/bpp"
)

// TestBPP_ValidConfigFieldsWhitelist pins content-lock §1 ② — the six
// fields match the blueprint §1.4 left-column text.
func TestBPP_ValidConfigFieldsWhitelist(t *testing.T) {
	t.Parallel()
	want := []string{
		"name",
		"avatar",
		"prompt",
		"model",
		"capabilities",
		"enabled",
	}
	for _, f := range want {
		if !bpp.ValidConfigFields[f] {
			t.Errorf("field %q missing from 6-field allow-list", f)
		}
	}
	if len(bpp.ValidConfigFields) != len(want) {
		t.Errorf("allow-list length mismatch: got %d, want 6", len(bpp.ValidConfigFields))
	}
}

// TestBPP_ValidatePayload_AcceptsWhitelistedFields pins acceptance
// §3.2 happy path — payload with the six allowed fields parses + returns
// the typed map.
func TestBPP_ValidatePayload_AcceptsWhitelistedFields(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"name":"Helper"}`,
		`{"avatar":"https://example.com/h.png"}`,
		`{"prompt":"You are a helper..."}`,
		`{"model":"openclaw-1"}`,
		`{"capabilities":["read","write"]}`,
		`{"enabled":true}`,
		`{"name":"Helper","avatar":"x","prompt":"p","model":"m","capabilities":[],"enabled":false}`,
	}
	for _, payload := range cases {
		frame := bpp.AgentConfigUpdateFrame{
			Type:          bpp.FrameTypeBPPAgentConfigUpdate,
			AgentID:       "agent-X",
			SchemaVersion: 1,
			Blob:          payload,
		}
		parsed, err := bpp.ValidateConfigPayload(frame)
		if err != nil {
			t.Errorf("payload=%s rejected: %v", payload, err)
			continue
		}
		if parsed == nil {
			t.Errorf("payload=%s parsed nil", payload)
		}
	}
}

// TestBPP_ValidatePayload_RejectsRuntimeFields pins acceptance §3.2
// 反断 + content-lock §2 ⑤ — runtime 调优字段 (蓝图 §1.4 右列) MUST
// reject (Borgee 不带 runtime 立场 ① 字面).
func TestBPP_ValidatePayload_RejectsRuntimeFields(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"api_key":"sk-..."}`,
		`{"temperature":0.7}`,
		`{"token_limit":4096}`,
		`{"retry_strategy":"exponential"}`,
		`{"name":"Helper","api_key":"sk-..."}`, // mixed: 1 valid + 1 invalid
		`{"max_tokens":1000}`,
		`{"top_p":0.9}`,
		`{"unknown_field":"value"}`,
	} {
		frame := bpp.AgentConfigUpdateFrame{
			Type:          bpp.FrameTypeBPPAgentConfigUpdate,
			AgentID:       "agent-X",
			SchemaVersion: 1,
			Blob:          payload,
		}
		_, err := bpp.ValidateConfigPayload(frame)
		if err == nil {
			t.Errorf("payload=%s accepted — should reject (runtime 调优字段)", payload)
			continue
		}
		if !bpp.IsConfigFieldDisallowed(err) {
			t.Errorf("payload=%s wrong sentinel: %v", payload, err)
		}
	}
}

// TestBPP_ValidatePayload_RejectsMalformedJSON pins payload parse
// branch — non-object JSON / syntax error → errConfigPayloadMalformed.
func TestBPP_ValidatePayload_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{not json`,
		`["array","not","object"]`,
		`"plain string"`,
		`123`,
		``,
	} {
		frame := bpp.AgentConfigUpdateFrame{
			Type:          bpp.FrameTypeBPPAgentConfigUpdate,
			AgentID:       "agent-X",
			SchemaVersion: 1,
			Blob:          payload,
		}
		_, err := bpp.ValidateConfigPayload(frame)
		if err == nil {
			t.Errorf("malformed payload=%q accepted — should reject", payload)
			continue
		}
		if !bpp.IsConfigPayloadMalformed(err) {
			t.Errorf("payload=%q wrong sentinel: %v", payload, err)
		}
	}
}

// TestBPP_ConfigRevTracker_IdempotentReload pins acceptance §3.4 +
// 蓝图 §1.5 字面 "幂等 reload" — same (agent_id, config_rev) pushed
// twice = ShouldApply returns true once + false on duplicates.
func TestBPP_ConfigRevTracker_IdempotentReload(t *testing.T) {
	t.Parallel()
	tr := bpp.NewConfigRevTracker()

	// First apply at rev=1 → true.
	if !tr.ShouldApply("agent-X", 1) {
		t.Error("first apply at rev=1 should be true")
	}
	// Same (agent, rev) again → false (duplicate).
	if tr.ShouldApply("agent-X", 1) {
		t.Error("duplicate rev=1 should be false (idempotent)")
	}
	if got := tr.LastRev("agent-X"); got != 1 {
		t.Errorf("LastRev after dup: got %d, want 1", got)
	}

	// Forward to rev=2 → true.
	if !tr.ShouldApply("agent-X", 2) {
		t.Error("forward rev=2 should be true")
	}

	// Stale rev=1 (after rev=2) → false.
	if tr.ShouldApply("agent-X", 1) {
		t.Error("stale rev=1 after rev=2 should be false")
	}
	if got := tr.LastRev("agent-X"); got != 2 {
		t.Errorf("LastRev after stale: got %d, want 2", got)
	}

	// Different agent — independent tracker state.
	if !tr.ShouldApply("agent-Y", 1) {
		t.Error("different agent should track independently")
	}
	if got := tr.LastRev("agent-Y"); got != 1 {
		t.Errorf("LastRev agent-Y: got %d, want 1", got)
	}
}

// TestBPP_ConfigRevTracker_NegativeRev pins defensive — negative
// rev (never expected) returns false (treated as stale).
func TestBPP_ConfigRevTracker_NegativeRev(t *testing.T) {
	t.Parallel()
	tr := bpp.NewConfigRevTracker()
	if tr.ShouldApply("agent-X", -1) {
		t.Error("negative rev should be false (defensive)")
	}
	if tr.ShouldApply("agent-X", 0) {
		t.Error("zero rev should be false (initial state)")
	}
}

// TestAgentConfigUpdate_ErrorCodeLiteralsByteIdentical pins content-lock §1 ⑥
// error code strings match expected literals.
func TestAgentConfigUpdate_ErrorCodeLiteralsByteIdentical(t *testing.T) {
	t.Parallel()
	if bpp.ConfigErrCodeFieldDisallowed != "bpp.config_field_disallowed" {
		t.Errorf("ConfigErrCodeFieldDisallowed drift: got %q",
			bpp.ConfigErrCodeFieldDisallowed)
	}
	if bpp.ConfigErrCodePayloadMalformed != "bpp.config_payload_malformed" {
		t.Errorf("ConfigErrCodePayloadMalformed drift: got %q",
			bpp.ConfigErrCodePayloadMalformed)
	}
}
