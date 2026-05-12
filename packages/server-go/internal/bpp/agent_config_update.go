// Package bpp — agent_config_update.go: BPP-2.3 source-of-truth for
// the agent_config_update fields whitelist + idempotent reload validation.
//
// Blueprint: docs/blueprint/current/plugin-protocol.md §1.5 (field-based
// config hot reload and idempotent plugin reload) + §1.4 (Borgee-managed
// fields versus runtime-managed fields).
//
// Spec brief: docs/implementation/modules/bpp-2-spec.md §0 + §1 BPP-2.3.
// Stance: docs/qa/bpp-2-stance-checklist.md §3. Content lock:
// docs/qa/bpp-2-content-lock.md §1 ②.
//
// What this file does:
//  1. ConfigField enum lock — 6 values byte-identical with the blueprint
//     §1.4 Borgee-managed field list.
//  2. ValidateConfigPayload — parses the AgentConfigUpdateFrame.Payload
//     JSON (opaque on wire) + asserts every key ∈ valid whitelist;
//     runtime-tuning fields reject with ConfigErrCodeFieldDisallowed.
//  3. Track per-agent ConfigRev for idempotent reload — same
//     (agent_id, config_rev) pushed twice is a no-op (blueprint §1.5).
//
// Negative constraints (acceptance §3 + content-lock §2):
//   - runtime-tuning fields do not enter frame payload — reverse grep CI count==0
//     (acceptance §4.5).
//   - config is server→plugin only; plugins do not send config upstream — reverse grep
//     CI lint count==0 (acceptance §4.3).
//   - the field whitelist is closed — enum-out values reject + log warn
//     `bpp.config_field_disallowed`.
package bpp

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ConfigField enum — content-lock §1 ② byte-identical with the blueprint §1.4
// Borgee-managed field list. Changes must be coordinated with the blueprint,
// spec §0, and this enum.
const (
	ConfigFieldName         = "name"
	ConfigFieldAvatar       = "avatar"
	ConfigFieldPrompt       = "prompt"
	ConfigFieldModel        = "model"
	ConfigFieldCapabilities = "capabilities"
	ConfigFieldEnabled      = "enabled"
)

// ValidConfigFields is the 6-field whitelist set. Membership is the only
// gate at the dispatcher boundary; runtime-tuning fields MUST reject.
//
// Negative constraint (acceptance §4.5 + content-lock §2 ⑤): runtime-tuning
// fields from blueprint §1.4 do not enter frame payload.
var ValidConfigFields = map[string]bool{
	ConfigFieldName:         true,
	ConfigFieldAvatar:       true,
	ConfigFieldPrompt:       true,
	ConfigFieldModel:        true,
	ConfigFieldCapabilities: true,
	ConfigFieldEnabled:      true,
}

// ConfigErrCode* — error code literals are byte-identical with content-lock §1 ⑥.
const (
	ConfigErrCodeFieldDisallowed  = "bpp.config_field_disallowed"
	ConfigErrCodePayloadMalformed = "bpp.config_payload_malformed"
)

// errConfigFieldDisallowed / errConfigPayloadMalformed — sentinels.
var (
	errConfigFieldDisallowed  = errors.New("bpp: config field disallowed (not in 6-whitelist)")
	errConfigPayloadMalformed = errors.New("bpp: config payload not valid JSON object")
)

// IsConfigFieldDisallowed / IsConfigPayloadMalformed — sentinel matchers.
func IsConfigFieldDisallowed(err error) bool {
	return errors.Is(err, errConfigFieldDisallowed)
}
func IsConfigPayloadMalformed(err error) bool {
	return errors.Is(err, errConfigPayloadMalformed)
}

// ValidateConfigPayload parses frame.Blob (opaque on wire) as a
// flat JSON object + asserts every top-level key ∈ ValidConfigFields.
//
// Returns the parsed map on success (caller may consume the typed
// values). On reject:
//   - JSON parse failure → errConfigPayloadMalformed
//   - any key ∉ ValidConfigFields → errConfigFieldDisallowed (carries
//     the offending key for log warn).
//
// Negative constraint: runtime-tuning fields from blueprint §1.4 reject at the
// BPP-2.3 frame ingress.
func ValidateConfigPayload(frame AgentConfigUpdateFrame) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(frame.Blob), &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", errConfigPayloadMalformed, err)
	}
	for key := range parsed {
		if !ValidConfigFields[key] {
			return nil, fmt.Errorf("%w: field=%q (6-whitelist: name/avatar/prompt/model/capabilities/enabled)",
				errConfigFieldDisallowed, key)
		}
	}
	return parsed, nil
}

// ConfigRevTracker is the per-agent idempotency guard for plugin
// config reload (blueprint §1.5 wording: "the same update payload sent twice
// should have no side effects"). Stores the last-applied config_rev per agent_id; ShouldApply
// returns true only when the incoming rev is strictly greater than the
// last seen rev.
//
// Negative constraint: stale rev (incoming ≤ last) returns false WITHOUT error —
// it's a legitimate retry / network double-send, not a protocol
// violation. The caller logs at debug level + drops the frame.
//
// Thread-safety: ConfigRevTracker is NOT goroutine-safe. The BPP
// listener guarantees per-plugin-connection serialization (single
// reader per WS), which is the Borgee BPP-1 invariant — concurrent
// agent_config_update for the same agent_id from different plugin
// connections is itself a protocol violation (one runtime per agent,
// AL-4.1 #398 schema UNIQUE(agent_id) principle ① wording).
type ConfigRevTracker struct {
	last map[string]int64
}

// NewConfigRevTracker creates an empty per-agent rev tracker.
func NewConfigRevTracker() *ConfigRevTracker {
	return &ConfigRevTracker{last: make(map[string]int64)}
}

// ShouldApply returns true iff the incoming (agent_id, config_rev)
// pair represents a forward step (rev > last seen). On true, the
// tracker records the new rev. On false (stale or duplicate rev),
// the tracker leaves state unchanged — caller treats as no-op.
//
// Negative constraint: rev MUST be strictly increasing (blueprint §1.5
// "idempotent reload" wording: same payload twice = no-op). Equal rev returns false (idempotent
// retry guard). Negative rev returns false (defensive — never seen
// in practice but spec doesn't forbid; we treat as stale).
func (t *ConfigRevTracker) ShouldApply(agentID string, configRev int64) bool {
	last := t.last[agentID]
	if configRev <= last {
		return false
	}
	t.last[agentID] = configRev
	return true
}

// LastRev returns the last-applied config_rev for agent_id, or 0 if
// the tracker has never seen this agent. Test seam — production code
// should use ShouldApply, not introspect state.
func (t *ConfigRevTracker) LastRev(agentID string) int64 {
	return t.last[agentID]
}
